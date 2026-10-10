// Package server is a small HTTP server shared by all services.
// It adds /healthz, /readyz, /metrics, /version and graceful shutdown.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	requests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "HTTP requests by route and status code. Probes and /metrics are not counted.",
	}, []string{"route", "code"})

	duration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration by route.",
		Buckets: prometheus.DefBuckets,
	}, []string{"route"})

	buildInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "app_build_info",
		Help: "Always 1. Labels show the running service and version.",
	}, []string{"service", "version"})
)

// Server is an HTTP server with health, metrics and graceful shutdown.
type Server struct {
	name    string
	version string
	mux     *http.ServeMux
	ready   atomic.Bool
}

// New creates a server with the built-in routes.
func New(name, version string) *Server {
	s := &Server{name: name, version: version, mux: http.NewServeMux()}
	buildInfo.WithLabelValues(name, version).Set(1)

	// Liveness: the process is running.
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok\n")
	})
	// Readiness: false during shutdown, so Kubernetes and the ALB stop sending traffic.
	s.mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		if !s.ready.Load() {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, "ready\n")
	})
	s.mux.Handle("GET /metrics", promhttp.Handler())
	s.mux.HandleFunc("GET /version", func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, map[string]string{"service": s.name, "version": s.version})
	})
	return s
}

// Handle registers an application route. Only these routes are counted in http_requests_total.
func (s *Server) Handle(pattern string, h http.HandlerFunc) {
	s.mux.Handle(pattern, instrument(pattern, h))
}

// ServeHTTP lets tests call the server without a network listener.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// Run serves until ctx is cancelled, then shuts down in this order:
//  1. readiness fails
//  2. wait SHUTDOWN_DELAY, so endpoints and ALB targets are removed
//  3. finish in-flight requests
func (s *Server) Run(ctx context.Context, addr string) error {
	srv := &http.Server{Addr: addr, Handler: s.mux, ReadHeaderTimeout: 5 * time.Second}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	s.ready.Store(true)
	slog.Info("server started", "service", s.name, "version", s.version, "addr", addr)

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	s.ready.Store(false)
	delay := EnvDuration("SHUTDOWN_DELAY", 10*time.Second)
	slog.Info("shutdown started", "delay", delay)
	time.Sleep(delay)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	slog.Info("shutdown complete")
	return nil
}

// WriteJSON writes v as a JSON response.
func WriteJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// Env returns the environment variable, or def if it is not set.
func Env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

// EnvInt returns the environment variable as an int, or def if it is not set or invalid.
func EnvInt(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return v
	}
	return def
}

// EnvDuration returns the environment variable as a duration (example: "10s"), or def.
func EnvDuration(key string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(os.Getenv(key)); err == nil {
		return v
	}
	return def
}

type statusRecorder struct {
	http.ResponseWriter
	code int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.code = code
	r.ResponseWriter.WriteHeader(code)
}

func instrument(route string, h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, code: http.StatusOK}
		h(rec, r)
		requests.WithLabelValues(route, strconv.Itoa(rec.code)).Inc()
		duration.WithLabelValues(route).Observe(time.Since(start).Seconds())
	})
}

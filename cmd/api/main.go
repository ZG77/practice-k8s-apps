// api is the backend. All routes are under /api, because the ALB sends /api/* here unchanged.
// Scaling practice: KEDA on requests per second (http_requests_total).
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ZG77/practice-k8s-apps/internal/load"
	"github.com/ZG77/practice-k8s-apps/internal/server"
)

var version = "dev" // set at build time with -ldflags "-X main.version=..."

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	s := server.New("api", version)
	s.Handle("GET /api/hello", hello)
	s.Handle("GET /api/cpu", load.CPUHandler)
	s.Handle("GET /api/mem", load.MemHandler)

	if err := s.Run(ctx, ":"+server.Env("PORT", "8080")); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}

func hello(w http.ResponseWriter, _ *http.Request) {
	host, _ := os.Hostname()
	server.WriteJSON(w, map[string]string{
		"service": "api",
		"version": version,
		"pod":     host,
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

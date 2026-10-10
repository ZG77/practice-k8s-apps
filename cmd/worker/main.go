// worker runs a steady background loop. It has no public routes.
// Scaling practice: VPA. The load is steady, so VPA can learn a good request.
//
// Settings (environment):
//
//	WORK_INTERVAL   time between jobs        (default 1s)
//	WORK_MS         CPU time per job, in ms  (default 100)
//	BASE_MEMORY_MB  memory held all the time (default 64)
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/ZG77/practice-k8s-apps/internal/load"
	"github.com/ZG77/practice-k8s-apps/internal/server"
)

var version = "dev" // set at build time with -ldflags "-X main.version=..."

var jobs = promauto.NewCounter(prometheus.CounterOpts{
	Name: "worker_jobs_total",
	Help: "Background jobs completed.",
})

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	interval := server.EnvDuration("WORK_INTERVAL", time.Second)
	work := time.Duration(server.EnvInt("WORK_MS", 100)) * time.Millisecond
	baseMB := server.EnvInt("BASE_MEMORY_MB", 64)

	// Hold the base memory for the whole life of the process.
	load.Allocate(baseMB, 100*365*24*time.Hour)
	go loop(ctx, interval, work)

	s := server.New("worker", version)
	// Manual load, in addition to the loop. Reach it with kubectl port-forward.
	s.Handle("GET /cpu", load.CPUHandler)
	s.Handle("GET /mem", load.MemHandler)

	if err := s.Run(ctx, ":"+server.Env("PORT", "8080")); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}

func loop(ctx context.Context, interval, work time.Duration) {
	slog.Info("worker loop started", "interval", interval, "work", work)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			load.Burn(work)
			jobs.Inc()
		}
	}
}

// web is the frontend. It serves "/" and calls the api service.
// Scaling practice: HPA on CPU.
package main

import (
	"context"
	"fmt"
	"io"
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

	apiURL := server.Env("API_URL", "http://api.backend.svc.cluster.local")
	client := &http.Client{Timeout: 2 * time.Second}

	s := server.New("web", version)
	s.Handle("GET /{$}", home(client, apiURL))
	s.Handle("GET /cpu", load.CPUHandler)
	s.Handle("GET /mem", load.MemHandler)

	if err := s.Run(ctx, ":"+server.Env("PORT", "8080")); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}

// home shows the web version and the answer from the api.
// If the api is down (or a NetworkPolicy blocks it), web still answers 200.
func home(client *http.Client, apiURL string) http.HandlerFunc {
	host, _ := os.Hostname()
	return func(w http.ResponseWriter, r *http.Request) {
		apiAnswer := callAPI(r.Context(), client, apiURL+"/api/hello")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "web %s (pod %s)\napi: %s\n", version, host, apiAnswer)
	}
}

func callAPI(ctx context.Context, client *http.Client, url string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "error: " + err.Error()
	}
	resp, err := client.Do(req)
	if err != nil {
		return "unavailable: " + err.Error()
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return string(body)
}

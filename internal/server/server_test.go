package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func get(t *testing.T, s *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestHealthz(t *testing.T) {
	s := New("test", "v1")
	if rec := get(t, s, "/healthz"); rec.Code != http.StatusOK {
		t.Fatalf("/healthz = %d, want 200", rec.Code)
	}
}

func TestReadyzFollowsState(t *testing.T) {
	s := New("test", "v1")
	if rec := get(t, s, "/readyz"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("/readyz before start = %d, want 503", rec.Code)
	}
	s.ready.Store(true)
	if rec := get(t, s, "/readyz"); rec.Code != http.StatusOK {
		t.Fatalf("/readyz after start = %d, want 200", rec.Code)
	}
}

func TestHandleCountsRequests(t *testing.T) {
	s := New("test", "v1")
	s.Handle("GET /hello", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	counter := requests.WithLabelValues("GET /hello", "418")
	before := testutil.ToFloat64(counter)
	get(t, s, "/hello")
	get(t, s, "/healthz") // probes are not counted

	if got := testutil.ToFloat64(counter) - before; got != 1 {
		t.Fatalf("counted %v requests, want 1", got)
	}
}

func TestEnvDefaults(t *testing.T) {
	t.Setenv("TEST_INT", "not-a-number")
	if got := EnvInt("TEST_INT", 7); got != 7 {
		t.Fatalf("EnvInt = %d, want 7", got)
	}
	if got := Env("TEST_UNSET_KEY", "def"); got != "def" {
		t.Fatalf("Env = %q, want def", got)
	}
}

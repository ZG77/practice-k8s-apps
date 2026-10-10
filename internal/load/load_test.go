package load

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestIntParam(t *testing.T) {
	tests := []struct {
		query string
		want  int
	}{
		{"", 100},           // missing -> default
		{"ms=abc", 100},     // invalid -> default
		{"ms=250", 250},     // valid
		{"ms=0", 1},         // below min -> min
		{"ms=999999", 5000}, // above max -> max
	}
	for _, tt := range tests {
		r := httptest.NewRequest("GET", "/cpu?"+tt.query, nil)
		if got := IntParam(r, "ms", 100, 1, 5000); got != tt.want {
			t.Errorf("IntParam(%q) = %d, want %d", tt.query, got, tt.want)
		}
	}
}

func TestBurnTakesAtLeastDuration(t *testing.T) {
	start := time.Now()
	Burn(20 * time.Millisecond)
	if d := time.Since(start); d < 20*time.Millisecond {
		t.Fatalf("Burn took %v, want >= 20ms", d)
	}
}

func TestAllocateHoldsAndReleases(t *testing.T) {
	before := testutil.ToFloat64(heldBytes)
	Allocate(1, 50*time.Millisecond)
	if got := testutil.ToFloat64(heldBytes) - before; got != 1<<20 {
		t.Fatalf("held %v bytes, want %d", got, 1<<20)
	}
	time.Sleep(200 * time.Millisecond)
	if got := testutil.ToFloat64(heldBytes) - before; got != 0 {
		t.Fatalf("after release held %v bytes, want 0", got)
	}
}

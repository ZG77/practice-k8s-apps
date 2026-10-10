// Package load creates CPU and memory load on demand, for autoscaling tests.
package load

import (
	"fmt"
	"math"
	"net/http"
	"runtime/debug"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	maxCPUMillis = 5000
	maxMemMB     = 1024
	maxHoldSec   = 600
)

var (
	heldBytes = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "app_memory_held_bytes",
		Help: "Memory held by /mem requests.",
	})

	mu     sync.Mutex
	held   = map[int][]byte{}
	nextID int

	sink float64 // keeps the compiler from removing the CPU loop
)

// CPUHandler burns CPU. Query: ms (default 100, max 5000).
func CPUHandler(w http.ResponseWriter, r *http.Request) {
	ms := IntParam(r, "ms", 100, 1, maxCPUMillis)
	Burn(time.Duration(ms) * time.Millisecond)
	fmt.Fprintf(w, "burned %dms of CPU\n", ms)
}

// MemHandler allocates memory and holds it. Query: mb (default 50, max 1024), sec (default 30, max 600).
func MemHandler(w http.ResponseWriter, r *http.Request) {
	mb := IntParam(r, "mb", 50, 1, maxMemMB)
	sec := IntParam(r, "sec", 30, 1, maxHoldSec)
	Allocate(mb, time.Duration(sec)*time.Second)
	fmt.Fprintf(w, "holding %dMB for %ds\n", mb, sec)
}

// Burn keeps one CPU core busy for d.
func Burn(d time.Duration) {
	end := time.Now().Add(d)
	x := 0.0
	for time.Now().Before(end) {
		for i := range 10000 {
			x += math.Sqrt(float64(i))
		}
	}
	sink = x
}

// Allocate holds mb megabytes for hold, then releases them.
// Every page is written, so the memory is really used (RSS), not only reserved.
func Allocate(mb int, hold time.Duration) {
	buf := make([]byte, mb<<20)
	for i := 0; i < len(buf); i += 4096 {
		buf[i] = 1
	}

	mu.Lock()
	id := nextID
	nextID++
	held[id] = buf
	mu.Unlock()
	heldBytes.Add(float64(len(buf)))

	time.AfterFunc(hold, func() {
		mu.Lock()
		delete(held, id)
		mu.Unlock()
		heldBytes.Sub(float64(mb << 20))
		debug.FreeOSMemory() // return memory to the OS now, so the drop is visible in metrics
	})
}

// IntParam reads an int query parameter. It returns def if the value is missing or invalid,
// and clamps the result to [lo, hi].
func IntParam(r *http.Request, name string, def, lo, hi int) int {
	v, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil {
		v = def
	}
	return min(max(v, lo), hi)
}

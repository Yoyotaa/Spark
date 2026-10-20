package overlay

import (
	"context"
	"sync"
	"testing"
	"time"
)

type fakePlatform struct {
	mu      sync.Mutex
	x, y    float64
	ignore  bool
	toggles int
}

func (f *fakePlatform) attach() error { return nil }

func (f *fakePlatform) cursor() (float64, float64, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.x, f.y, true
}

func (f *fakePlatform) setIgnoreMouse(v bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ignore = v
	f.toggles++
}

func (f *fakePlatform) focus()        {}
func (f *fakePlatform) restoreFocus() {}

func (f *fakePlatform) move(x, y float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.x, f.y = x, y
}

func (f *fakePlatform) state() (bool, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ignore, f.toggles
}

func waitIgnore(t *testing.T, f *fakePlatform, want bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if got, _ := f.state(); got == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("ignoreMouse never became %v", want)
}

func TestControllerTogglesClickThrough(t *testing.T) {
	f := &fakePlatform{x: 500, y: 500}
	c := NewController()
	c.native = f
	c.interval.Store(int64(time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatal(err)
	}
	waitIgnore(t, f, true)

	c.SetRegions([]Rect{{X: 10, Y: 10, W: 100, H: 50}})
	f.move(50, 30)
	waitIgnore(t, f, false)

	f.move(200, 200)
	waitIgnore(t, f, true)

	c.SetCaptured(true)
	waitIgnore(t, f, false)
	c.SetCaptured(false)
	waitIgnore(t, f, true)

	// Paused: click-through even above a region.
	f.move(50, 30)
	waitIgnore(t, f, false)
	c.SetPaused(true)
	waitIgnore(t, f, true)
	c.SetPaused(false)
	waitIgnore(t, f, false)
	f.move(200, 200)
	waitIgnore(t, f, true)

	// Steady state: no redundant native calls.
	_, before := f.state()
	time.Sleep(20 * time.Millisecond)
	if _, after := f.state(); after != before {
		t.Fatalf("expected no toggles at rest, got %d", after-before)
	}
}

func BenchmarkHit(b *testing.B) {
	regions := []Rect{{0, 0, 10, 10}, {100, 100, 300, 400}, {900, 700, 80, 80}}
	b.ReportAllocs()
	for b.Loop() {
		hit(regions, 950, 750)
	}
}

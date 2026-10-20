// Package overlay turns the Wails window into a transparent, always-on-top,
// click-through layer.
//
// Why polling instead of DOM events: once the window ignores the mouse, the
// webview receives no events at all, so it can never tell us "the cursor is
// back over a button". The OS, however, always knows where the cursor is.
// A goroutine therefore reads the cursor position at ~60 Hz (one cheap native
// call), hit-tests it against the regions reported by the frontend and flips
// the native "ignore mouse" flag only when the answer changes.
package overlay

import (
	"context"
	"sync/atomic"
	"time"
)

// platform is the OS-specific half of the overlay (native_<os>.go).
type platform interface {
	// attach locates the Wails window and applies the overlay styles
	// (transparent, top-most, full screen, excluded from screen capture).
	attach() error
	// cursor returns the cursor position in CSS pixels relative to the
	// window's top-left corner.
	cursor() (x, y float64, ok bool)
	// setIgnoreMouse toggles native click-through.
	setIgnoreMouse(ignore bool)
	// focus gives keyboard focus to the overlay, remembering the app that
	// had it; restoreFocus hands it back.
	focus()
	restoreFocus()
}

// WindowClassName is given to Wails (options.Windows.WindowClassName) so the
// Windows backend finds its own HWND without enumerating every window.
const WindowClassName = "SparkOverlay"

const defaultPollInterval = 16 * time.Millisecond

// Controller owns the click-through state machine.
type Controller struct {
	native   platform
	interval time.Duration

	// regions is swapped atomically by the frontend and read lock-free by
	// the polling loop, so the hot path never allocates nor contends.
	regions atomic.Pointer[[]Rect]
	// OnToggle, if set before Start, is called from the polling goroutine
	// each time click-through changes (debug logging).
	OnToggle func(ignoring bool)

	// captured forces the window to keep the mouse (e.g. chat opened with
	// a hotkey), regardless of the cursor position.
	captured atomic.Bool
	// paused stops cursor polling and keeps the window click-through, used
	// while the overlay is hidden.
	paused atomic.Bool
}

func NewController() *Controller {
	c := &Controller{native: newPlatform(), interval: defaultPollInterval}
	empty := []Rect{}
	c.regions.Store(&empty)
	return c
}

// Start attaches to the native window and runs the polling loop until ctx is
// cancelled. It must be called once the window exists (OnDomReady).
func (c *Controller) Start(ctx context.Context) error {
	if err := c.native.attach(); err != nil {
		return err
	}
	c.native.setIgnoreMouse(true)
	go c.loop(ctx)
	return nil
}

// SetRegions replaces the interactive regions. The slice is owned by the
// controller afterwards.
func (c *Controller) SetRegions(r []Rect) {
	c.regions.Store(&r)
}

// SetCaptured forces (or releases) mouse capture for the whole window.
func (c *Controller) SetCaptured(v bool) {
	c.captured.Store(v)
}

// SetPaused suspends (or resumes) cursor polling.
func (c *Controller) SetPaused(v bool) {
	c.paused.Store(v)
}

// Focus gives keyboard focus to the overlay (chat input).
func (c *Controller) Focus() {
	c.native.focus()
}

// RestoreFocus gives keyboard focus back to the previously active app.
func (c *Controller) RestoreFocus() {
	c.native.restoreFocus()
}

func (c *Controller) setIgnore(v bool) {
	c.native.setIgnoreMouse(v)
	if c.OnToggle != nil {
		c.OnToggle(v)
	}
}

func (c *Controller) loop(ctx context.Context) {
	t := time.NewTicker(c.interval)
	defer t.Stop()

	ignoring := true
	lastX, lastY := -1.0, -1.0
	dirty := true // re-evaluate even if the cursor did not move

	var lastRegions *[]Rect
	var lastCaptured bool

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}

		if c.paused.Load() {
			if !ignoring {
				c.setIgnore(true)
				ignoring = true
			}
			dirty = true
			continue
		}

		x, y, ok := c.native.cursor()
		if !ok {
			continue
		}
		regions := c.regions.Load()
		captured := c.captured.Load()
		if regions != lastRegions || captured != lastCaptured {
			lastRegions, lastCaptured, dirty = regions, captured, true
		}
		if !dirty && x == lastX && y == lastY {
			continue
		}
		lastX, lastY, dirty = x, y, false

		want := !captured && !hit(*regions, x, y)
		if want != ignoring {
			c.setIgnore(want)
			ignoring = want
		}
	}
}

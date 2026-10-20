// Package app is the bridge between Wails and Spark's modules. Exported
// methods of App are bound to the frontend (frontend/wailsjs/go/app).
package app

import (
	"context"
	"log"
	"sync/atomic"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"spark/internal/config"
	"spark/internal/hotkey"
	"spark/internal/overlay"
)

// Events emitted to the frontend (mirrored in frontend/src/lib/bridge.ts).
const (
	EventToggleChat = "aria:toggle-chat"
	EventVisibility = "spark:visibility"
)

type App struct {
	cfg     config.Config
	overlay *overlay.Controller
	visible atomic.Bool

	ctx    context.Context
	cancel context.CancelFunc
}

func New(cfg config.Config) *App {
	ov := overlay.NewController()
	if cfg.Debug {
		ov.OnToggle = func(ignoring bool) { log.Printf("overlay: click-through=%v", ignoring) }
	}
	a := &App{cfg: cfg, overlay: ov}
	a.visible.Store(true)
	return a
}

func (a *App) Startup(ctx context.Context) {
	a.ctx, a.cancel = context.WithCancel(ctx)
}

// DomReady runs once the native window and the webview exist.
func (a *App) DomReady(context.Context) {
	if err := a.overlay.Start(a.ctx); err != nil {
		log.Printf("overlay disabled: %v", err)
	}
	err := hotkey.Listen(a.ctx, []hotkey.Binding{
		{Mod: hotkey.ModAlt | hotkey.ModShift, Key: 'A', Fn: a.toggleChat},
		{Mod: hotkey.ModAlt | hotkey.ModShift, Key: 'H', Fn: a.toggleVisible},
	})
	if err != nil {
		log.Printf("hotkeys disabled: %v", err)
	}
}

// Shutdown only cancels: it must not wait on goroutines that may be
// dispatching to the main thread, which is the one calling Shutdown.
func (a *App) Shutdown(context.Context) {
	a.cancel()
}

func (a *App) debugf(format string, args ...any) {
	if a.cfg.Debug {
		log.Printf(format, args...)
	}
}

func (a *App) toggleChat() {
	a.debugf("hotkey: toggle chat")
	if !a.visible.Load() {
		a.setVisible(true)
	}
	runtime.EventsEmit(a.ctx, EventToggleChat)
}

func (a *App) toggleVisible() {
	a.debugf("hotkey: toggle visibility")
	a.setVisible(!a.visible.Load())
}

func (a *App) setVisible(v bool) {
	a.visible.Store(v)
	a.overlay.SetPaused(!v)
	runtime.EventsEmit(a.ctx, EventVisibility, v)
}

// SetInteractiveRegions is called by the frontend whenever the layout of its
// clickable elements changes.
func (a *App) SetInteractiveRegions(regions []overlay.Rect) {
	a.overlay.SetRegions(regions)
}

// SetMouseCaptured forces the overlay to keep (or release) the mouse.
func (a *App) SetMouseCaptured(v bool) {
	a.overlay.SetCaptured(v)
}

// SetChatOpen takes the keyboard focus when the chat opens and gives it back
// to the previous app (game, browser...) when it closes.
func (a *App) SetChatOpen(open bool) {
	a.debugf("chat open=%v", open)
	if open {
		a.overlay.Focus()
	} else {
		a.overlay.RestoreFocus()
	}
}

func (a *App) Debug() bool {
	return a.cfg.Debug
}

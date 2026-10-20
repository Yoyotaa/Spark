// Package app is the bridge between Wails and Spark's modules. Exported
// methods of App are bound to the frontend (frontend/wailsjs/go/app).
package app

import (
	"context"
	"log"

	"spark/internal/config"
	"spark/internal/overlay"
)

type App struct {
	cfg     config.Config
	overlay *overlay.Controller

	ctx    context.Context
	cancel context.CancelFunc
}

func New(cfg config.Config) *App {
	ov := overlay.NewController()
	if cfg.Debug {
		ov.OnToggle = func(ignoring bool) { log.Printf("overlay: click-through=%v", ignoring) }
	}
	return &App{cfg: cfg, overlay: ov}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx, a.cancel = context.WithCancel(ctx)
}

// DomReady runs once the native window and the webview exist.
func (a *App) DomReady(context.Context) {
	if err := a.overlay.Start(a.ctx); err != nil {
		log.Printf("overlay disabled: %v", err)
	}
}

// Shutdown only cancels: it must not wait on goroutines that may be
// dispatching to the main thread, which is the one calling Shutdown.
func (a *App) Shutdown(context.Context) {
	a.cancel()
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

func (a *App) Debug() bool {
	return a.cfg.Debug
}

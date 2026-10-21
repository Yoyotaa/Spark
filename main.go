package main

import (
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"spark/internal/app"
	"spark/internal/config"
	"spark/internal/overlay"
	"spark/internal/selfresp"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Before anything touches AppKit: on macOS, become the owner of our own
	// privacy permissions when launched from a terminal (see selfresp).
	if code, relaunched := selfresp.Relaunch(); relaunched {
		os.Exit(code)
	}

	settings, err := config.OpenStore()
	if err != nil {
		log.Printf("settings: %v (defaults used)", err)
	}
	a := app.New(config.LoadEnv(), settings)

	err = wails.Run(&options.App{
		Title: "Spark",
		// Placeholder size: the overlay resizes itself to the full screen
		// once attached (see overlay.platform.attach).
		Width:            1280,
		Height:           800,
		Frameless:        true,
		AlwaysOnTop:      true,
		DisableResize:    true,
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        a.Startup,
		OnDomReady:       a.DomReady,
		OnShutdown:       a.Shutdown,
		Bind:             []any{a},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHidden(),
			WebviewIsTransparent: true,
			WindowIsTranslucent:  false,
			ContentProtection:    true,
		},
		Windows: &windows.Options{
			WebviewIsTransparent:              true,
			WindowIsTranslucent:               false,
			BackdropType:                      windows.None,
			DisableFramelessWindowDecorations: true,
			WindowClassName:                   overlay.WindowClassName,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}

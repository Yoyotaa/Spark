package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"spark/internal/app"
	"spark/internal/config"
	"spark/internal/overlay"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	a := app.New(config.Load())

	err := wails.Run(&options.App{
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

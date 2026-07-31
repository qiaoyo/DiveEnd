package main

import (
	"context"
	"embed"

	diveapp "github.com/qiaoyo/DiveEnd/internal/app"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	application := diveapp.NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "DiveEnd",
		Width:     1280,
		Height:    800,
		MinWidth:  1024,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: diveapp.AssetServerHandler(application),
		},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 255},
		OnStartup: func(ctx context.Context) {
			diveapp.Startup(application, ctx)
		},
		OnBeforeClose: func(ctx context.Context) bool {
			return diveapp.BeforeClose(application, ctx)
		},
		OnShutdown: func(ctx context.Context) {
			diveapp.Shutdown(application, ctx)
		},
		Bind: []interface{}{
			application,
		},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarDefault(),
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

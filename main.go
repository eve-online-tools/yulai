package main

import (
	"context"
	"embed"
	"log"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/eve-online-tools/yulai/app"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	cfg, err := app.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	a, err := app.New(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer a.Close()

	var mainWindow *application.WebviewWindow
	wails := application.New(application.Options{
		Name:        cfg.SSO.Name,
		Description: cfg.SSO.Description,
		Services:    a.Services(),
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		// Before a.Start binds the login port, so a second launch focuses this one
		// instead of failing on the busy port.
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "tools.eve-online.yulai",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				mainWindow.Restore()
				mainWindow.Focus()
			},
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	mainWindow = wails.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "Yulai",
		Width:  1100,
		Height: 700,
		// The frontend frame draws the title bar. Windows and Linux get its own controls; macOS keeps
		// the native traffic lights over a transparent title bar so they behave as users expect.
		Frameless: runtime.GOOS != "darwin",
		Mac: application.MacWindow{
			TitleBar: application.MacTitleBar{
				AppearsTransparent:   true,
				HideTitle:            true,
				FullSizeContent:      true,
				UseToolbar:           true,
				HideToolbarSeparator: true,
				ToolbarStyle:         application.MacToolbarStyleUnifiedCompact,
			},
		},
		MinWidth:         720,
		MinHeight:        480,
		BackgroundColour: application.NewRGB(6, 7, 15),
		URL:              "/",
	})

	if err := a.Start(ctx); err != nil {
		log.Fatal(err)
	}

	if err := wails.Run(); err != nil {
		log.Fatal(err)
	}
}

package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"log/slog"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/eve-online-tools/yulai/app"
)

// frontend/dist holds one build per app: yulai (the window) and webserver (the login pages).
//
//go:embed all:frontend/dist
var dist embed.FS

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// run holds main's body so its defers run before the process exits.
func run() error {
	cfg, err := app.LoadConfig()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	assets, err := fs.Sub(dist, "frontend/dist/yulai")
	if err != nil {
		return err
	}
	web, err := fs.Sub(dist, "frontend/dist/webserver")
	if err != nil {
		return err
	}

	a, err := app.New(ctx, cfg, web)
	if err != nil {
		return err
	}
	defer func() {
		if err := a.Close(); err != nil {
			slog.Error("close", "err", err)
		}
	}()

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
		return err
	}
	return wails.Run()
}

package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {

	app := NewApp()

	wailsApp := application.New(application.Options{
		Name:        "Condui",
		Description: "Condui SSH/SFTP/Docker terminal GUI",
		Services: []application.Service{
			application.NewService(app),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Condui",
		Width:            1400,
		Height:           900,
		MinWidth:         1000,
		MinHeight:        700,
		StartState:       application.WindowStateMaximised,
		BackgroundColour: application.NewRGB(17, 24, 39),
		URL:              "/",
		Mac: application.MacWindow{
			TitleBar:   application.MacTitleBarHiddenInset,
			Appearance: application.NSAppearanceNameDarkAqua,
		},
	})

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}

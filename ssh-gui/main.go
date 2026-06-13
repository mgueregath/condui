package main

import (
	"embed"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp(nil)

	a := application.New(application.Options{
		Name:        "Condui",
		Description: "Condui",
		Services: []application.Service{
			application.NewService(app),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	a.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Condui",
		Width:            1400,
		Height:           900,
		MinWidth:         1000,
		MinHeight:        700,
		BackgroundColour: application.NewRGB(17, 24, 39),
		StartState:       application.WindowStateMaximised,
		Mac: application.MacWindow{
			TitleBar: application.MacTitleBar{
				Hide: true,
			},
			CollectionBehavior: application.MacWindowCollectionBehaviorFullScreenPrimary | application.MacWindowCollectionBehaviorFullScreenAuxiliary,
			Backdrop:           application.MacBackdropNormal,
		},
		Windows: application.WindowsWindow{
			BackdropType:                      application.None,
			DisableFramelessWindowDecorations: false,
		},
		Linux: application.LinuxWindow{
			WindowIsTranslucent: false,
		},
		EnableFileDrop: true,
	})

	err := a.Run()
	if err != nil {
		panic(err)
	}
}

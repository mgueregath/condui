package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS


func main() {

	app := NewApp()


	err := wails.Run(
		&options.App{

			Title:
				"Condui",

			Width:
				1400,

			Height:
				900,

			MinWidth:
				1000,

			MinHeight:
				700,


			AssetServer:
				&assetserver.Options{
					Assets: assets,
				},


			BackgroundColour:
				&options.RGBA{
					R: 17,
					G: 24,
					B: 39,
					A: 1,
				},


			WindowStartState:
				options.Maximised,


			OnStartup:
				app.startup,


			Windows:
				&windows.Options{

					WindowIsTranslucent:
						false,

					WebviewIsTransparent:
						false,
				},


			Mac:
				&mac.Options{

					TitleBar:
						mac.TitleBarHiddenInset(),

					Appearance:
						mac.NSAppearanceNameDarkAqua,
				},


			Bind:
				[]interface{}{
					app,
				},
		},
	)


	if err != nil {
		println(
			"Error:",
			err.Error(),
		)
	}
}
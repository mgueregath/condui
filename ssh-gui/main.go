package main

import (
	"embed"
	"log"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"

	"ssh-gui/backend/buildconfig"
)

//go:embed all:frontend/dist
var assets embed.FS

// currentVersion is overridden at release build time via
// -ldflags "-X main.currentVersion=X.Y.Z" (see build/*/Taskfile.yml). Dev
// builds report "dev", which the GitHub provider always treats as
// out-of-date, so "Check for Updates" is exercisable locally too.
var currentVersion = "dev"

// updateAssetMatcher wraps github.DefaultAssetMatcher to exclude
// fresh-install-only artifacts (NSIS installer, .deb, .rpm) from
// consideration. Those assets share the same platform/arch filename tokens
// as the in-app updater's own assets (e.g. "Condui-windows-x64-installer.exe"
// and "Condui-windows-amd64-update.exe" both contain "windows"+"amd64"), so
// without this filter DefaultAssetMatcher could pick whichever one happens
// to come first in the release's asset list and hand the installer to a
// running app to swap itself with — see docs/RELEASING.md.
func updateAssetMatcher(req updater.CheckRequest, assets []github.ReleaseAsset) int {
	candidates := make([]github.ReleaseAsset, 0, len(assets))
	originalIndex := make([]int, 0, len(assets))
	for i, a := range assets {
		name := strings.ToLower(a.Name)
		if strings.Contains(name, "installer") || strings.HasSuffix(name, ".deb") || strings.HasSuffix(name, ".rpm") {
			continue
		}
		candidates = append(candidates, a)
		originalIndex = append(originalIndex, i)
	}
	i := github.DefaultAssetMatcher(req, candidates)
	if i < 0 {
		return -1
	}
	return originalIndex[i]
}

func main() {

	app := NewApp()

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

	gh, err := github.New(github.Config{
		Repository:    buildconfig.Values.UpdateRepo,
		ChecksumAsset: "SHA256SUMS",
		AssetMatcher:  updateAssetMatcher,
	})
	if err != nil {
		log.Fatalf("github.New: %v", err)
	}
	if err := a.Updater.Init(updater.Config{
		CurrentVersion: currentVersion,
		Providers:      []updater.Provider{gh},
	}); err != nil {
		log.Fatalf("Updater.Init: %v", err)
	}
	app.appVersion = currentVersion

	win := a.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Condui",
		Width:            1400,
		Height:           900,
		MinWidth:         1000,
		MinHeight:        700,
		BackgroundColour: application.NewRGB(17, 24, 39),
		StartState:       application.WindowStateMaximised,
		Mac: application.MacWindow{
			TitleBar: application.MacTitleBar{
				AppearsTransparent: true,
				Hide:               false,
				HideTitle:          true,
				FullSizeContent:    true,
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

	win.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
		application.Get().Event.Emit("remote-files-dropped", map[string]any{
			"files":   event.Context().DroppedFiles(),
			"details": event.Context().DropTargetDetails(),
		})
	})

	err = a.Run()
	if err != nil {
		println(
			"Error:",
			err.Error(),
		)
	}
}

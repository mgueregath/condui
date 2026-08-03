package main

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// GetAppVersion returns the version this binary was built with (see
// main.currentVersion), so the frontend can display it without hardcoding it.
func (a *App) GetAppVersion() string {
	return a.appVersion
}

// CheckForUpdates opens the framework's built-in updater window, checks
// GitHub Releases for a newer version, and — if found — downloads, verifies
// and stages it for install. The user applies the update from that window
// (Restart & Apply); this call returns once the window flow settles into
// "Up to Date" or "Update Ready", not after an install completes.
func (a *App) CheckForUpdates() error {
	return application.Get().Updater.CheckAndInstall(context.Background())
}

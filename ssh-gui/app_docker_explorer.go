package main

import (
	"fmt"
	"net/url"

	"github.com/wailsapp/wails/v3/pkg/application"
	"ssh-gui/backend/dockerinfo"
)

func (a *App) OpenDockerExplorerWindow(sessionID string) error {
	if _, ok := a.sessionManager.Get(sessionID); !ok {
		return fmt.Errorf("sesion no encontrada")
	}
	window := application.Get().Window.NewWithOptions(application.WebviewWindowOptions{
		URL:   "/#/docker-explorer?session=" + url.QueryEscape(sessionID),
		Title: "Docker - Condui", Width: 1200, Height: 800, MinWidth: 760, MinHeight: 540,
		BackgroundColour: application.NewRGB(240, 240, 247),
	})
	window.Show()
	return nil
}

func (a *App) GetDockerExplorerSnapshot(sessionID string) (*dockerinfo.ExplorerSnapshot, error) {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return nil, fmt.Errorf("sesion desconectada")
	}
	return dockerinfo.GetExplorerSnapshot(session.Client, session.SudoPassword)
}

func (a *App) DeleteDockerExplorerImage(sessionID, imageID string) error {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return fmt.Errorf("sesion desconectada")
	}
	return dockerinfo.DeleteExplorerImage(session.Client, session.SudoPassword, imageID)
}

func (a *App) ControlDockerExplorerStack(sessionID, project, action string) error {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return fmt.Errorf("sesion desconectada")
	}
	return dockerinfo.ControlExplorerStack(session.Client, session.SudoPassword, project, action)
}

func (a *App) DeleteDockerExplorerVolume(sessionID, name string) error {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return fmt.Errorf("sesion desconectada")
	}
	return dockerinfo.DeleteExplorerVolume(session.Client, session.SudoPassword, name)
}

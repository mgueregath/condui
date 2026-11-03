package main

import (
	"fmt"
	"strings"
)

// HasContainerRuntime checks installation without requiring daemon or sudo access.
func (a *App) HasContainerRuntime(sessionID string) (bool, error) {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok || session.Client == nil {
		return false, fmt.Errorf("SSH session not found")
	}
	cmdSession, err := session.Client.NewSession()
	if err != nil {
		return false, err
	}
	defer cmdSession.Close()
	output, err := cmdSession.Output("if command -v docker >/dev/null 2>&1 || command -v podman >/dev/null 2>&1; then printf 'installed'; fi")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(output)) == "installed", nil
}

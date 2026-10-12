package main

import (
	"bufio"
	"fmt"

	"golang.org/x/crypto/ssh"

	"github.com/wailsapp/wails/v3/pkg/application"

	"ssh-gui/backend/dockerinfo"
	"ssh-gui/backend/models"
)

func (a *App) GetDockerContainers(sessionID string) ([]models.DockerContainer, error) {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return nil, fmt.Errorf("session not found")
	}
	return dockerinfo.GetContainers(session.Client, session.SudoPassword)
}

func (a *App) GetListeningPorts(sessionID string) ([]models.PortInfo, error) {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return nil, fmt.Errorf("session not found")
	}
	return dockerinfo.GetListeningPorts(session.Client)
}

// SetSudoPassword establece la contraseña sudo para una sesión
func (a *App) SetSudoPassword(sessionID string, password string) error {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return fmt.Errorf("session not found")
	}
	session.SudoPassword = password
	return nil
}

// ToggleContainer ejecuta las acciones vitales: start, stop o restart
func (a *App) ToggleContainer(sessionID string, containerID string, action string) (string, error) {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return "", fmt.Errorf("session not found")
	}

	output, err := dockerinfo.ToggleContainer(session.Client, containerID, action, session.SudoPassword)
	if err == nil {
		a.emitLog("DOCKER", fmt.Sprintf("Contenedor %s ejecutó: %s", containerID, action), "success")
	} else {
		a.emitLog("DOCKER", fmt.Sprintf("Error en contenedor %s: %s", containerID, output), "error")
	}
	return output, err
}

// StartDockerLogs abre docker logs -f en el contenedor y emite cada línea como evento Wails
func (a *App) StartDockerLogs(sessionID string, containerID string) error {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return fmt.Errorf("session not found")
	}

	a.StopDockerLogs(sessionID, containerID)

	cmd := fmt.Sprintf("docker logs -f --tail=300 %s 2>&1", containerID)

	// Intenta primero sin sudo
	cmdSession, err := session.Client.NewSession()
	if err != nil {
		return err
	}

	stdout, err := cmdSession.StdoutPipe()
	if err != nil {
		cmdSession.Close()
		return err
	}

	if err := cmdSession.Start(cmd); err != nil {
		cmdSession.Close()

		// Si falla, intenta con sudo -n (non-interactive)
		cmdSession, err = session.Client.NewSession()
		if err != nil {
			return err
		}

		stdout, err = cmdSession.StdoutPipe()
		if err != nil {
			cmdSession.Close()
			return err
		}

		if err := cmdSession.Start("sudo -n " + cmd); err != nil {
			cmdSession.Close()
			return err
		}
	}

	key := sessionID + ":" + containerID
	a.dockerLogMu.Lock()
	if a.dockerLogSessions == nil {
		a.dockerLogSessions = make(map[string]*ssh.Session)
	}
	a.dockerLogSessions[key] = cmdSession
	a.dockerLogMu.Unlock()

	go func() {
		defer func() {
			cmdSession.Close()
			a.dockerLogMu.Lock()
			delete(a.dockerLogSessions, key)
			a.dockerLogMu.Unlock()
			application.Get().Event.Emit("docker-log-end-" + containerID)
		}()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			application.Get().Event.Emit("docker-log-"+containerID, scanner.Text())
		}
	}()

	return nil
}

// StopDockerLogs detiene el stream de logs cerrando la sesión SSH
func (a *App) StopDockerLogs(sessionID string, containerID string) {
	key := sessionID + ":" + containerID
	a.dockerLogMu.Lock()
	sess, ok := a.dockerLogSessions[key]
	if ok {
		delete(a.dockerLogSessions, key)
	}
	a.dockerLogMu.Unlock()
	if ok {
		sess.Close()
	}
}

func (a *App) GetDatabases(sessionID string) ([]models.DatabaseInfo, error) {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return nil, fmt.Errorf("session not found")
	}
	return dockerinfo.GetDatabases(session.Client, session.SudoPassword)
}

// GetDockerStats returns a one-shot snapshot of CPU and memory usage
// for all running containers. Non-running containers are absent from the result.
func (a *App) GetDockerStats(sessionID string) ([]models.DockerStats, error) {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return nil, fmt.Errorf("session not found")
	}
	return dockerinfo.GetStats(session.Client, session.SudoPassword)
}

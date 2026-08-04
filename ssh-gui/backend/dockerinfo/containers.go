package dockerinfo

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"

	"ssh-gui/backend/models"
)

// GetContainers lists all docker containers (running and stopped) on the remote host.
func GetContainers(client *ssh.Client, sudoPassword string) ([]models.DockerContainer, error) {
	cmd := "docker ps -a --format '{\"id\":\"{{.ID}}\",\"names\":\"{{.Names}}\",\"image\":\"{{.Image}}\",\"status\":\"{{.Status}}\",\"state\":\"{{.State}}\",\"ports\":\"{{.Ports}}\"}'"

	// Intenta primero sin sudo
	cmdSession, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer cmdSession.Close()

	output, err := cmdSession.Output(cmd)

	// Si falla, intenta con sudo
	if err != nil {
		cmdSession.Close()

		cmdSession2, err2 := client.NewSession()
		if err2 != nil {
			return nil, fmt.Errorf("docker no responde: %v", err)
		}
		defer cmdSession2.Close()

		var sudoCmd string
		if sudoPassword != "" {
			// Usa sudo -S con contraseña
			sudoCmd = fmt.Sprintf("echo '%s' | sudo -S %s", sudoPassword, cmd)
		} else {
			// Intenta sudo -n (sin contraseña)
			sudoCmd = "sudo -n " + cmd
		}

		output, err = cmdSession2.Output(sudoCmd)
		if err != nil {
			if sudoPassword == "" {
				return nil, fmt.Errorf("SUDO_PASSWORD_REQUIRED")
			}
			return nil, fmt.Errorf("docker no responde, verifique la contraseña sudo")
		}
	}

	lines := strings.Split(string(output), "\n")

	var containers []models.DockerContainer

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var c models.DockerContainer
		c.ID = extractJSONField(line, "id")
		c.Names = extractJSONField(line, "names")
		c.Image = extractJSONField(line, "image")
		c.Status = extractJSONField(line, "status")
		c.State = extractJSONField(line, "state")
		c.Ports = extractJSONField(line, "ports")

		if c.ID != "" {
			containers = append(containers, c)
		}
	}

	return containers, nil
}

// ToggleContainer ejecuta las acciones vitales: start, stop o restart
func ToggleContainer(client *ssh.Client, containerID string, action string, sudoPassword string) (string, error) {
	validActions := map[string]bool{"start": true, "stop": true, "restart": true}
	if !validActions[action] {
		return "", fmt.Errorf("acción inválida")
	}

	cmd := fmt.Sprintf("docker %s %s", action, containerID)

	// Intenta primero sin sudo
	cmdSession, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer cmdSession.Close()

	output, err := cmdSession.CombinedOutput(cmd)

	// Si falla, intenta con sudo
	if err != nil {
		cmdSession.Close()

		cmdSession2, err2 := client.NewSession()
		if err2 != nil {
			return string(output), err
		}
		defer cmdSession2.Close()

		var sudoCmd string
		if sudoPassword != "" {
			sudoCmd = fmt.Sprintf("echo '%s' | sudo -S %s", sudoPassword, cmd)
		} else {
			sudoCmd = "sudo -n " + cmd
		}

		output, err = cmdSession2.CombinedOutput(sudoCmd)
	}

	return string(output), err
}

// Función utilitaria limpia para parsear los campos string JSON devueltos por el comando docker sin romper el flujo
func extractJSONField(jsonStr, field string) string {
	key := fmt.Sprintf("\"%s\":\"", field)
	idx := strings.Index(jsonStr, key)
	if idx == -1 {
		return ""
	}
	start := idx + len(key)
	end := strings.Index(jsonStr[start:], "\"")
	if end == -1 {
		return ""
	}
	return jsonStr[start : start+end]
}

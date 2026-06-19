package dockerinfo

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"

	"ssh-gui/backend/models"
)

// GetContainers lists all docker containers (running and stopped) on the remote host.
func GetContainers(client *ssh.Client) ([]models.DockerContainer, error) {
	cmdSession, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer cmdSession.Close()

	output, err := cmdSession.Output("docker ps -a --format '{\"id\":\"{{.ID}}\",\"names\":\"{{.Names}}\",\"image\":\"{{.Image}}\",\"status\":\"{{.Status}}\",\"state\":\"{{.State}}\",\"ports\":\"{{.Ports}}\"}'")
	if err != nil {
		return nil, fmt.Errorf("docker no responde, puede que no esté instalado en el servidor remoto o requiera privilegios de sudo")
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
func ToggleContainer(client *ssh.Client, containerID string, action string) (string, error) {
	cmdSession, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer cmdSession.Close()

	validActions := map[string]bool{"start": true, "stop": true, "restart": true}
	if !validActions[action] {
		return "", fmt.Errorf("acción inválida")
	}

	output, err := cmdSession.CombinedOutput(fmt.Sprintf("docker %s %s", action, containerID))
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

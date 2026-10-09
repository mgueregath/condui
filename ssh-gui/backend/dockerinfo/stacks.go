package dockerinfo

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

// Stack actions operate on existing Compose containers, without recreating services.
func ControlExplorerStack(client *ssh.Client, password, project, action string) error {
	if project == "" {
		return fmt.Errorf("proyecto Compose requerido")
	}
	if action != "start" && action != "stop" && action != "restart" {
		return fmt.Errorf("acción inválida")
	}
	snapshot, err := GetExplorerSnapshot(client, password)
	if err != nil {
		return err
	}
	var failures []string
	found := false
	for _, container := range snapshot.Containers {
		labels := container.Config.Labels
		if labels["com.docker.compose.project"] != project || labels["com.docker.stack.namespace"] != "" {
			continue
		}
		found = true
		running := container.State.Status == "running"
		if (action == "start" && running) || (action != "start" && !running) {
			continue
		}
		// IDs come from Docker inspect and are quoted before reaching the remote shell.
		if _, err := explorerCommand(client, password, "docker container "+action+" "+shellQuote(container.ID)); err != nil {
			failures = append(failures, strings.TrimPrefix(container.Name, "/"))
		}
	}
	if !found {
		return fmt.Errorf("stack no encontrado; actualice el listado")
	}
	if len(failures) > 0 {
		return fmt.Errorf("no se pudo completar la acción en: %s; algunos contenedores pueden haber cambiado de estado", strings.Join(failures, ", "))
	}
	return nil
}

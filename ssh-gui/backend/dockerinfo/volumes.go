package dockerinfo

import (
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/crypto/ssh"
)

type ExplorerMount struct {
	Type        string
	Name        string
	Destination string
}
type ExplorerVolume struct {
	Name       string
	Driver     string
	Scope      string
	CreatedAt  string
	Mountpoint string
}

var explorerVolumeName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

func DeleteExplorerVolume(client *ssh.Client, password, name string) error {
	if !explorerVolumeName.MatchString(name) {
		return fmt.Errorf("nombre de volumen invalido")
	}
	snapshot, err := GetExplorerSnapshot(client, password)
	if err != nil {
		return err
	}
	found := false
	for _, volume := range snapshot.Volumes {
		if volume.Name == name {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("el volumen ya no existe; actualice el listado")
	}
	// Include stopped containers: their persisted data must remain protected.
	for _, container := range snapshot.Containers {
		for _, mount := range container.Mounts {
			if mount.Type == "volume" && mount.Name == name {
				return fmt.Errorf("volumen referenciado por el contenedor %s", strings.TrimPrefix(container.Name, "/"))
			}
		}
	}
	// Docker checks references again to protect against concurrent container creation.
	if _, err := explorerCommand(client, password, "docker volume rm "+shellQuote(name)); err != nil {
		return fmt.Errorf("Docker rechazo la eliminacion del volumen; actualice y compruebe referencias y permisos")
	}
	return nil
}

package dockerinfo

import (
	"fmt"
	"regexp"

	"golang.org/x/crypto/ssh"
)

func markNetworkUsage(snapshot *ExplorerSnapshot) {
	referenced := make(map[string]bool)
	for _, container := range snapshot.Containers {
		for _, network := range container.NetworkSettings.Networks {
			if network.NetworkID != "" {
				referenced[network.NetworkID] = true
			}
		}
	}
	for i := range snapshot.Networks {
		network := &snapshot.Networks[i]
		network.InUse = len(network.Containers) > 0 || referenced[network.ID]
		network.Protected = network.Ingress || network.Name == "bridge" || network.Name == "host" || network.Name == "none"
	}
}

var explorerNetworkID = regexp.MustCompile("^[a-z0-9]{12,64}$")

func DeleteExplorerNetwork(client *ssh.Client, password, id string) error {
	if !explorerNetworkID.MatchString(id) {
		return fmt.Errorf("ID de red invalido")
	}
	snapshot, err := GetExplorerSnapshot(client, password)
	if err != nil {
		return err
	}
	for _, network := range snapshot.Networks {
		if network.ID != id {
			continue
		}
		if network.InUse || network.Protected {
			return fmt.Errorf("la red esta en uso o protegida")
		}
		if _, err := explorerCommand(client, password, "docker network rm "+shellQuote(id)); err != nil {
			return fmt.Errorf("Docker rechazo la eliminacion de la red; compruebe sus referencias y permisos")
		}
		return nil
	}
	return fmt.Errorf("la red ya no existe; actualice el listado")
}

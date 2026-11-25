package dockerinfo

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"

	"golang.org/x/crypto/ssh"
)

type ExplorerComposeFile struct {
	Content  string
	Location string
}

var portainerComposePath = regexp.MustCompile("^/data/compose/[0-9]+/.+")
var portainerImage = regexp.MustCompile("^((docker[.]io|index[.]docker[.]io)/)?portainer/(portainer|portainer-ce|portainer-ee)([:@].+)?$")

var portainerAgentImage = regexp.MustCompile("^((docker[.]io|index[.]docker[.]io)/)?portainer/agent([:@].+)?$")

func localPortainerServer(containers []ExplorerContainer) (*ExplorerContainer, bool) {
	var server *ExplorerContainer
	agent := false
	ambiguous := false
	for i := range containers {
		container := &containers[i]
		image := container.Config.Image
		agent = agent || portainerAgentImage.MatchString(image)
		if !portainerImage.MatchString(image) {
			continue
		}
		for _, mount := range container.Mounts {
			if mount.Destination == "/data" && (mount.Type == "volume" || mount.Type == "bind") {
				ambiguous = ambiguous || server != nil
				server = container
				break
			}
		}
	}
	if ambiguous {
		return nil, agent
	}
	return server, agent
}

func ReadPortainerComposeFile(client *ssh.Client, password, filename string) (*ExplorerComposeFile, error) {
	filename = path.Clean(filename)
	if !portainerComposePath.MatchString(filename) {
		return nil, fmt.Errorf("ruta de Compose de Portainer invalida")
	}
	snapshot, err := GetExplorerSnapshot(client, password)
	if err != nil {
		return nil, err
	}
	portainer, _ := localPortainerServer(snapshot.Containers)
	if portainer == nil {
		return nil, fmt.Errorf("no se pudo identificar un servidor local de Portainer")
	}
	knownPath := false
	for _, container := range snapshot.Containers {
		labels := container.Config.Labels
		for _, entry := range strings.Split(labels["com.docker.compose.project.config_files"], ",") {
			entry = strings.TrimSpace(entry)
			if entry == "" || entry == "-" {
				continue
			}
			if !path.IsAbs(entry) {
				entry = path.Join(labels["com.docker.compose.project.working_dir"], entry)
			}
			knownPath = knownPath || path.Clean(entry) == filename
		}
	}
	if !knownPath {
		return nil, fmt.Errorf("archivo de Compose no localizado en Portainer")
	}
	archive, err := explorerCommand(client, password, "docker cp "+shellQuote(portainer.ID+":"+filename)+" -")
	if err != nil {
		return nil, err
	}
	reader := tar.NewReader(bytes.NewReader(archive))
	header, err := reader.Next()
	if err != nil || header.Typeflag != tar.TypeReg || path.Base(header.Name) != path.Base(filename) {
		return nil, fmt.Errorf("archivo de Compose invalido")
	}
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return &ExplorerComposeFile{
		Content:  string(content),
		Location: strings.TrimPrefix(portainer.Name, "/") + ":" + filename,
	}, nil
}

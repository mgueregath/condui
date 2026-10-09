package dockerinfo

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/crypto/ssh"
)

type ExplorerContainer struct {
	ID     string `json:"Id"`
	Name   string
	Image  string
	Config struct {
		Image  string
		Labels map[string]string
	}
	State           struct{ Status string }
	NetworkSettings struct{ Ports map[string]json.RawMessage }
}
type ExplorerImage struct {
	ID       string `json:"Id"`
	RepoTags []string
	Created  string
	Size     int64
}
type ExplorerSnapshot struct {
	Containers []ExplorerContainer
	Images     []ExplorerImage
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

// Run the whole read operation under the same Docker permissions.
func explorerCommand(client *ssh.Client, password, command string) ([]byte, error) {
	run := func(sudo bool) ([]byte, error) {
		session, err := client.NewSession()
		if err != nil {
			return nil, err
		}
		defer session.Close()
		cmd := "sh -c " + shellQuote(command)
		if sudo {
			if password == "" {
				cmd = "sudo -n " + cmd
			} else {
				cmd = "sudo -S -p '' " + cmd
				session.Stdin = strings.NewReader(password + "\n")
			}
		}
		return session.Output(cmd)
	}
	output, err := run(false)
	if err == nil {
		return output, nil
	}
	output, err = run(true)
	if err != nil {
		return nil, fmt.Errorf("no se pudo consultar Docker; compruebe la conexión y los permisos sudo")
	}
	return output, nil
}

func GetExplorerSnapshot(client *ssh.Client, password string) (*ExplorerSnapshot, error) {
	// Empty lists must not invoke inspect without arguments.
	command := `containers=$(docker container ls -aq) || exit 1
images=$(docker image ls -aq --no-trunc) || exit 1
images=$(printf '%s\n' "$images" | sort -u) || exit 1
printf '{"Containers":'
if [ -n "$containers" ]; then docker container inspect $containers || exit 1; else printf '[]'; fi
printf ',"Images":'
if [ -n "$images" ]; then docker image inspect $images || exit 1; else printf '[]'; fi
printf '}'`
	output, err := explorerCommand(client, password, command)
	if err != nil {
		return nil, err
	}
	var snapshot ExplorerSnapshot
	if err := json.Unmarshal(output, &snapshot); err != nil {
		return nil, fmt.Errorf("respuesta de Docker inválida: %w", err)
	}
	// Expose only the metadata needed by the browser, never container environment variables.
	return &snapshot, nil
}

var explorerImageID = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

func DeleteExplorerImage(client *ssh.Client, password, id string) error {
	if !explorerImageID.MatchString(id) {
		return fmt.Errorf("ID de imagen inválido")
	}
	snapshot, err := GetExplorerSnapshot(client, password)
	if err != nil {
		return err
	}
	found := false
	for _, image := range snapshot.Images {
		if image.ID == id {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("la imagen ya no existe; actualice el listado")
	}
	for _, container := range snapshot.Containers {
		if container.Image == id {
			return fmt.Errorf("imagen referenciada por el contenedor %s", strings.TrimPrefix(container.Name, "/"))
		}
	}
	// Docker performs the final reference check; never force removal.
	_, err = explorerCommand(client, password, "docker image rm "+shellQuote(id))
	if err != nil {
		return fmt.Errorf("Docker rechazó la eliminación; actualice el listado y compruebe las referencias y permisos")
	}
	return nil
}

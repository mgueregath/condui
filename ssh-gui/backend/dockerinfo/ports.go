package dockerinfo

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"

	"ssh-gui/backend/models"
)

// GetListeningPorts returns the TCP/UDP ports currently listening on the remote host.
func GetListeningPorts(client *ssh.Client) ([]models.PortInfo, error) {
	cmdSession, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer cmdSession.Close()

	// Extrae proto, puerto, dirección y proceso de ss -tlnp / ss -ulnp
	awkProg := `NR>1{n=split($4,addr,":");port=addr[n];line=$0;proc="-";` +
		`if(index(line,"users:((\"")>0){sub(/.*users:\(\("/, "",line);sub(/".*/, "",line);proc=line};` +
		`print proto" "port" "$4" "proc}`

	cmd := `ss -tlnp 2>/dev/null | awk -v proto=TCP '` + awkProg + `'` +
		`; ss -ulnp 2>/dev/null | awk -v proto=UDP '` + awkProg + `'`

	output, err := cmdSession.Output(cmd)
	if err != nil {
		return nil, err
	}

	var ports []models.PortInfo
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		var port int
		fmt.Sscanf(parts[1], "%d", &port)
		if port == 0 {
			continue
		}
		proc := "-"
		if len(parts) >= 4 {
			proc = parts[3]
		}
		ports = append(ports, models.PortInfo{
			Proto:   parts[0],
			Port:    port,
			Address: parts[2],
			Process: proc,
		})
	}

	return ports, nil
}

package vboxinfo

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"

	"ssh-gui/backend/models"
)

// IsAvailable returns true if VBoxManage is present on the remote host.
func IsAvailable(client *ssh.Client) bool {
	s, err := client.NewSession()
	if err != nil {
		return false
	}
	defer s.Close()
	_, err = s.Output("which VBoxManage 2>/dev/null || which vboxmanage 2>/dev/null")
	return err == nil
}

// GetVMs lists all VMs with state and hardware specs.
// A single SSH session runs a shell pipeline to avoid N+1 round trips.
func GetVMs(client *ssh.Client) ([]models.VMInfo, error) {
	s, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer s.Close()

	// For each VM, call showvminfo once and guestproperty for the IP.
	// Fields separated by | so we can split safely.
	// Output per VM: uuid|name|state|memMB|cpus|ostype|ip|autostart
	cmd := `VBoxManage list vms 2>/dev/null | awk -F'"' '{print $2}' | while IFS= read -r name; do` +
		` [ -z "$name" ] && continue;` +
		` info=$(VBoxManage showvminfo "$name" --machinereadable 2>/dev/null);` +
		` uuid=$(printf '%s' "$info" | grep '^UUID=' | head -1 | sed 's/UUID=//;s/"//g');` +
		` state=$(printf '%s' "$info" | grep '^VMState=' | head -1 | sed 's/VMState=//;s/"//g');` +
		` mem=$(printf '%s' "$info" | grep '^memory=' | head -1 | sed 's/memory=//');` +
		` cpus=$(printf '%s' "$info" | grep '^cpus=' | head -1 | sed 's/cpus=//');` +
		` os=$(printf '%s' "$info" | grep '^ostype=' | head -1 | sed 's/ostype=//;s/"//g');` +
		` autostart=$(printf '%s' "$info" | grep '^autostart-enabled=' | head -1 | sed 's/autostart-enabled=//;s/"//g');` +
		` ip=$(VBoxManage guestproperty get "$name" /VirtualBox/GuestInfo/Net/0/V4/IP 2>/dev/null | grep '^Value:' | sed 's/Value: //');` +
		` printf '%s|%s|%s|%s|%s|%s|%s|%s\n' "$uuid" "$name" "$state" "$mem" "$cpus" "$os" "$ip" "$autostart";` +
		` done`

	out, err := s.Output(cmd)
	if err != nil {
		return nil, fmt.Errorf("VirtualBox no está disponible en el servidor remoto")
	}

	var vms []models.VMInfo
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 8)
		if len(parts) < 3 {
			continue
		}
		field := func(i int) string {
			if i < len(parts) {
				return strings.TrimSpace(parts[i])
			}
			return ""
		}

		name := field(1)
		if name == "" {
			continue
		}

		mem, _ := strconv.Atoi(field(3))
		cpus, _ := strconv.Atoi(field(4))
		autostart := strings.ToLower(field(7)) == "on"

		vms = append(vms, models.VMInfo{
			Name:      name,
			UUID:      field(0),
			State:     normalizeState(field(2)),
			MemoryMB:  mem,
			CPUs:      cpus,
			OS:        friendlyOS(field(5)),
			IP:        field(6),
			Autostart: autostart,
		})
	}
	return vms, nil
}

// ControlVM executes a VBoxManage action on a VM identified by name.
// Allowed actions: start-gui, start-headless, stop-acpi, stop-force,
// pause, resume, reset, savestate.
func ControlVM(client *ssh.Client, vmName, action string) (string, error) {
	cmd, err := buildCommand(vmName, action)
	if err != nil {
		return "", err
	}

	s, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer s.Close()

	out, err := s.CombinedOutput(cmd)
	return strings.TrimSpace(string(out)), err
}

func buildCommand(name, action string) (string, error) {
	safe := strings.ReplaceAll(name, `"`, `\"`)
	switch action {
	case "start-gui":
		return fmt.Sprintf(`VBoxManage startvm "%s" --type gui`, safe), nil
	case "start-headless":
		return fmt.Sprintf(`VBoxManage startvm "%s" --type headless`, safe), nil
	case "stop-acpi":
		return fmt.Sprintf(`VBoxManage controlvm "%s" acpipowerbutton`, safe), nil
	case "stop-force":
		return fmt.Sprintf(`VBoxManage controlvm "%s" poweroff`, safe), nil
	case "pause":
		return fmt.Sprintf(`VBoxManage controlvm "%s" pause`, safe), nil
	case "resume":
		return fmt.Sprintf(`VBoxManage controlvm "%s" resume`, safe), nil
	case "reset":
		return fmt.Sprintf(`VBoxManage controlvm "%s" reset`, safe), nil
	case "savestate":
		return fmt.Sprintf(`VBoxManage controlvm "%s" savestate`, safe), nil
	default:
		return "", fmt.Errorf("acción desconocida: %q", action)
	}
}

func normalizeState(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "running":
		return "running"
	case "paused":
		return "paused"
	case "saved":
		return "saved"
	case "aborted":
		return "aborted"
	case "poweroff", "powered off":
		return "stopped"
	case "gurumeditation":
		return "error"
	default:
		if raw == "" {
			return "stopped"
		}
		return strings.ToLower(raw)
	}
}

// friendlyOS converts VBoxManage ostype identifiers to readable strings.
func friendlyOS(raw string) string {
	m := map[string]string{
		"Ubuntu_64": "Ubuntu 64-bit", "Ubuntu": "Ubuntu 32-bit",
		"Debian_64": "Debian 64-bit", "Debian": "Debian 32-bit",
		"RedHat_64": "Red Hat 64-bit", "RedHat": "Red Hat 32-bit",
		"Fedora_64": "Fedora 64-bit", "Fedora": "Fedora 32-bit",
		"Windows11_64": "Windows 11", "Windows10_64": "Windows 10",
		"Windows7_64": "Windows 7 64-bit", "Windows7": "Windows 7",
		"WindowsNT_64": "Windows Server", "MacOS_64": "macOS",
		"OpenSUSE_64": "openSUSE 64-bit", "ArchLinux_64": "Arch Linux",
	}
	if v, ok := m[raw]; ok {
		return v
	}
	if raw == "" {
		return ""
	}
	// Strip trailing _64 / _32 for unknown types
	raw = strings.TrimSuffix(raw, "_64")
	raw = strings.TrimSuffix(raw, "_32")
	return raw
}

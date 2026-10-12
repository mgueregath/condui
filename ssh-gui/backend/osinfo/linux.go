package osinfo

import (
	"strings"

	"golang.org/x/crypto/ssh"
)

// LinuxDistribution contains the identity reported by os-release and its family.
type LinuxDistribution struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Family string `json:"family"`
}

var linuxFamilies = map[string]string{
	"debian": "debian", "ubuntu": "debian", "linuxmint": "debian",
	"pop": "debian", "kali": "debian", "raspbian": "debian",
	"neon": "debian", "elementary": "debian", "zorin": "debian",
	"rhel": "rhel", "centos": "rhel", "rocky": "rhel",
	"almalinux": "rhel", "ol": "rhel", "scientific": "rhel",
	"fedora": "fedora", "amzn": "amazon",
	"arch": "arch", "manjaro": "arch", "endeavouros": "arch",
	"garuda": "arch", "artix": "arch",
	"suse": "suse", "sles": "suse", "sled": "suse",
	"opensuse": "suse", "opensuse-leap": "suse", "opensuse-tumbleweed": "suse",
	"alpine": "alpine", "gentoo": "gentoo", "funtoo": "gentoo",
	"slackware": "slackware", "nixos": "nixos", "void": "void",
}

// LinuxFamily prefers the distribution itself, then ID_LIKE in declared order.
// Fedora and Amazon remain separate because they are not RHEL derivatives.
func LinuxFamily(id, idLike string) string {
	ids := append([]string{strings.ToLower(strings.TrimSpace(id))}, strings.Fields(strings.ToLower(idLike))...)
	for _, candidate := range ids {
		if family, ok := linuxFamilies[candidate]; ok {
			return family
		}
	}
	return "unknown"
}

// DetectRemoteLinuxDistribution reads metadata without sourcing remote shell code.
// Call only for a remote host identified as Linux.
func DetectRemoteLinuxDistribution(client *ssh.Client) (LinuxDistribution, error) {
	output, err := runSSHCommand(client, "cat /etc/os-release 2>/dev/null || cat /usr/lib/os-release")
	if err != nil {
		return LinuxDistribution{Family: "unknown"}, err
	}

	values := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		values[key] = value
	}

	return LinuxDistribution{
		ID:     values["ID"],
		Name:   values["NAME"],
		Family: LinuxFamily(values["ID"], values["ID_LIKE"]),
	}, nil
}

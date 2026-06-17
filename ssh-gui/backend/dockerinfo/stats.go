package dockerinfo

import (
	"strings"

	"golang.org/x/crypto/ssh"

	"ssh-gui/backend/models"
)

// GetStats runs docker stats --no-stream for all running containers and returns
// a snapshot of CPU and memory usage. Non-running containers are absent from
// the result — the frontend should treat a missing entry as "n/a".
func GetStats(client *ssh.Client) ([]models.DockerStats, error) {
	s, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer s.Close()

	out, err := s.Output(
		`docker stats --no-stream --format "{{.ID}}|{{.CPUPerc}}|{{.MemUsage}}|{{.MemPerc}}" 2>/dev/null`,
	)
	if err != nil {
		// docker not available or no running containers — return empty, not an error
		return nil, nil
	}

	var stats []models.DockerStats
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) != 4 {
			continue
		}
		stats = append(stats, models.DockerStats{
			ID:       strings.TrimSpace(parts[0]),
			CPUPerc:  strings.TrimSpace(parts[1]),
			MemUsage: strings.TrimSpace(parts[2]),
			MemPerc:  strings.TrimSpace(parts[3]),
		})
	}
	return stats, nil
}

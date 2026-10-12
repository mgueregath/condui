package systeminfo

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type CPUCore struct {
	ID      string  `json:"id"`
	Percent float64 `json:"percent"`
}

type Partition struct {
	Device      string  `json:"device"`
	MountPoint  string  `json:"mountPoint"`
	UsedGB      float64 `json:"usedGB"`
	TotalGB     float64 `json:"totalGB"`
	AvailableGB float64 `json:"availableGB"`
	UsedPercent float64 `json:"usedPercent"`
}

// PartitionCache belongs to one SSH session. The caller holds SystemStatsMu.
type PartitionCache struct {
	Partitions  []Partition
	UpdatedAt   time.Time
	attemptedAt time.Time
}

func (c *PartitionCache) Load(client *ssh.Client) {
	if time.Since(c.attemptedAt) < time.Minute {
		return
	}
	c.attemptedAt = time.Now()
	session, err := client.NewSession()
	if err != nil {
		return
	}
	defer session.Close()
	timeout := time.AfterFunc(10*time.Second, func() { session.Close() })
	defer timeout.Stop()
	output, err := session.Output("LC_ALL=C /bin/df -Pk -l")
	if err != nil {
		return
	}
	partitions, err := parsePartitions(string(output))
	if err != nil {
		return
	}
	c.Partitions = partitions
	c.UpdatedAt = time.Now()
}

func parsePartitions(output string) ([]Partition, error) {
	partitions := make([]Partition, 0)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("missing partition data")
	}
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			return nil, fmt.Errorf("invalid partition data")
		}
		switch fields[0] {
		case "tmpfs", "devtmpfs", "devfs", "udev", "none":
			continue
		}
		var values [4]float64
		for i, field := range fields[1:5] {
			value, err := strconv.ParseFloat(strings.TrimSuffix(field, "%"), 64)
			if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
				return nil, fmt.Errorf("invalid partition value")
			}
			values[i] = value
		}
		if values[0] == 0 {
			continue
		}
		if values[0] < 0 || values[1] < 0 || values[1] > values[0] || values[3] < 0 {
			return nil, fmt.Errorf("partition values out of range")
		}
		const kbPerGB = 1048576.0
		partitions = append(partitions, Partition{
			Device: fields[0], MountPoint: strings.Join(fields[5:], " "),
			TotalGB: values[0] / kbPerGB, UsedGB: values[1] / kbPerGB,
			AvailableGB: math.Max(0, values[2]) / kbPerGB, UsedPercent: values[3],
		})
	}
	return partitions, nil
}

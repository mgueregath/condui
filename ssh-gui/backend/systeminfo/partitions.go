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
	Filesystem  string  `json:"filesystem"`
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

func (c *PartitionCache) Load(client *ssh.Client, macOS bool) {
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
	mountCommand := "/bin/cat /proc/self/mounts"
	if macOS {
		mountCommand = "/sbin/mount"
	}
	command := "/bin/sh -c 'LC_ALL=C /bin/df -Pk -l && { echo; " + mountCommand + " || :; }'"
	output, err := session.Output(command)
	if err != nil {
		return
	}
	usage, mounts, _ := strings.Cut(string(output), "\n\n")
	partitions, err := parsePartitions(usage)
	if err != nil {
		return
	}
	formats := partitionFormats(mounts, macOS)
	for i := range partitions {
		partitions[i].Filesystem = formats[partitions[i].MountPoint]
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

func partitionFormats(output string, macOS bool) map[string]string {
	formats := make(map[string]string)
	unescape := strings.NewReplacer("\\040", " ", "\\011", "\t", "\\012", "\n", "\\134", "\\")
	for _, line := range strings.Split(output, "\n") {
		var mountPoint, filesystem string
		if macOS {
			_, mount, ok := strings.Cut(line, " on ")
			optionsStart := strings.LastIndex(mount, " (")
			if !ok || optionsStart < 0 || !strings.HasSuffix(mount, ")") {
				continue
			}
			mountPoint = mount[:optionsStart]
			filesystem, _, _ = strings.Cut(mount[optionsStart+2:len(mount)-1], ",")
		} else {
			fields := strings.Fields(line)
			if len(fields) < 3 {
				continue
			}
			mountPoint, filesystem = unescape.Replace(fields[1]), fields[2]
		}
		formats[strings.Join(strings.Fields(mountPoint), " ")] = filesystem
	}
	return formats
}

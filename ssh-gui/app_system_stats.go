package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"ssh-gui/backend/osinfo"
	"ssh-gui/backend/systeminfo"
)

type SystemStats struct {
	CPUCores            []systeminfo.CPUCore   `json:"cpuCores"`
	MemReservedGB       *float64               `json:"memReservedGB"`
	MemBuffersGB        float64                `json:"memBuffersGB"`
	MemAvailableGB      float64                `json:"memAvailableGB"`
	Partitions          []systeminfo.Partition `json:"partitions"`
	PartitionsUpdatedAt int64                  `json:"partitionsUpdatedAt"`
	CPUPercent          float64                `json:"cpuPercent"`
	MemUsedGB           float64                `json:"memUsedGB"`
	MemFreeGB           float64                `json:"memFreeGB"`
	MemTotalGB          float64                `json:"memTotalGB"`
	DiskUsedGB          float64                `json:"diskUsedGB"`
	DiskFreeGB          float64                `json:"diskFreeGB"`
	DiskTotalGB         float64                `json:"diskTotalGB"`
	UptimeSecs          float64                `json:"uptimeSecs"`
	NetRxBps            float64                `json:"netRxBps"`
	NetTxBps            float64                `json:"netTxBps"`
	DiskReadBps         float64                `json:"diskReadBps"`
	DiskWriteBps        float64                `json:"diskWriteBps"`
}

func (a *App) GetSystemStats(sessionID string) (*SystemStats, error) {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok || session.Client == nil {
		return nil, fmt.Errorf("SSH session not found")
	}
	session.SystemStatsMu.Lock()
	defer session.SystemStatsMu.Unlock()
	remoteOS, err := a.DetectRemoteOS(sessionID)
	if err != nil {
		return nil, err
	}
	var command string
	diskUnit := 512.0
	switch remoteOS {
	case osinfo.OSLinux:
		command = systeminfo.LinuxCommand
	case osinfo.OSDarwin:
		command = systeminfo.MacOSCommand
		diskUnit = 1
	default:
		return nil, fmt.Errorf("system monitoring is not supported for %s", remoteOS)
	}
	// SSH executes commands through the user's shell; explicitly select POSIX sh.
	command = "/bin/sh -c '" + strings.ReplaceAll(command, "'", "'\"'\"'") + "'"

	cmdSession, err := session.Client.NewSession()
	if err != nil {
		return nil, err
	}
	defer cmdSession.Close()
	timeout := time.AfterFunc(15*time.Second, func() { cmdSession.Close() })
	output, err := cmdSession.Output(command)
	timeout.Stop()
	if err != nil {
		return nil, fmt.Errorf("collect system stats: %w", err)
	}
	parsed, err := parseSystemStats(string(output), diskUnit)
	if err != nil {
		return nil, err
	}
	if remoteOS == osinfo.OSDarwin {
		coreSession, coreErr := session.Client.NewSession()
		if coreErr == nil {
			coreTimeout := time.AfterFunc(5*time.Second, func() { coreSession.Close() })
			coreOutput, coreErr := coreSession.Output("/bin/sh -c '" + strings.ReplaceAll(systeminfo.MacOSCoreCommand, "'", "'\"'\"'") + "'")
			coreTimeout.Stop()
			coreSession.Close()
			if coreErr == nil {
				// Preserve the valid memory record while parsing optional core rows.
				details := &SystemStats{MemTotalGB: parsed.MemTotalGB}
				rows := append([]string{"MEM 0 0 0"}, strings.Split(strings.TrimSpace(string(coreOutput)), "\n")...)
				if parseSystemDetails(details, rows) == nil {
					parsed.CPUCores = details.CPUCores
				}
			}
		}
	}
	if remoteOS == osinfo.OSLinux {
		parsed.MemReservedGB = nil
	}
	session.SystemPartitions.Load(session.Client)
	parsed.Partitions = session.SystemPartitions.Partitions
	if !session.SystemPartitions.UpdatedAt.IsZero() {
		parsed.PartitionsUpdatedAt = session.SystemPartitions.UpdatedAt.Unix()
	}
	return parsed, nil
}

func parseSystemStats(output string, diskUnit float64) (*SystemStats, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	widths := [...]int{1, 2, 2, 1, 2, 2, 2, 2, 1}
	if len(lines) < len(widths) {
		return nil, fmt.Errorf("unexpected system stats output")
	}
	var values [9][2]float64
	for i, width := range widths {
		fields := strings.Fields(lines[i])
		if len(fields) != width {
			return nil, fmt.Errorf("invalid system stats line %d", i+1)
		}
		for j, field := range fields {
			value, err := strconv.ParseFloat(field, 64)
			if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
				return nil, fmt.Errorf("invalid system stats value on line %d", i+1)
			}
			values[i][j] = value
		}
	}
	cpu := values[0][0]
	memUsed, memTotal := values[1][0], values[1][1]
	diskUsed, diskTotal := values[2][0], values[2][1]
	elapsed := values[8][0]
	if cpu > 100 || memTotal <= 0 || memUsed > memTotal || diskTotal <= 0 || diskUsed > diskTotal || elapsed <= 0 {
		return nil, fmt.Errorf("system stats values out of range")
	}
	const gb = 1073741824.0
	stats := &SystemStats{
		CPUPercent:   cpu,
		MemUsedGB:    memUsed / gb,
		MemFreeGB:    (memTotal - memUsed) / gb,
		MemTotalGB:   memTotal / gb,
		DiskUsedGB:   diskUsed / gb,
		DiskFreeGB:   (diskTotal - diskUsed) / gb,
		DiskTotalGB:  diskTotal / gb,
		UptimeSecs:   values[3][0],
		NetRxBps:     math.Max(0, values[6][0]-values[4][0]) / elapsed,
		NetTxBps:     math.Max(0, values[6][1]-values[4][1]) / elapsed,
		DiskReadBps:  math.Max(0, values[7][0]-values[5][0]) * diskUnit / elapsed,
		DiskWriteBps: math.Max(0, values[7][1]-values[5][1]) * diskUnit / elapsed,
	}
	if err := parseSystemDetails(stats, lines[len(widths):]); err != nil {
		return nil, err
	}
	return stats, nil
}

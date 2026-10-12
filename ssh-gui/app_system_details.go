package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"ssh-gui/backend/systeminfo"
)

// Detail records follow the nine summary lines: MEM and zero or more CORE rows.
func parseSystemDetails(stats *SystemStats, lines []string) error {
	stats.CPUCores = make([]systeminfo.CPUCore, 0)
	seen := make(map[string]bool)
	memoryFound := false
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "MEM":
			if len(fields) != 4 || memoryFound {
				return fmt.Errorf("invalid memory details")
			}
			var values [3]float64
			for i, field := range fields[1:] {
				value, err := strconv.ParseFloat(field, 64)
				if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value/1073741824.0 > stats.MemTotalGB {
					return fmt.Errorf("invalid memory detail value")
				}
				values[i] = value / 1073741824.0
			}
			stats.MemReservedGB = &values[0]
			stats.MemBuffersGB, stats.MemAvailableGB = values[1], values[2]
			memoryFound = true
		case "CORE":
			if len(fields) != 3 || seen[fields[1]] {
				return fmt.Errorf("invalid CPU core details")
			}
			if _, err := strconv.ParseUint(fields[1], 10, 32); err != nil {
				return fmt.Errorf("invalid CPU core ID")
			}
			value, err := strconv.ParseFloat(fields[2], 64)
			if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 100 {
				return fmt.Errorf("invalid CPU core usage")
			}
			stats.CPUCores = append(stats.CPUCores, systeminfo.CPUCore{ID: fields[1], Percent: value})
			seen[fields[1]] = true
		default:
			return fmt.Errorf("unknown system detail record")
		}
	}
	if !memoryFound {
		return fmt.Errorf("missing memory details")
	}
	return nil
}

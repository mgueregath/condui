package models

type DockerStats struct {
	ID       string `json:"id"`
	CPUPerc  string `json:"cpuPerc"`  // e.g. "0.15%"
	MemUsage string `json:"memUsage"` // e.g. "123MiB / 8GiB"
	MemPerc  string `json:"memPerc"`  // e.g. "1.50%"
}

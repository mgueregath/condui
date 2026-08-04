package models

type VMInfo struct {
	Name string `json:"name"`
	UUID string `json:"uuid"`
	// State: "running", "paused", "saved", "stopped", "aborted", "error"
	State string `json:"state"`
	// Hardware specs (from showvminfo)
	MemoryMB int    `json:"memoryMb"`
	CPUs     int    `json:"cpus"`
	OS       string `json:"os"`
	// Network — IP from Guest Additions (empty if not installed / VM off)
	IP        string `json:"ip"`
	Autostart bool   `json:"autostart"`
}

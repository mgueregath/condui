package models

type PortInfo struct {
	Port    int    `json:"port"`
	Address string `json:"address"`
	Process string `json:"process"`
	Proto   string `json:"proto"`
}

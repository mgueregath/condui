package models

type DatabaseInfo struct {
	Name      string `json:"name"`
	Port      int    `json:"port"`
	Address   string `json:"address"`
	Source    string `json:"source"`    // "system" | "docker"
	Container string `json:"container"` // nombre del contenedor si es docker
	Image     string `json:"image"`     // imagen docker si aplica
}

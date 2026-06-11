package dbexplorer

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Cred holds saved credentials for a given db type/port combination.
type Cred struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

func credPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".condui", "db-creds.json")
}

// LoadCreds loads all saved DB credentials from disk.
func LoadCreds() map[string]Cred {
	data, err := os.ReadFile(credPath())
	if err != nil {
		return map[string]Cred{}
	}
	var m map[string]Cred
	json.Unmarshal(data, &m)
	if m == nil {
		m = map[string]Cred{}
	}
	return m
}

// SaveCreds persists all DB credentials to disk.
func SaveCreds(m map[string]Cred) {
	p := credPath()
	os.MkdirAll(filepath.Dir(p), 0700)
	data, _ := json.Marshal(m)
	os.WriteFile(p, data, 0600)
}

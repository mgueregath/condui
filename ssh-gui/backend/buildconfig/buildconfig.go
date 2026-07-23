// Package buildconfig holds values fixed at compile time via go:embed. The
// embedded bytes are whatever build.config.yaml contained when `go build`
// ran — editing that file on disk after the binary exists has no effect,
// since it's never read from the filesystem again at runtime.
package buildconfig

import (
	_ "embed"

	"gopkg.in/yaml.v3"
)

//go:embed build.config.yaml
var raw []byte

type Config struct {
	ServerURL        string `yaml:"server_url"`
	DbManagerEnabled bool   `yaml:"db_manager_enabled"`
}

// Values is the parsed build.config.yaml, fixed at compile time.
var Values Config

func init() {
	if err := yaml.Unmarshal(raw, &Values); err != nil {
		panic("buildconfig: invalid build.config.yaml embedded at build time: " + err.Error())
	}
}

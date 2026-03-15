package environment

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type LocalEnvironmentConfig struct {
	Cwd     string            `json:"cwd" yaml:"cwd"`
	Env     map[string]string `json:"env" yaml:"env"`
	Timeout int               `json:"timeout" yaml:"timeout"`
}

func BuildEnvironmentConfigFromRawMap(raw map[string]any) (LocalEnvironmentConfig, error) {
	data, err := yaml.Marshal(raw)
	if err != nil {
		return LocalEnvironmentConfig{}, fmt.Errorf("marshaling env config: %w", err)
	}
	var cfg LocalEnvironmentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return LocalEnvironmentConfig{}, fmt.Errorf("unmarshaling env config: %w", err)
	}
	return cfg, nil
}

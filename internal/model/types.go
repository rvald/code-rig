package model

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type ModelConfig struct {
	ModelName           string         `json:"model_name" yaml:"model_name"`
	ModelKwargs         map[string]any `json:"model_kwargs" yaml:"model_kwargs"`
	FormatErrorTemplate string         `json:"format_error_template" yaml:"format_error_template"`
	ObservationTemplate string         `json:"observation_template" yaml:"observation_template"`
}

func BuildModelConfigFromRawMap(raw map[string]any) (ModelConfig, error) {
	data, err := yaml.Marshal(raw)
	if err != nil {
		return ModelConfig{}, fmt.Errorf("marshaling model config: %w", err)
	}
	var cfg ModelConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return ModelConfig{}, fmt.Errorf("unmarshaling model config: %w", err)
	}
	return cfg, nil
}

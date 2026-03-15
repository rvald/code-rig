package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/rvald/code-rig/internal/utils"
	"gopkg.in/yaml.v3"
)

// RawConfig holds the three top-level sections of a parsed YAML config.
// Each section is a map[string]any — the caller is responsible for mapping
// these to typed structs (AgentConfig, LocalEnvironmentConfig, etc.).
type RawConfig struct {
	Agent       map[string]any `yaml:"agent"`
	Model       map[string]any `yaml:"model"`
	Environment map[string]any `yaml:"environment"`
}

func LoadConfigFile(path string) (RawConfig, error) {
	resolved, err := resolveConfigPath(path)
	if err != nil {
		return RawConfig{}, err
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return RawConfig{}, fmt.Errorf("reading config file: %w", err)
	}
	return ParseConfigBytes(data)
}

func ParseConfigBytes(data []byte) (RawConfig, error) {
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return RawConfig{}, fmt.Errorf("parsing YAML: %w", err)
	}
	cfg := RawConfig{
		Agent:       toStringAnyMap(raw["agent"]),
		Model:       toStringAnyMap(raw["model"]),
		Environment: toStringAnyMap(raw["environment"]),
	}
	return cfg, nil
}

func toStringAnyMap(v any) map[string]any {
	if v == nil {
		return make(map[string]any)
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return make(map[string]any)
}

func KeyValueToNestedMap(spec string) (map[string]any, error) {
	parts := strings.SplitN(spec, "=", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid key=value spec: %q", spec)
	}
	key, value := parts[0], parts[1]

	// Try to parse value as JSON (for numbers, booleans, etc.)
	var parsed any
	if err := json.Unmarshal([]byte(value), &parsed); err != nil {
		parsed = value // Keep as string if not valid JSON
	}

	keys := strings.Split(key, ".")
	result := make(map[string]any)
	current := result
	for _, k := range keys[:len(keys)-1] {
		next := make(map[string]any)
		current[k] = next
		current = next
	}
	current[keys[len(keys)-1]] = parsed
	return result, nil
}

func MergeConfigs(configs ...RawConfig) RawConfig {
	result := RawConfig{
		Agent:       make(map[string]any),
		Model:       make(map[string]any),
		Environment: make(map[string]any),
	}
	for _, cfg := range configs {
		result.Agent = utils.RecursiveMerge(result.Agent, cfg.Agent)
		result.Model = utils.RecursiveMerge(result.Model, cfg.Model)
		result.Environment = utils.RecursiveMerge(result.Environment, cfg.Environment)
	}
	return result
}

func ParseRawMap(m map[string]any) (RawConfig, error) {
	return RawConfig{
		Agent:       toStringAnyMap(m["agent"]),
		Model:       toStringAnyMap(m["model"]),
		Environment: toStringAnyMap(m["environment"]),
	}, nil
}

func resolveConfigPath(spec string) (string, error) {
	candidates := []string{spec}
	if !strings.HasSuffix(spec, ".yaml") && !strings.HasSuffix(spec, ".yml") {
		candidates = append(candidates, spec+".yaml", spec+".yml")
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("config file not found: %q (tried: %v)", spec, candidates)
}

func GetConfigFromSpec(spec string) (RawConfig, error) {
	if strings.Contains(spec, "=") {
		m, err := KeyValueToNestedMap(spec)
		if err != nil {
			return RawConfig{}, err
		}
		return ParseRawMap(m)
	}
	return LoadConfigFile(spec)
}

func LoadAndMerge(specs []string) (RawConfig, error) {
	var configs []RawConfig
	for _, spec := range specs {
		cfg, err := GetConfigFromSpec(spec)
		if err != nil {
			return RawConfig{}, fmt.Errorf("loading spec %q: %w", spec, err)
		}
		configs = append(configs, cfg)
	}
	return MergeConfigs(configs...), nil
}




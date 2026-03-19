package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rvald/code-rig/internal/config"
)

// ParseKeyValueSpec converts "a.b.c=val" into map[string]any{"a": {"b": {"c": val}}}
func ParseKeyValueSpec(spec string) (map[string]any, error) {
	parts := strings.SplitN(spec, "=", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid spec format, expected key=value: %s", spec)
	}

	keyPath := parts[0]
	rawVal := parts[1]

	// Attempt to parse value as JSON (handles bools, ints, floats)
	var parsedVal any
	if err := json.Unmarshal([]byte(rawVal), &parsedVal); err != nil {
		// Fallback to raw string if it's not valid JSON (like "yolo")
		parsedVal = rawVal
	}

	keys := strings.Split(keyPath, ".")
	result := make(map[string]any)
	current := result

	for i := 0; i < len(keys)-1; i++ {
		k := keys[i]
		next := make(map[string]any)
		current[k] = next
		current = next
	}

	current[keys[len(keys)-1]] = parsedVal
	return result, nil
}

func BuildFinalConfigFromSpecs(specs []string) (config.RawConfig, error) {
	var configs []config.RawConfig

	for _, spec := range specs {
		if strings.Contains(spec, "=") {
			override, err := ParseKeyValueSpec(spec)
			if err != nil {
				return config.RawConfig{}, err
			}
			raw, _ := config.ParseRawMap(override)
			configs = append(configs, raw)
		} else {
			fileConfig, err := config.LoadConfigFile(spec)
			if err != nil {
				return config.RawConfig{}, err
			}
			configs = append(configs, fileConfig)
		}
	}

	return config.MergeConfigs(configs...), nil
}

package config

import "testing"

// Phase 1: RawConfig struct

func TestRawConfigSections(t *testing.T) {
	cfg := RawConfig{
		Agent:       map[string]any{"system_template": "hello"},
		Model:       map[string]any{"model_name": "gpt-4"},
		Environment: map[string]any{"timeout": 30},
	}
	if cfg.Agent["system_template"] != "hello" {
		t.Errorf("Agent system_template = %v, want 'hello'", cfg.Agent["system_template"])
	}
	if cfg.Model["model_name"] != "gpt-4" {
		t.Errorf("Model model_name = %v, want 'gpt-4'", cfg.Model["model_name"])
	}
	if cfg.Environment["timeout"] != 30 {
		t.Errorf("Environment timeout = %v, want 30", cfg.Environment["timeout"])
	}
}

func TestLoadConfigFromYAML(t *testing.T) {
	cfg, err := LoadConfigFile("testdata/minimal.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Agent["system_template"] != "You are a test agent." {
		t.Errorf("system_template = %v, want 'You are a test agent.'", cfg.Agent["system_template"])
	}
	if cfg.Agent["instance_template"] != "Do: {{.Task}}" {
		t.Errorf("instance_template = %v, want 'Do: {{.Task}}'", cfg.Agent["instance_template"])
	}
	if cfg.Environment["timeout"] != 10 {
		t.Errorf("timeout = %v, want 10", cfg.Environment["timeout"])
	}
}

func TestLoadConfigFileMissing(t *testing.T) {
	_, err := LoadConfigFile("testdata/nonexistent.yaml")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestLoadConfigFileInvalidYAML(t *testing.T) {
	_, err := LoadConfigFile("testdata/invalid.yaml")
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestLoadConfigPartialSections(t *testing.T) {
	cfg, err := ParseConfigBytes([]byte("agent:\n  step_limit: 5\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Agent["step_limit"] != 5 {
		t.Errorf("step_limit = %v, want 5", cfg.Agent["step_limit"])
	}
	// Missing sections should be empty maps, not nil
	if cfg.Model == nil {
		t.Error("Model should not be nil for missing section")
	}
	if cfg.Environment == nil {
		t.Error("Environment should not be nil for missing section")
	}
}

func TestKeyValueSimple(t *testing.T) {
	result, err := KeyValueToNestedMap("model.model_name=gpt-4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	model, ok := result["model"].(map[string]any)
	if !ok {
		t.Fatal("model should be a map")
	}
	if model["model_name"] != "gpt-4" {
		t.Errorf("model_name = %v, want 'gpt-4'", model["model_name"])
	}
}

func TestKeyValueDeeplyNested(t *testing.T) {
	result, err := KeyValueToNestedMap("model.model_kwargs.temperature=0.5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	model := result["model"].(map[string]any)
	kwargs := model["model_kwargs"].(map[string]any)
	if kwargs["temperature"] != 0.5 {
		t.Errorf("temperature = %v, want 0.5", kwargs["temperature"])
	}
}

func TestKeyValueJSONParsing(t *testing.T) {
	tests := []struct {
		spec string
		key  string
		want any
	}{
		{"a.val=0.5", "val", 0.5},
		{"a.val=true", "val", true},
		{"a.val=42", "val", float64(42)}, // JSON numbers are float64
		{"a.val=hello", "val", "hello"},  // Not valid JSON, stays string
	}
	for _, tt := range tests {
		result, err := KeyValueToNestedMap(tt.spec)
		if err != nil {
			t.Fatalf("spec %q: unexpected error: %v", tt.spec, err)
		}
		inner := result["a"].(map[string]any)
		if inner[tt.key] != tt.want {
			t.Errorf("spec %q: %v = %v (%T), want %v (%T)",
				tt.spec, tt.key, inner[tt.key], inner[tt.key], tt.want, tt.want)
		}
	}
}

func TestKeyValueNoEquals(t *testing.T) {
	_, err := KeyValueToNestedMap("not_a_key_value")
	if err == nil {
		t.Error("expected error for spec without '=', got nil")
	}
}

func TestMergeConfigs(t *testing.T) {
	base, err := LoadConfigFile("testdata/minimal.yaml")
	if err != nil {
		t.Fatalf("loading base: %v", err)
	}
	override, err := LoadConfigFile("testdata/override.yaml")
	if err != nil {
		t.Fatalf("loading override: %v", err)
	}

	merged := MergeConfigs(base, override)

	// Base values preserved
	if merged.Agent["system_template"] != "You are a test agent." {
		t.Errorf("system_template should come from base, got %v", merged.Agent["system_template"])
	}
	// Override values applied
	if merged.Agent["step_limit"] != 5 {
		t.Errorf("step_limit = %v, want 5 (from override)", merged.Agent["step_limit"])
	}
	if merged.Agent["cost_limit"] != 2.0 {
		t.Errorf("cost_limit = %v, want 2.0 (from override)", merged.Agent["cost_limit"])
	}
	// New section from override
	if merged.Model["model_name"] != "claude-3" {
		t.Errorf("model_name = %v, want 'claude-3'", merged.Model["model_name"])
	}
	// Untouched section from base
	if merged.Environment["timeout"] != 10 {
		t.Errorf("timeout = %v, want 10 (from base)", merged.Environment["timeout"])
	}
}

func TestMergeKeyValueWithFileConfig(t *testing.T) {
	base, err := LoadConfigFile("testdata/minimal.yaml")
	if err != nil {
		t.Fatalf("loading base: %v", err)
	}
	kvMap, err := KeyValueToNestedMap("agent.step_limit=10")
	if err != nil {
		t.Fatalf("parsing key-value: %v", err)
	}
	kvConfig, err := ParseRawMap(kvMap)
	if err != nil {
		t.Fatalf("converting to RawConfig: %v", err)
	}

	merged := MergeConfigs(base, kvConfig)
	if merged.Agent["step_limit"] != float64(10) {
		t.Errorf("step_limit = %v, want 10", merged.Agent["step_limit"])
	}
	// Other values preserved
	if merged.Agent["system_template"] != "You are a test agent." {
		t.Errorf("system_template should be preserved from base")
	}
}

func TestGetConfigFromSpecFile(t *testing.T) {
	cfg, err := GetConfigFromSpec("testdata/minimal.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Agent["system_template"] != "You are a test agent." {
		t.Errorf("system_template = %v, want 'You are a test agent.'", cfg.Agent["system_template"])
	}
}

func TestGetConfigFromSpecKeyValue(t *testing.T) {
	cfg, err := GetConfigFromSpec("model.model_name=gpt-4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Model["model_name"] != "gpt-4" {
		t.Errorf("model_name = %v, want 'gpt-4'", cfg.Model["model_name"])
	}
}

func TestGetConfigFromSpecAutoExtension(t *testing.T) {
	// Copy minimal.yaml so we can test without extension
	// This test assumes "testdata/minimal" resolves to "testdata/minimal.yaml"
	cfg, err := GetConfigFromSpec("testdata/minimal")
	if err != nil {
		t.Fatalf("should resolve 'minimal' to 'minimal.yaml': %v", err)
	}
	if cfg.Agent["system_template"] != "You are a test agent." {
		t.Errorf("system_template = %v, want 'You are a test agent.'", cfg.Agent["system_template"])
	}
}

func TestLoadAndMerge(t *testing.T) {
	specs := []string{
		"testdata/minimal.yaml",
		"testdata/override.yaml",
		"agent.cost_limit=5.0",
	}
	cfg, err := LoadAndMerge(specs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// From minimal.yaml
	if cfg.Agent["system_template"] != "You are a test agent." {
		t.Errorf("system_template from base, got %v", cfg.Agent["system_template"])
	}
	// From override.yaml
	if cfg.Agent["step_limit"] != 5 {
		t.Errorf("step_limit from override, got %v (%T)", cfg.Agent["step_limit"], cfg.Agent["step_limit"])
	}
	// From key-value spec (overrides override.yaml's cost_limit)
	if cfg.Agent["cost_limit"] != 5.0 {
		t.Errorf("cost_limit from kv spec, got %v", cfg.Agent["cost_limit"])
	}
	// Model from override.yaml
	if cfg.Model["model_name"] != "claude-3" {
		t.Errorf("model_name from override, got %v", cfg.Model["model_name"])
	}
}

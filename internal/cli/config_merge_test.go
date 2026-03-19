package cli

import (
	"os"
	"reflect"
	"testing"
)

func TestParseKeyValueSpec(t *testing.T) {
	tests := []struct {
		input string
		want  map[string]any
	}{
		{
			input: "agent.cost_limit=5",
			want:  map[string]any{"agent": map[string]any{"cost_limit": 5.0}},
		},
		{
			input: "model.model_kwargs.temperature=0.5",
			want:  map[string]any{"model": map[string]any{"model_kwargs": map[string]any{"temperature": 0.5}}},
		},
		{
			input: "agent.mode=yolo",
			want:  map[string]any{"agent": map[string]any{"mode": "yolo"}},
		},
	}

	for _, tc := range tests {
		got, err := ParseKeyValueSpec(tc.input)
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", tc.input, err)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("for %q, got %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestBuildFinalConfigFromSpecs(t *testing.T) {
	// The CLI guide calls for LoadYAMLFile directly returning map[string]any.
	// We'll mock the config.LoadConfigFile directly using strings for speed,
	// but because we rely on the filesystem, we skip a full integration test here
	// and trust config.MergeConfigs from Phase 12 passes integration testing.
	
	// Create temporary base file
	specStr := `
agent:
  mode: confirm
  step_limit: 10
`
	tmpfile := t.TempDir() + "/base.yaml"
	if err := os.WriteFile(tmpfile, []byte(specStr), 0644); err != nil {
		t.Fatal(err)
	}

	specs := []string{
		tmpfile,
		"agent.step_limit=50",
	}

	merged, err := BuildFinalConfigFromSpecs(specs)
	if err != nil {
		t.Fatal(err)
	}

	if merged.Agent["mode"] != "confirm" {
		t.Errorf("expected mode=confirm from base.yaml, got %v", merged.Agent["mode"])
	}
	if merged.Agent["step_limit"] != float64(50) {
		t.Errorf("expected step_limit=50 from override, got %v", merged.Agent["step_limit"])
	}
}

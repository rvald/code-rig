package utils

import "testing"

func TestRenderTemplate(t *testing.T) {
	result, err := RenderTemplate("Hello {{.Name}}", map[string]any{"Name": "World"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Hello World" {
		t.Errorf("result = %q, want %q", result, "Hello World")
	}
}

func TestRenderTemplateMissingVar(t *testing.T) {
	_, err := RenderTemplate("Hello {{.Missing}}", map[string]any{"Name": "World"})
	if err == nil {
		t.Error("expected error for missing template variable, got nil")
	}
}

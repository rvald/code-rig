package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsConfigured(t *testing.T) {
	os.Unsetenv("MSWEA_CONFIGURED")
	if IsConfigured() {
		t.Error("expected false when env var missing")
	}

	os.Setenv("MSWEA_CONFIGURED", "true")
	if !IsConfigured() {
		t.Error("expected true when env var set")
	}
	os.Unsetenv("MSWEA_CONFIGURED")
}

func TestRunSetupWizard(t *testing.T) {
	inBuf := strings.NewReader("openai/gpt-4\nOPENAI_API_KEY\nsk-test123\n")
	var outBuf bytes.Buffer

	// Use a temporary file for .env target
	tmpEnv := filepath.Join(t.TempDir(), ".env")

	err := RunSetupWizard(inBuf, &outBuf, tmpEnv)
	if err != nil {
		t.Fatal(err)
	}

	envContent, _ := os.ReadFile(tmpEnv)
	content := string(envContent)

	if !strings.Contains(content, `MSWEA_MODEL_NAME="openai/gpt-4"`) {
		t.Errorf("missing model name in .env")
	}
	if !strings.Contains(content, `OPENAI_API_KEY="sk-test123"`) {
		t.Errorf("missing api key in .env")
	}
	if !strings.Contains(content, `MSWEA_CONFIGURED="true"`) {
		t.Errorf("missing configured flag")
	}
}

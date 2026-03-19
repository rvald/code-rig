package cli

import (
	"testing"
)

func TestAppParseFlags(t *testing.T) {
	app := NewApp()

	args := []string{
		"--task", "Fix the db connection",
		"--model", "gpt-4",
		"--cost-limit", "10",
		"--yolo",
		"-c", "config.yaml",
		"-c", "agent.step_limit=5",
	}

	if err := app.ParseArgs(args); err != nil {
		t.Fatal(err)
	}

	if app.Task != "Fix the db connection" {
		t.Errorf("Task = %q", app.Task)
	}
	if !app.Yolo {
		t.Errorf("Expected Yolo to be true")
	}
	if app.CostLimit != 10 {
		t.Errorf("Expected CostLimit=10")
	}
	if len(app.ConfigSpecs) != 2 {
		t.Errorf("Expected 2 config specs")
	}
}

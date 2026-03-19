package cli

import (
	"bufio"
	"flag"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/rvald/code-rig/internal/utils"
)

type stringSlice []string

func (i *stringSlice) String() string { return fmt.Sprint(*i) }
func (i *stringSlice) Set(value string) error {
	*i = append(*i, value)
	return nil
}

type App struct {
	Task             string
	Model            string
	ModelClass       string
	AgentClass       string
	EnvironmentClass string
	Yolo             bool
	CostLimit        float64
	ConfigSpecs      stringSlice
	OutputPath       string
	ExitImmediately  bool
}

func NewApp() *App {
	return &App{}
}

func (a *App) ParseArgs(args []string) error {
	fs := flag.NewFlagSet("mini", flag.ContinueOnError)

	fs.StringVar(&a.Task, "task", "", "Task/problem statement (or -t)")
	fs.StringVar(&a.Task, "t", "", "Task (shorthand)")

	fs.StringVar(&a.Model, "model", "", "Model to use")
	fs.StringVar(&a.Model, "m", "", "Model (shorthand)")

	fs.BoolVar(&a.Yolo, "yolo", false, "Run without confirmation")
	fs.BoolVar(&a.Yolo, "y", false, "Yolo (shorthand)")

	fs.Float64Var(&a.CostLimit, "cost-limit", -1, "Cost limit")
	fs.Float64Var(&a.CostLimit, "l", -1, "Cost limit (shorthand)")

	fs.StringVar(&a.ModelClass, "model-class", "", "Model class")
	fs.StringVar(&a.AgentClass, "agent-class", "", "Agent class")
	fs.StringVar(&a.EnvironmentClass, "environment-class", "", "Env class")

	fs.Var(&a.ConfigSpecs, "config", "Config specs")
	fs.Var(&a.ConfigSpecs, "c", "Config specs (shorthand)")

	fs.StringVar(&a.OutputPath, "output", "last_mini_run.traj.json", "Output trajectory")
	fs.StringVar(&a.OutputPath, "o", "last_mini_run.traj.json", "Output (shorthand)")

	return fs.Parse(args)
}

func (a *App) BuildOverrideMap() map[string]any {
	// Mimics the `configs.append({...})` block in mini.py line 73
	m := map[string]any{
		"run":         make(map[string]any),
		"agent":       make(map[string]any),
		"model":       make(map[string]any),
		"environment": make(map[string]any),
	}

	if a.Task != "" {
		m["run"].(map[string]any)["task"] = a.Task
	}

	if a.Yolo {
		m["agent"].(map[string]any)["mode"] = "yolo"
	}
	if a.CostLimit >= 0 {
		m["agent"].(map[string]any)["cost_limit"] = a.CostLimit
	}
	if a.ExitImmediately {
		m["agent"].(map[string]any)["confirm_exit"] = false
	}
	if a.OutputPath != "" {
		m["agent"].(map[string]any)["output_path"] = a.OutputPath
	}

	if a.Model != "" {
		m["model"].(map[string]any)["model_name"] = a.Model
	}
	if a.ModelClass != "" {
		m["model"].(map[string]any)["model_class"] = a.ModelClass
	}
	if a.EnvironmentClass != "" {
		m["environment"].(map[string]any)["environment_class"] = a.EnvironmentClass
	}
	if a.AgentClass != "" {
		m["agent"].(map[string]any)["agent_class"] = a.AgentClass
	}

	return m
}

func (a *App) Execute(args []string) error {
	_ = godotenv.Load(".env") // Load env vars from .env if it exists

	if !IsConfigured() {
		RunSetupWizard(os.Stdin, os.Stdout, ".env")
		_ = godotenv.Load(".env") // Reload after setup wizard writes to it
	}

	// 1. Parse Args
	if err := a.ParseArgs(args); err != nil {
		return err
	}

	// 2. Default Config Spec behavior
	if len(a.ConfigSpecs) == 0 {
		a.ConfigSpecs = []string{"mini.yaml"} // fallback equivalent
	}

	// 3. Build & merge maps
	fileRawConfig, err := BuildFinalConfigFromSpecs(a.ConfigSpecs)
	var fileConfigMap map[string]any
	if err != nil {
		// Log but continue, we might not have a mini.yaml and that's fine if we have -c overrides
		fileConfigMap = make(map[string]any)
	} else {
		// Convert the merged RawConfig back to map[string]any to merge with our override map
		fileConfigMap = map[string]any{
			"agent":       fileRawConfig.Agent,
			"model":       fileRawConfig.Model,
			"environment": fileRawConfig.Environment,
		}
	}

	finalMap := utils.RecursiveMerge(GetDefaultConfig(), fileConfigMap, a.BuildOverrideMap())

	// 4. Prompt for task if missing
	runSection, _ := finalMap["run"].(map[string]any)
	task, _ := runSection["task"].(string)
	if task == "" {
		fmt.Println("What do you want to do?")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			task = scanner.Text()
		}
	}

	// 5. Build dependencies using Factory Switchers
	envClass, _ := finalMap["environment"].(map[string]any)["environment_class"].(string)
	env, err := GetEnvironment(envClass, finalMap["environment"].(map[string]any))
	if err != nil {
		return fmt.Errorf("environment factory: %w", err)
	}

	modelClass, _ := finalMap["model"].(map[string]any)["model_class"].(string)
	mod, err := GetModel(modelClass, finalMap["model"].(map[string]any))
	if err != nil {
		return fmt.Errorf("model factory: %w", err)
	}

	agentClass, _ := finalMap["agent"].(map[string]any)["agent_class"].(string)
	ag, err := GetAgent(agentClass, mod, env, finalMap["agent"].(map[string]any))
	if err != nil {
		return fmt.Errorf("agent factory: %w", err)
	}

	// 6. Run
	_, err = ag.Run(task)

	// 7. Save trajectory
	outPath, _ := finalMap["agent"].(map[string]any)["output_path"].(string)
	if outPath == "" {
		outPath = "last_mini_run.traj.json" // fallback
	}
	ag.Save(outPath)
	fmt.Printf("Saved trajectory to '%s'\n", outPath)

	if cl, ok := env.(interface{ Cleanup() }); ok {
		cl.Cleanup()
	}

	return err
}



package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rvald/code-rig/internal/agent"
	"github.com/rvald/code-rig/internal/config"
	"github.com/rvald/code-rig/internal/environment"
	"github.com/rvald/code-rig/internal/model"
	"github.com/spf13/cobra"
)

type App struct {
	Task        string
	Model       string
	CostLimit   float64
	ConfigFiles []string

	ModelFactory func(model.ModelConfig) agent.Model
	EnvFactory   func(environment.LocalEnvironmentConfig) agent.Environment
}

func DefaultModelFactory(cfg model.ModelConfig) agent.Model {
	return model.NewOpenAIModel(cfg, os.Getenv("OPENAI_API_KEY"))
}

func DefaultEnvFactory(cfg environment.LocalEnvironmentConfig) agent.Environment {
	return environment.NewLocalEnvironment(cfg)
}

func NewApp() *App {
	return &App{
		ModelFactory: DefaultModelFactory,
		EnvFactory:   DefaultEnvFactory,
	}
}

func (a *App) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "code-rig",
		Short: "Code-Rig: Minimal SWE Agent",
		Run: func(cmd *cobra.Command, args []string) {
			// No-op for now, actual run logic added in Phase 4
		},
	}

	cmd.Flags().StringVar(&a.Task, "task", "", "Task/problem statement")
	cmd.Flags().StringVar(&a.Model, "model", "", "Model to use")
	cmd.Flags().Float64Var(&a.CostLimit, "cost-limit", -1, "Cost limit. Set to 0 to disable.")
	cmd.Flags().StringSliceVar(&a.ConfigFiles, "config", nil, "Path to config files or key-value pairs")

	// Ensure pflag allows parsing even without the subcommand context running
	// Normally cobra parses automatically during Execute()
	return cmd
}

func (a *App) GetCLIOverrideConfig() config.RawConfig {
	cfg := config.RawConfig{
		Agent: make(map[string]any),
		Model: make(map[string]any),
	}

	if a.CostLimit >= 0 {
		cfg.Agent["cost_limit"] = a.CostLimit
	}
	if a.Model != "" {
		cfg.Model["model_name"] = a.Model
	}
	return cfg
}

func (a *App) BuildFinalConfig() (config.RawConfig, error) {
	fileCfg, err := config.LoadAndMerge(a.ConfigFiles)
	if err != nil {
		return config.RawConfig{}, err
	}

	overrideCfg := a.GetCLIOverrideConfig()

	return config.MergeConfigs(fileCfg, overrideCfg), nil
}

func (a *App) AssembleAgent(raw config.RawConfig) (*agent.DefaultAgent, error) {
	agentCfg, err := agent.BuildAgentConfigFromRawMap(raw.Agent)
	if err != nil {
		return nil, fmt.Errorf("building agent config: %w", err)
	}
	if err := agent.ValidateAgentConfig(agentCfg); err != nil {
		return nil, err
	}

	modelCfg, err := model.BuildModelConfigFromRawMap(raw.Model)
	if err != nil {
		return nil, fmt.Errorf("building model config: %w", err)
	}

	envCfg, err := environment.BuildEnvironmentConfigFromRawMap(raw.Environment)
	if err != nil {
		return nil, fmt.Errorf("building env config: %w", err)
	}

	m := a.ModelFactory(modelCfg)
	e := a.EnvFactory(envCfg)

	ag := agent.NewDefaultAgent(agentCfg, m, e)
	return ag, nil
}

func (a *App) GetTask(r io.Reader) (string, error) {
	if a.Task != "" {
		return a.Task, nil
	}

	fmt.Println("What do you want to do? (Press Ctrl+D to submit)")
	bytes, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(bytes)), nil
}

func (a *App) Run(r io.Reader) error {
	task, err := a.GetTask(r)
	if err != nil {
		return fmt.Errorf("getting task: %w", err)
	}
	if task == "" {
		return fmt.Errorf("task cannot be empty")
	}

	rawCfg, err := a.BuildFinalConfig()
	if err != nil {
		return fmt.Errorf("building config: %w", err)
	}

	ag, err := a.AssembleAgent(rawCfg)
	if err != nil {
		return fmt.Errorf("assembling agent: %w", err)
	}

	_, err = ag.Run(task)
	return err
}

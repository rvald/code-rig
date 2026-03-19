package cli

import (
	"fmt"
	"os"

	"github.com/rvald/code-rig/internal/agent"
	"github.com/rvald/code-rig/internal/environment"
	"github.com/rvald/code-rig/internal/model"
)

func GetEnvironment(envClass string, cfg map[string]any) (agent.Environment, error) {
	if envClass == "" || envClass == "local" {
		envCfg, err := environment.BuildEnvironmentConfigFromRawMap(cfg)
		if err != nil {
			return nil, fmt.Errorf("bad local env config: %w", err)
		}
		return environment.NewLocalEnvironment(envCfg), nil
	}
	if envClass == "docker" {
		dockerCfg, err := environment.BuildDockerEnvironmentConfigFromRawMap(cfg)
		if err != nil {
			return nil, fmt.Errorf("bad docker env config: %w", err)
		}
		env, err := environment.NewDockerEnvironment(dockerCfg)
		return env, err
	}
	return nil, fmt.Errorf("unknown environment class: %s", envClass)
}

func GetModel(modelClass string, cfg map[string]any) (agent.Model, error) {
	if modelClass == "" || modelClass == "litellm" {
		modelCfg, err := model.BuildModelConfigFromRawMap(cfg)
		if err != nil {
			return nil, fmt.Errorf("bad model config: %w", err)
		}
		// Fallback to MSWEA_MODEL_NAME env var if no model_name in config
		if modelCfg.ModelName == "" {
			if envModel := os.Getenv("MSWEA_MODEL_NAME"); envModel != "" {
				modelCfg.ModelName = envModel
			}
		}
		if modelCfg.ModelName == "" {
			return nil, fmt.Errorf("no model name provided: use --model flag, set model_name in config, or set MSWEA_MODEL_NAME env var")
		}
		return model.NewOpenAIModel(modelCfg, os.Getenv("OPENAI_API_KEY")), nil
	}
	return nil, fmt.Errorf("unknown model class: %s", modelClass)
}

func GetAgent(agentClass string, m agent.Model, e agent.Environment, cfg map[string]any) (agent.Agent, error) {
	if agentClass == "" || agentClass == "interactive" || agentClass == "default" {
		agentCfg, err := agent.BuildInteractiveAgentConfigFromRawMap(cfg)
		if err != nil {
			return nil, fmt.Errorf("bad agent config: %w", err)
		}
		// Based on tests, InteractiveAgent embeds DefaultAgent, so if we instantiate InteractiveAgent
		// but mode="yolo", it behaves identically to DefaultAgent but just passes the interface.
		return agent.NewInteractiveAgent(agentCfg, m, e), nil
	}
	return nil, fmt.Errorf("unknown agent class: %s", agentClass)
}

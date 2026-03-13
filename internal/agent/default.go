package agent

import (
	"text/template"
	"fmt"
	"bytes"
	"os"
	"errors"
	"path/filepath"
	"encoding/json"
)

func renderTemplate(tmpl string, vars map[string]any) (string, error) {
	t, err := template.New("").Option("missingkey=error").Parse(tmpl)
    if err != nil {
        return "", fmt.Errorf("parsing template: %w", err)
    }
    var buf bytes.Buffer
    if err := t.Execute(&buf, vars); err != nil {
        return "", fmt.Errorf("executing template: %w", err)
    }
    return buf.String(), nil
}

type DefaultAgent struct {
	config            AgentConfig
    model             Model
    env               Environment
 	messages          []Message
    extraTemplateVars map[string]any
    cost              float64
    nCalls            int
}

func NewDefaultAgent(cfg AgentConfig, model Model, env Environment) *DefaultAgent {
	return &DefaultAgent{
		config: 			cfg,
		model: 				model,
		env:    			env,
		messages: 			[]Message{},
		extraTemplateVars:  make(map[string]any),
	}
}

func (a *DefaultAgent) addMessages(msgs ...Message) []Message {
	a.messages = append(a.messages, msgs...)
	return msgs
}

func (a *DefaultAgent) query() (Message, error) {
	// Check limits (0 means unlimited)
    if a.config.StepLimit > 0 && a.nCalls >= a.config.StepLimit {
        return Message{}, NewLimitsExceededError()
    }
    if a.config.CostLimit > 0 && a.cost >= a.config.CostLimit {
        return Message{}, NewLimitsExceededError()
    }

    a.nCalls++
	msg, err := a.model.Query(a.messages)
    if err != nil {
        return Message{}, err
    }

	// Track cost
    if cost, ok := msg.Extra["cost"].(float64); ok {
        a.cost += cost
    }

    a.addMessages(msg)
    return msg, nil
}

func (a *DefaultAgent) executeActions(msg Message) ([]Message, error) {
    actions := a.getActions(msg)
    var outputs []Observation
    for _, action := range actions {
        obs, err := a.env.Execute(action)
        if err != nil {
            var flow *InterruptAgentFlowError
            if errors.As(err, &flow) {
                return nil, err
            }
            obs = Observation{Output: "", ReturnCode: -1, ExceptionInfo: err.Error()}
        }
        outputs = append(outputs, obs)
    }
    obsMessages := a.model.FormatObservationMessages(msg, outputs)
    return a.addMessages(obsMessages...), nil
}

func (a *DefaultAgent) getActions(msg Message) []Action {
    raw, ok := msg.Extra["actions"]
    if !ok {
        return nil
    }
    actions, ok := raw.([]Action)
    if !ok {
        return nil
    }
    return actions
}

func (a *DefaultAgent) getTemplateVars() map[string]any {
    vars := make(map[string]any)
    for k, v := range a.model.GetTemplateVars() {
        vars[k] = v
    }
    for k, v := range a.env.GetTemplateVars() {
        vars[k] = v
    }
    for k, v := range a.extraTemplateVars {
        vars[k] = v
    }
    return vars
}

func (a *DefaultAgent) step() error {
    msg, err := a.query()
    if err != nil {
        return err
    }
    _, err = a.executeActions(msg)
    return err
}

type RunResult struct {
    ExitStatus string
    Submission string
}

func (a *DefaultAgent) Run(task string) (RunResult, error) {
    a.extraTemplateVars["Task"] = task
    a.messages = []Message{}

    vars := a.getTemplateVars()
    sysContent, err := renderTemplate(a.config.SystemTemplate, vars)
    if err != nil {
        return RunResult{}, fmt.Errorf("rendering system template: %w", err)
    }
    instContent, err := renderTemplate(a.config.InstanceTemplate, vars)
    if err != nil {
        return RunResult{}, fmt.Errorf("rendering instance template: %w", err)
    }

    a.addMessages(
        a.model.FormatMessage("system", sysContent, nil),
        a.model.FormatMessage("user", instContent, nil),
    )

    for {
        err := a.step()
        if err != nil {
            var flow *InterruptAgentFlowError
            if errors.As(err, &flow) {
                a.addMessages(flow.Messages...)
            } else {
                // Uncaught exception
                a.addMessages(Message{
                    Role: "exit", Content: err.Error(),
                    Extra: map[string]any{"exit_status": "Error", "submission": ""},
                })
                a.save()
                return RunResult{}, err
            }
        }
        a.save()

        last := a.messages[len(a.messages)-1]
        if last.Role == "exit" {
            status, _ := last.Extra["exit_status"].(string)
            submission, _ := last.Extra["submission"].(string)
            return RunResult{ExitStatus: status, Submission: submission}, nil
        }
    }
}

func (a *DefaultAgent) serialize() map[string]any {
    var lastExtra map[string]any
    if len(a.messages) > 0 {
        lastExtra = a.messages[len(a.messages)-1].Extra
    }
    exitStatus, _ := lastExtra["exit_status"].(string)
    submission, _ := lastExtra["submission"].(string)

    data := map[string]any{
        "info": map[string]any{
            "model_stats": map[string]any{
                "instance_cost": a.cost,
                "api_calls":     a.nCalls,
            },
            "exit_status": exitStatus,
            "submission":  submission,
        },
        "messages":          a.messages,
        "trajectory_format": "mini-swe-agent-1.1",
    }
    return recursiveMerge(data, a.model.Serialize(), a.env.Serialize())
}

func (a *DefaultAgent) save() {
    if a.config.OutputPath == "" {
        return
    }
    data := a.serialize()
    b, _ := json.MarshalIndent(data, "", "  ")
    dir  := filepath.Dir(a.config.OutputPath)
    os.MkdirAll(dir, 0o755)
    os.WriteFile(a.config.OutputPath, b, 0o644)
}
package agent

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rvald/code-rig/internal/utils"
)

type InteractiveAgent struct {
	*DefaultAgent
	interactiveConfig InteractiveAgentConfig
	Stdin             io.Reader
	Stdout            io.Writer
}

func NewInteractiveAgent(cfg InteractiveAgentConfig, model Model, env Environment) *InteractiveAgent {
	// Base constructor
	baseCfg := cfg.AgentConfig
	base := NewDefaultAgent(baseCfg, model, env)

	return &InteractiveAgent{
		DefaultAgent:      base,
		interactiveConfig: cfg,
	}
}

func (a *InteractiveAgent) addMessages(msgs ...Message) []Message {
	out := a.Stdout
	if out == nil {
		out = os.Stdout
	}

	for _, msg := range msgs {
		role := msg.Role
		if role == "" {
			role = "unknown"
		}

		if role == "assistant" {
			// Include step and cost like Python does
			fmt.Fprintf(out, "\n[mini-swe-agent: step %d, $%.2f]:\n", a.nCalls, a.cost)
		} else {
			fmt.Fprintf(out, "\n%s:\n", role)
		}
		fmt.Fprintf(out, "%s\n", msg.Content)
	}

	// Call embedded base method to add to trajectory
	return a.DefaultAgent.addMessages(msgs...)
}

func (a *InteractiveAgent) query() (Message, error) {
	if a.interactiveConfig.Mode == "human" {
		cmd := a.promptUser("> ")

		if cmd != "/y" && cmd != "/c" && cmd != "/u" {
			msg := Message{
				Role:    "user",
				Content: fmt.Sprintf("User command:\n```bash\n%s\n```", cmd),
				Extra: map[string]any{
					"actions": []Action{{Command: cmd}},
				},
			}
			a.addMessages(msg) // Calls the InteractiveAgent printing version
			return msg, nil
		}
	}

	// Not human mode? Fall back to the real LM query
	return a.DefaultAgent.query()
}

func (a *InteractiveAgent) promptUser(prompt string) string {
	in := a.Stdin
	if in == nil {
		in = os.Stdin
	}
	out := a.Stdout
	if out == nil {
		out = os.Stdout
	}

	fmt.Fprint(out, prompt)
	scanner := bufio.NewScanner(in)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}
	return ""
}

func (a *InteractiveAgent) executeActions(msg Message) error {
	actions := a.GetActions(msg)
	if len(actions) == 0 {
		return nil
	}

	if a.interactiveConfig.Mode == "confirm" {
		// Build prompt
		prompt := fmt.Sprintf("Execute %d action(s)? [Enter] to confirm, or comment to reject\n> ", len(actions))
		userInput := a.promptUser(prompt)

		switch userInput {
		case "", "/y":
			// Confirmed, continue
		case "/u":
			return &UserInterruptionError{
				InterruptAgentFlowError: InterruptAgentFlowError{
					Messages: []Message{{Role: "user", Content: "Commands not executed. Switching to human mode", Extra: map[string]any{"interrupt_type": "UserRejection"}}},
				},
			}
		default:
			// Rejected with comment
			return &UserInterruptionError{
				InterruptAgentFlowError: InterruptAgentFlowError{
					Messages: []Message{{Role: "user", Content: "Commands not executed. User rejected: " + userInput, Extra: map[string]any{"interrupt_type": "UserRejection"}}},
				},
			}
		}
	}

	// If confirmed (or yolo mode), execute them
	var outputs []Observation
	for _, action := range actions {
		obs, err := a.env.Execute(action)
		if err != nil {
			// Check SubmittedError first (more specific) before the generic InterruptAgentFlowError
			var subErr *SubmittedError
			if errors.As(err, &subErr) {
				outputs = append(outputs, obs)
				obsMsgs := a.model.FormatObservationMessages(msg, outputs)
				a.addMessages(obsMsgs...)
				return a.checkForNewTaskOrSubmit(subErr)
			}
			var flow *InterruptAgentFlowError
			if errors.As(err, &flow) {
				return err
			}
			obs = Observation{ReturnCode: -1, ExceptionInfo: err.Error()}
		}
		outputs = append(outputs, obs)
	}

	obsMsgs := a.model.FormatObservationMessages(msg, outputs)
	a.addMessages(obsMsgs...)
	return nil
}

func (a *InteractiveAgent) checkForNewTaskOrSubmit(e *SubmittedError) error {
	if a.interactiveConfig.ConfirmExit {
		userInput := a.promptUser("Agent wants to finish. Type new task or [Enter] to quit\n> ")
		if userInput != "" && userInput != "/u" && userInput != "/c" && userInput != "/y" {
			// User provided a new task, interrupt the submission
			return &UserInterruptionError{
				InterruptAgentFlowError: InterruptAgentFlowError{
					Messages: []Message{{Role: "user", Content: "The user added a new task: " + userInput, Extra: map[string]any{"interrupt_type": "UserNewTask"}}},
				},
			}
		}
	}
	return e // Propagate the exit if no new task
}

func (a *InteractiveAgent) step() error {
	msg, err := a.query()
	if err != nil {
		return err
	}
	return a.executeActions(msg)
}

func (a *InteractiveAgent) Run(task string) (RunResult, error) {
	a.extraTemplateVars["Task"] = task
	a.messages = []Message{}

	vars := a.getTemplateVars()
	sysContent, err := utils.RenderTemplate(a.config.SystemTemplate, vars)
	if err != nil {
		return RunResult{}, fmt.Errorf("rendering system template: %w", err)
	}
	instContent, err := utils.RenderTemplate(a.config.InstanceTemplate, vars)
	if err != nil {
		return RunResult{}, fmt.Errorf("rendering instance template: %w", err)
	}

	a.addMessages(
		a.model.FormatMessage("system", sysContent, nil),
		a.model.FormatMessage("user", instContent, nil),
	)

	for {
		err := a.step() // Calls InteractiveAgent step override
		if err != nil {
			var flow *InterruptAgentFlowError
			if errors.As(err, &flow) {
				a.addMessages(flow.Messages...)
			} else {
				a.addMessages(Message{
					Role: "exit", Content: err.Error(),
					Extra: map[string]any{"exit_status": "Error", "submission": ""},
				})
				a.Save("last_mini_run.traj.json")
				return RunResult{}, err
			}
		}
		a.Save("last_mini_run.traj.json")

		last := a.messages[len(a.messages)-1]
		if last.Role == "exit" {
			status, _ := last.Extra["exit_status"].(string)
			submission, _ := last.Extra["submission"].(string)
			return RunResult{ExitStatus: status, Submission: submission}, nil
		}
	}
}

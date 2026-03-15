# TDD Guide: Interactive Agent in Go — Phase 15

This guide walks through extending the `DefaultAgent` into an `InteractiveAgent` that puts a human in the loop. This follows strict TDD (red-green-refactor). 

By the end of this phase, your agent will support three modes:
- **yolo**: execute LM commands immediately (the `DefaultAgent` behavior)
- **confirm**: prompt the user before executing LM commands
- **human**: let the user type and execute bash commands directly

> [!IMPORTANT]
> **Source of truth:** Always refer back to [agents/interactive.py](file:///home/rvald/mini-swe-agent/src/minisweagent/agents/interactive.py) when in doubt about behavior.

---

## How the Python InteractiveAgent Works (Reference)

`InteractiveAgent` inherits from `DefaultAgent`. It overrides key methods to inject UI printing and human-in-the-loop pauses.

| Python Method | What it does | Go Equivalent |
|---|---|---|
| `add_messages` | Prints messages to the console using `rich` before appending to trajectory. | `AddMessages` override |
| `query` | If `mode == human`, gets a command from the user instead of calling the LM. Catches `LimitsExceeded` to ask user for new limits. | `Query` override |
| `step` | Wraps the `step` loop in a try/except for `KeyboardInterrupt` (Ctrl+C). Allows user to interrupt the LM. | `Step` override |
| `execute_actions` | Prompts user before executing actions (if in `confirm` mode). Catches `Submitted` to ask user if they want to exit or provide a new task. | `ExecuteActions` override |
| `_ask_confirmation...` | The UI loop for `/y`, `/c`, `/u` commands. | Internal helper |

### Terminal UI Considerations in Go
Python uses `rich` for formatting and `prompt_toolkit` for interactive multiline input with history.
In Go, you can use:
- `github.com/fatih/color` or `github.com/charmbracelet/lipgloss` for styling.
- `github.com/chzyer/readline` or `github.com/charmbracelet/huh` for prompts and history.
- Context cancellation (`context.WithCancel`) tied to `os.Signal` for handling Ctrl+C interruptions gracefully.

---

## File Structure

```
internal/agent/
├── interactive.go         # InteractiveAgent logic
└── interactive_test.go    # Tests involving mock UI inputs
```

Since `InteractiveAgent` is a specialized version of `DefaultAgent`, we will embed it in Go to achieve inheritance-like behavior.

---

## Phase 1: Configuration and Constructor

### Step 1.1 — InteractiveAgentConfig

**What it does:** Adds `mode`, `whitelist_actions`, and `confirm_exit` to the base config.

**🔴 RED** — In `interactive_test.go`:

```go
func TestInteractiveConfigDefaults(t *testing.T) {
    // Mode should default to "confirm" in the constructor or config parser
    cfg := InteractiveAgentConfig{
        AgentConfig: AgentConfig{SystemTemplate: "sys"},
        Mode:        "confirm",
        ConfirmExit: true,
    }
    if cfg.Mode != "confirm" {
        t.Errorf("Mode = %q, want 'confirm'", cfg.Mode)
    }
    if len(cfg.WhitelistActions) != 0 {
        t.Error("WhitelistActions should be empty")
    }
}
```

**🟢 GREEN** — In `types.go` or `interactive.go`:

```go
type InteractiveAgentConfig struct {
    AgentConfig      // Embed the base config
    Mode             string   `json:"mode" yaml:"mode"`                           // "human", "confirm", "yolo"
    WhitelistActions []string `json:"whitelist_actions" yaml:"whitelist_actions"` // Regex patterns
    ConfirmExit      bool     `json:"confirm_exit" yaml:"confirm_exit"`
}
```

---

### Step 1.2 — Embedding DefaultAgent

**What it does:** Go doesn't have class inheritance. We embed `*DefaultAgent` in `InteractiveAgent` so it automatically gains `Run()`, `save()`, etc. We then attach new methods to `InteractiveAgent` that shadow or wrap the embedded ones.

**🔴 RED:**

```go
func TestNewInteractiveAgentIsDefaultAgent(t *testing.T) {
    cfg := InteractiveAgentConfig{Mode: "confirm"}
    agent := NewInteractiveAgent(cfg, &MockModel{}, &MockEnv{})

    if agent == nil {
        t.Fatal("expected non-nil agent")
    }
    // It should have access to DefaultAgent fields/methods
    if agent.cost != 0.0 {
        t.Errorf("expected inherited cost to be 0")
    }
}
```

**🟢 GREEN** — In `interactive.go`:

```go
type InteractiveAgent struct {
    *DefaultAgent
    interactiveConfig InteractiveAgentConfig
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
```

---

## Phase 2: Intercepting `add_messages` (Console Output)

### Step 2.1 — Print and Append

**What Python does:** Iterates over messages, prints them to the console with rich formatting, then calls `super().add_messages()`.

**🔴 RED:**

Testing console printing directly is tricky. A good pattern is to inject an `io.Writer` (defaulting to `os.Stdout`) so we can capture output in tests.

```go
func TestInteractiveAddMessagesPrints(t *testing.T) {
    var buf bytes.Buffer
    agent := NewInteractiveAgent(InteractiveAgentConfig{}, &MockModel{}, &MockEnv{})
    agent.Stdout = &buf // Assuming you add this field for testing
    
    msg := Message{Role: "user", Content: "hello world"}
    agent.addMessages(msg) // We must ensure this calls the *InteractiveAgent* method, not the embedded one
    
    if len(agent.messages) != 1 {
        t.Errorf("message not appended to trajectory")
    }
    
    output := buf.String()
    if !strings.Contains(output, "User:") || !strings.Contains(output, "hello world") {
        t.Errorf("output did not contain expected formatting, got %q", output)
    }
}
```

**🟢 GREEN:**

```go
import (
    "fmt"
    "io"
    "os"
)

// Add to InteractiveAgent struct: Stdout io.Writer

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
```

> [!CAUTION]
> **Method Shadowing vs Virtual Dispatch.** In Go, if `a.DefaultAgent.Run()` calls `a.addMessages()`, it will call `DefaultAgent.addMessages`, NOT `InteractiveAgent.addMessages`. Go does not have late binding / dynamic dispatch for struct methods!
> 
> To fix this, you must refactor `DefaultAgent` so its loop calls methods on an interface, OR you must override `step()` and `executeActions()` in `InteractiveAgent` entirely so they explicitly call `a.addMessages()` on the `InteractiveAgent` directly. Python uses `super()`, Go requires manual wrappers.

---

## Phase 3: Human Mode & Query Interception

### Step 3.1 — Human Mode Skips the LLM

**What Python does:** In `query()`, if `mode == "human"`, it prompts the user with `> `, parses slash commands, and returns a synthetic `Message` containing the user's bash command as an `action`.

**🔴 RED:**

```go
func TestQueryHumanMode(t *testing.T) {
    var inBuf, outBuf bytes.Buffer
    inBuf.WriteString("ls -la\n") // User types this
    
    agent := NewInteractiveAgent(InteractiveAgentConfig{Mode: "human"}, &MockModel{}, &MockEnv{})
    agent.Stdin = &inBuf
    agent.Stdout = &outBuf

    msg, err := agent.query()
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    
    // It should NOT have called the MockModel
    if agent.nCalls != 0 {
        t.Errorf("nCalls = %d, want 0 (human mode skips LLM)", agent.nCalls)
    }
    
    // It should have built a synthetic message
    if msg.Role != "user" {
        t.Errorf("Role = %q, want 'user'", msg.Role)
    }
    actions, ok := msg.Extra["actions"].([]Action)
    if !ok || len(actions) != 1 || actions[0].Command != "ls -la" {
        t.Errorf("actions = %v, want [{Command: 'ls -la'}]", actions)
    }
}
```

**🟢 GREEN:**

```go
import (
    "bufio"
    "strings"
)

func (a *InteractiveAgent) query() (Message, error) {
    if a.interactiveConfig.Mode == "human" {
        cmd := a.promptUser("> ")
        
        // Assume slash command logic is handled inside promptUser or here
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
```

---

## Phase 4: Confirmation Mode & `execute_actions` Interception

### Step 4.1 — Confirming Actions

**What Python does:** Before calling `env.execute()`, if `mode == "confirm"`, it shows the commands and waits for `Enter` (allow) or text (reject).

**🔴 RED:**

```go
func TestExecuteActionsConfirmModeReject(t *testing.T) {
    var inBuf bytes.Buffer
    inBuf.WriteString("no, do something else\n") // User rejects
    
    env := &MockEnv{}
    agent := NewInteractiveAgent(InteractiveAgentConfig{Mode: "confirm"}, &MockModel{}, env)
    agent.Stdin = &inBuf
    
    msg := Message{Extra: map[string]any{"actions": []Action{{Command: "rm -rf /"}}}}
    
    // We expect a UserInterruption error (a type of InterruptAgentFlowError)
    err := agent.executeActions(msg)
    
    var flowErr *InterruptAgentFlowError
    if !errors.As(err, &flowErr) {
        t.Fatalf("expected InterruptAgentFlowError from rejection, got %v", err)
    }
    if env.CallCount != 0 {
        t.Error("environment should not execute rejected commands")
    }
    if flowErr.Messages[0].Extra["interrupt_type"] != "UserRejection" {
        t.Errorf("interrupt_type = %v", flowErr.Messages[0].Extra["interrupt_type"])
    }
}
```

**🟢 GREEN:**

```go
// New error type for internal/agent/errors.go
type UserInterruptionError struct {
    InterruptAgentFlowError
}

func (a *InteractiveAgent) executeActions(msg Message) error {
    actions := a.getActions(msg)
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
            obs = Observation{ReturnCode: -1, ExceptionInfo: err.Error()}
        }
        outputs = append(outputs, obs)
        
        // Note: Check for SubmittedError here like Python does in try/except!
        var subErr *SubmittedError
        if errors.As(err, &subErr) {
            // Output observation messages first (finally block in python)
            obsMsgs := a.model.FormatObservationMessages(msg, outputs)
            a.addMessages(obsMsgs...)
            return a.checkForNewTaskOrSubmit(subErr)
        }
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
```

---

## Phase 5: Tying the Loop Together

### Step 5.1 — The Step Loop

Because Go doesn't dispatch `DefaultAgent.Run()` calls to `InteractiveAgent` overrides automatically, we must override `step()`.

```go
func (a *InteractiveAgent) step() error {
    // 1. Query (uses InteractiveAgent.query)
    msg, err := a.query()
    if err != nil {
        return err
    }
    // 2. Execute (uses InteractiveAgent.executeActions)
    return a.executeActions(msg)
}
```

We technically don't need to override `Run()` unless we want to catch Ctrl+C (KeyboardInterrupt) there. In Go, trapping Ctrl+C requires setting up `os.Notify` and passing a context through your `Agent` methods (or using a channel). That's slightly complex and can be implemented as a final polish step.

---

## Summary — Implementation Order

| Step | Test file | Production file | What you're proving |
|---|---|---|---|
| 1.1 | `TestInteractiveConfigDefaults` | `interactive.go` | Extended config properties |
| 1.2 | `TestNewInteractiveAgent...` | `interactive.go` | Constructor and struct embedding |
| 2.1 | `TestInteractiveAddMessagesPrints` | `interactive.go` | Intercept console printing |
| 3.1 | `TestQueryHumanMode` | `interactive.go` | `/u` bypasses the LLM entirely |
| 4.1 | `TestExecuteActionsConfirmModeReject` | `interactive.go` | User can reject LLM tool calls |
| 5.1 | (Integration config) | `interactive.go` | `step()` explicitly routes to overridden methods |

This completes the `InteractiveAgent`. By adding this layer, `mini-swe-agent` goes from a headless CI tool to a collaborative terminal copilot!

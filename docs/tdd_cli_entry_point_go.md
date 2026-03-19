# TDD Guide: CLI Entry Point in Go — Phase 14 (Deep Dive)

This guide walks through implementing a robust command-line interface (CLI) for your Go agent using strict TDD (red-green-refactor). 

Because the CLI is the glue that binds `Config`, `Model`, `Environment`, and `Agent` together, it must handle:
1. Interactive prompts (fallback when args are missing)
2. Complex YAML config loading and merging
3. On-the-fly dot-notation config overrides (e.g., `-c agent.cost_limit=5`)
4. Dependency injection to wire up the core structs
5. First-time setup configuration and `.env` management

> [!IMPORTANT]
> **Source of truth:** Always refer back to [run/mini.py](file:///home/rvald/mini-swe-agent/src/minisweagent/run/mini.py), [run/utilities/config.py](file:///home/rvald/mini-swe-agent/src/minisweagent/run/utilities/config.py), and [config/__init__.py](file:///home/rvald/mini-swe-agent/src/minisweagent/config/__init__.py) when in doubt.

---

## File Structure

For Go CLI applications, standard layout dictates the `main` package lives in `cmd/`, while the testable application logic lives in `internal/app` or `internal/cli`.

```
cmd/mini/
└── main.go               # Minimal wrapper: cli.Main()
internal/cli/
├── setup.go              # First-time config (.env generation)
├── setup_test.go         
├── config_merge.go       # Parsing dot-notation and merging YAMLs
├── config_merge_test.go  
├── app.go                # Flag parsing and constructor wiring
└── app_test.go           
```

---

## Phase 1: Config Spec Parsing and Merging

Before the app can run, it must gather settings. Python does this by parsing a list of `-c` arguments which can be either **file paths** (`mini.yaml`) or **key-value overrides** (`model.model_kwargs.temperature=0.5`). 

### Step 1.1 — Dot-Notation -> Nested Map

**What Python does:** `_key_value_spec_to_nested_dict` splits on `=` and then recursively creates dictionaries from the dot-separated keys.

**🔴 RED** — In `config_merge_test.go`:

```go
func TestParseKeyValueSpec(t *testing.T) {
    tests := []struct{
        input string
        want  map[string]any
    }{
        {
            input: "agent.cost_limit=5",
            want:  map[string]any{"agent": map[string]any{"cost_limit": 5.0}}, // Note json/yaml unmarshals nums as float64 usually
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
```

**🟢 GREEN** — In `config_merge.go`:

```go
import (
    "encoding/json"
    "fmt"
    "strconv"
    "strings"
)

// ParseKeyValueSpec converts "a.b.c=val" into map[string]any{"a": {"b": {"c": val}}}
func ParseKeyValueSpec(spec string) (map[string]any, error) {
    parts := strings.SplitN(spec, "=", 2)
    if len(parts) != 2 {
        return nil, fmt.Errorf("invalid spec format, expected key=value: %s", spec)
    }
    
    keyPath := parts[0]
    rawVal := parts[1]
    
    // Attempt to parse value as JSON (handles bools, ints, floats)
    var parsedVal any
    if err := json.Unmarshal([]byte(rawVal), &parsedVal); err != nil {
        // Fallback to raw string if it's not valid JSON (like "yolo")
        parsedVal = rawVal
    }

    keys := strings.Split(keyPath, ".")
    result := make(map[string]any)
    current := result

    for i := 0; i < len(keys)-1; i++ {
        k := keys[i]
        next := make(map[string]any)
        current[k] = next
        current = next
    }
    
    current[keys[len(keys)-1]] = parsedVal
    return result, nil
}
```

### Step 1.2 — Resolving the Config Spec List

**What Python does:** Iterates over the `-c` flags. If it contains `=`, parse as key-value. Otherwise, find the `.yaml` file and parse it. Finally, `recursive_merge` them all together.

**🔴 RED:**

```go
func TestBuildFinalConfigFromSpecs(t *testing.T) {
    // Requires a dummy YAML file and your RecursiveMerge implementation from Phase 4
    specs := []string{
        "testdata/base.yaml",
        "agent.step_limit=50",
    }
    
    // testdata/base.yaml contains:
    // agent:
    //   mode: confirm
    //   step_limit: 10
    
    merged, err := BuildFinalConfigFromSpecs(specs)
    if err != nil {
        t.Fatal(err)
    }
    
    agentBlock, _ := merged["agent"].(map[string]any)
    if agentBlock["mode"] != "confirm" {
        t.Errorf("expected mode=confirm from base.yaml")
    }
    if agentBlock["step_limit"] != float64(50) {
        t.Errorf("expected step_limit=50 from override, got %v", agentBlock["step_limit"])
    }
}
```

**🟢 GREEN:**

Use the `RecursiveMerge` function you already built for `DefaultAgent` (or `internal/utils`).

```go
func BuildFinalConfigFromSpecs(specs []string) (map[string]any, error) {
    var configs []map[string]any
    
    for _, spec := range specs {
        if strings.Contains(spec, "=") {
            override, err := ParseKeyValueSpec(spec)
            if err != nil {
                return nil, err
            }
            configs = append(configs, override)
        } else {
            fileConfig, err := LoadYAMLFile(spec) // Implementation relies on os.ReadFile + yaml.Unmarshal
            if err != nil {
                return nil, err
            }
            configs = append(configs, fileConfig)
        }
    }
    
    // Merge all sequentially
    result := make(map[string]any)
    for _, cfg := range configs {
        result = utils.RecursiveMerge(result, cfg)
    }
    return result, nil
}
```

---

## Phase 2: First-Time Setup (`configure_if_first_time`)

Python's `run/mini.py` calls `configure_if_first_time()`, which checks if `MSWEA_CONFIGURED` is set. If not, it runs an interactive wizard to create a `.env` file holding the default model and API key.

### Step 2.1 — Checking for Configuration Status

**🔴 RED** — In `setup_test.go`:

```go
func TestIsConfigured(t *testing.T) {
    os.Unsetenv("MSWEA_CONFIGURED")
    if IsConfigured() {
        t.Error("expected false when env var missing")
    }
    
    os.Setenv("MSWEA_CONFIGURED", "true")
    if !IsConfigured() {
        t.Error("expected true when env var set")
    }
}
```

**🟢 GREEN** — In `setup.go`:

```go
func IsConfigured() bool {
    // In Go, typically we use godotenv.Load() to load .env files into os.Environ()
    return os.Getenv("MSWEA_CONFIGURED") == "true"
}
```

### Step 2.2 — The Setup Wizard

**What Python does:** Uses `prompt_toolkit` to ask for Model and API Key, then writes to `{global_config_dir}/.env`.

*Note: For TDD, wrap this behind an interface or pass `io.Reader`/`io.Writer` to test the prompt flow cleanly.*

**🔴 RED:**

```go
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
```

**🟢 GREEN:**

```go
func RunSetupWizard(in io.Reader, out io.Writer, envFilePath string) error {
    scanner := bufio.NewScanner(in)
    
    fmt.Fprintln(out, "To get started, we need to set up your global config file.")
    fmt.Fprint(out, "Enter your default model (e.g., openai/gpt-4o): ")
    scanner.Scan()
    model := strings.TrimSpace(scanner.Text())
    
    fmt.Fprint(out, "Enter your API key name (e.g., OPENAI_API_KEY): ")
    scanner.Scan()
    keyName := strings.TrimSpace(scanner.Text())
    
    var keyValue string
    if keyName != "" {
        fmt.Fprintf(out, "Enter your API key value for %s: ", keyName)
        scanner.Scan()
        keyValue = strings.TrimSpace(scanner.Text())
    }
    
    // Write to file (in Python this uses dotenv.set_key)
    f, err := os.OpenFile(envFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
    if err != nil {
        return err
    }
    defer f.Close()
    
    if model != "" {
        fmt.Fprintf(f, "MSWEA_MODEL_NAME=\"%s\"\n", model)
    }
    if keyName != "" && keyValue != "" {
        fmt.Fprintf(f, "%s=\"%s\"\n", keyName, keyValue)
    }
    fmt.Fprintf(f, "MSWEA_CONFIGURED=\"true\"\n")
    
    return nil
}
```

---

## Phase 3: The `App` Struct and Flag Parsing

Python uses `typer.Option`. In Go, we'll use `flag` to construct our base configuration overrides.

### Step 3.1 — Parse CLI Flags

**🔴 RED:**

```go
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
```

**🟢 GREEN:**

```go
type stringSlice []string
func (i *stringSlice) String() string { return fmt.Sprint(*i) }
func (i *stringSlice) Set(value string) error {
    *i = append(*i, value)
    return nil
}

type App struct {
    Task            string
    Model           string
    ModelClass      string
    AgentClass      string
    EnvironmentClass string
    Yolo            bool
    CostLimit       float64
    ConfigSpecs     stringSlice
    OutputPath      string
    ExitImmediately bool
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
```

---

## Phase 4: Constructing the Final Payload & Dependency Injection

The magic of `mini.py` is passing the CLI-derived `configs` dict into the `get_agent()`, `get_environment()`, and `get_model()` functions. 

You need factories in Go that map these strings (`model_class="litellm"`) to struct implementations.

### Step 4.1 — Final Config Construction

**🔴 RED:**

```go
func TestAppBuildFinalConfig(t *testing.T) {
    app := NewApp()
    app.Yolo = true
    app.Task = "test task"
    
    finalMap := app.BuildOverrideMap()
    
    agentSection := finalMap["agent"].(map[string]any)
    if agentSection["mode"] != "yolo" {
        t.Errorf("Expected mode=yolo")
    }
    runSection := finalMap["run"].(map[string]any)
    if runSection["task"] != "test task" {
        t.Errorf("Expected task=test task")
    }
}
```

**🟢 GREEN:**

```go
func (a *App) BuildOverrideMap() map[string]any {
    // Mimics the `configs.append({...})` block in mini.py line 73
    m := map[string]any{
        "run": map[string]any{},
        "agent": map[string]any{},
        "model": map[string]any{},
        "environment": map[string]any{},
    }
    
    if a.Task != "" { m["run"].(map[string]any)["task"] = a.Task }
    
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
    
    if a.Model != "" { m["model"].(map[string]any)["model_name"] = a.Model }
    if a.ModelClass != "" { m["model"].(map[string]any)["model_class"] = a.ModelClass }
    if a.EnvironmentClass != "" { m["environment"].(map[string]any)["environment_class"] = a.EnvironmentClass }
    if a.AgentClass != "" { m["agent"].(map[string]any)["agent_class"] = a.AgentClass }
    
    return m
}
```

### Step 4.2 — Component Wiring (Factories)

Because Go doesn't have Python's `importlib.import_module`, we map strings manually.

```go
// internal/cli/factories.go
func GetEnvironment(envClass string, cfg map[string]any) (environment.Environment, error) {
    if envClass == "" || envClass == "local" {
        envCfg, _ := config.BuildEnvironmentConfig(cfg) // From Phase 12
        return environment.NewLocalEnvironment(envCfg), nil
    }
    if envClass == "docker" {
        // ... build docker config
        return environment.NewDockerEnvironment(dockerCfg)
    }
    return nil, fmt.Errorf("unknown environment class: %s", envClass)
}

func GetModel(modelClass string, cfg map[string]any) (agent.Model, error) {
    if modelClass == "" || modelClass == "litellm" {
        modelCfg, _ := config.BuildModelConfig(cfg)
        return model.NewOpenAIModel(modelCfg, os.Getenv("OPENAI_API_KEY")), nil
    }
    return nil, fmt.Errorf("unknown model class")
}

func GetAgent(agentClass string, m agent.Model, e environment.Environment, cfg map[string]any) (agent.Agent, error) {
    if agentClass == "" || agentClass == "interactive" {
        agentCfg, _ := config.BuildInteractiveAgentConfig(cfg)
        return agent.NewInteractiveAgent(agentCfg, m, e), nil
    }
    return nil, fmt.Errorf("unknown agent class")
}
```

---

## Phase 5: The Execution Path

Tie it all together in `Execute()`.

```go
func (a *App) Execute(args []string) error {
    if !IsConfigured() {
        RunSetupWizard(os.Stdin, os.Stdout, ".env")
    }

    // 1. Parse Args
    if err := a.ParseArgs(args); err != nil {
        return err
    }

    // 2. Default Config Spec behavior
    if len(a.ConfigSpecs) == 0 {
        a.ConfigSpecs = []string{"mini.yaml"}
    }

    // 3. Build & merge maps
    fileConfigMap, err := BuildFinalConfigFromSpecs(a.ConfigSpecs)
    if err != nil {
        return err
    }
    
    finalMap := utils.RecursiveMerge(fileConfigMap, a.BuildOverrideMap())

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
    env, _ := GetEnvironment(envClass, finalMap["environment"].(map[string]any))

    modelClass, _ := finalMap["model"].(map[string]any)["model_class"].(string)
    mod, _ := GetModel(modelClass, finalMap["model"].(map[string]any))

    agentClass, _ := finalMap["agent"].(map[string]any)["agent_class"].(string)
    ag, _ := GetAgent(agentClass, mod, env, finalMap["agent"].(map[string]any))

    // 6. Run
    _, err = ag.Run(task) // Depending on your return signature
    
    // 7. Save trajectory
    outPath, _ := finalMap["agent"].(map[string]any)["output_path"].(string)
    if outPath != "" {
        ag.Save(outPath) // Implement Save on DefaultAgent if not already done
        fmt.Printf("Saved trajectory to '%s'\n", outPath)
    }

    return err
}
```

### Run it!

```go
// cmd/mini/main.go
package main

import (
    "os"
    "fmt"
    "your_repo/internal/cli"
)

func main() {
    app := cli.NewApp()
    if err := app.Execute(os.Args[1:]); err != nil {
        fmt.Fprintf(os.Stderr, "Fatal error: %v\n", err)
        os.Exit(1)
    }
}
```

## Summary — Deep Port Highlights

1. **Config Spec Parsing:** You must handle both strings (`mini.yaml`) AND key=values (`agent.yolo=true`). `ParseKeyValueSpec` solves this.
2. **Dynamic Wiring:** Python relies on `importlib` and arbitrary dicts. Go relies on strict types, requiring explicit factory switches (`GetEnvironment`, etc.) mapping config dictionaries to strong structures before passing to constructors.
3. **Dotenv Lifecycle:** Calling `IsConfigured()` early ensures users have an easy onboarding experience before the app panics on a missing API key.

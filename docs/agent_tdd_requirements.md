# DefaultAgent TDD Implementation Breakdown

This document outlines a hyper-granular, red-green-refactor approach for the `DefaultAgent` logic. We will ignore `InteractiveAgent` entirely for now.

All work will take place in `internal/agent`. We will start by defining the absolute minimum structs and mock interfaces needed to pass each step.

## Phase 1: Core Domain & Initialization
*Goal: The agent requires basic config and can initialize a trajectory with formatted system/user prompts.*

- [ ] **Step 1: Domain Structs**
  - **Red**: Write a test verifying that `AgentConfig`, `Message`, `Action`, and `Observation` structs can be instantiated.
  - **Green**: Define empty structs in `types.go`.
- [ ] **Step 2: Interfaces definition**
  - **Red**: Write a test that accepts an `Environment` (with `Execute`) and `Model` (with `Query`) interface, using a simple mock struct.
  - **Green**: Define the `Environment` and `Model` interfaces in `types.go`.
- [ ] **Step 3: NewAgent Constructor**
  - **Red**: Test `NewDefaultAgent(cfg, model, env)` returns an initialized agent with 0 cost, 0 steps, and an empty trajectory.
  - **Green**: Implement `NewDefaultAgent` in `default.go`.
- [ ] **Step 4: Template Rendering**
  - **Red**: Write a test for an internal `renderTemplate(template, vars)`. Given `"Hello {{.Name}}"`, it should return `"Hello World"`.
  - **Green**: Implement `renderTemplate` using `text/template`.
- [ ] **Step 5: Agent run initialization loop**
  - **Red**: Call `agent.Run("Fix bug")`. Assert that the agent's `messages` slice immediately gets exactly two messages: a System message (configured via `cfg.SystemTemplate`) and a User message (configured via `cfg.InstanceTemplate`, containing "Fix bug"). For this test, immediately exit the Run loop.
  - **Green**: Implement the initial message injection logic in `Run()`.

## Phase 2: The Core `Step()` Method
*Goal: The agent queries the model, receives action(s), executes them, and records the observation.*

- [ ] **Step 6: Model Querying**
  - **Red**: In `Step()`, mock the `Model` to return a `Message` with `Role="assistant"` and content `"Thinking..."`. Assert this message is appended to the agent's trajectory.
  - **Green**: Implement `agent.model.Query(agent.messages)` and append the result.
- [ ] **Step 7: Action Extraction**
  - **Red**: Expand the mock `Model` message to contain a tool call/`Action` (e.g., `bash: echo "hi"`). Assert that `Step()` detects this action (does not execute it yet), and the test passes.
  - **Green**: Check for actions in the returned `Message.Extra.Actions`.
- [ ] **Step 8: Environment Execution**
  - **Red**: Mock the `Environment` to return an `Observation` (Output: "hi", ExitCode: 0) when `Execute()` is called. Assert that `env.Execute()` was actually called with the exact action returned by the model.
  - **Green**: Loop over actions and call `env.Execute(action)`.
- [ ] **Step 9: Observation Appending**
  - **Red**: Assert that after the environment executes, an `Observation` message (Role: "tool/user") is appended to the trajectory. If the `Observation` has exit code 0, it should be formatted accordingly.
  - **Green**: Implement logic to format the `Observation` using another Go template (or hardcoded string for now) and append it to `agent.messages`.

## Phase 3: Limits & Cost Tracking
*Goal: The agent stops if it takes too many steps or exceeds an API cost limit.*

- [ ] **Step 10: Step Limits**
  - **Red**: Initialize an agent with `StepLimit: 2`. Run `Step()` 3 times. The 3rd time should return an error (e.g. `LimitsExceeded`). The `Model` should only be queried twice.
  - **Green**: Add a `n_calls` counter to the agent and an `if n_calls >= limit` check inside `Query()`.
- [ ] **Step 11: Cost Tracking**
  - **Red**: Initialize an agent with `CostLimit: 1.0`. The mock `Model` returns a `Message` with `Extra.Cost: 0.6`. Run `Step()` twice. The 2nd call should trigger `LimitsExceeded`.
  - **Green**: Add a `cost` accumulator and update it after every `Model` query.

## Phase 4: Task Completion & Serialization
*Goal: The agent can successfully complete a task and save its history to disk.*

- [ ] **Step 12: Submission Sentinel**
  - **Red**: The `Environment` returns an output containing `"COMPLETE_TASK_AND_SUBMIT_FINAL_OUTPUT"` with exit code 0. Assert that `Step()` returns a specific `SubmittedError` containing the submission payload, halting the run loop.
  - **Green**: Implement the delimiter check inside the `ExecuteActions` phase.
- [ ] **Step 13: JSON Trajectory Serialization**
  - **Red**: Add `OutputPath: "/tmp/trajectory.json"` to the config. Run one `Step()`. Assert that the file is created and contains valid JSON matching the agent's message slice.
  - **Green**: Implement an internal `save()` method using `encoding/json` and `os.WriteFile`, called at the end of `Step()` (or in `defer` inside `Run()`).

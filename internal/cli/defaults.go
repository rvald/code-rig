package cli

// GetDefaultConfig returns the default configuration map, equivalent to Python's
// built-in mini.yaml. This provides system/instance templates for the agent,
// observation/format_error templates for the model, and environment defaults.
// It is merged as the base layer so any user config or CLI flags override it.
func GetDefaultConfig() map[string]any {
	return map[string]any{
		"agent": map[string]any{
			"system_template": `You are a helpful assistant that can interact with a computer.`,
			"instance_template": `Please solve this issue: {{.Task}}

You can execute bash commands and edit files to implement the necessary changes.

## Recommended Workflow

This workflow should be done step-by-step so that you can iterate on your changes and any possible problems.

1. Analyze the codebase by finding and reading relevant files
2. Create a script to reproduce the issue
3. Edit the source code to resolve the issue
4. Verify your fix works by running your script again
5. Test edge cases to ensure your fix is robust
6. Submit your changes and finish your work by issuing the following command: ` + "`echo COMPLETE_TASK_AND_SUBMIT_FINAL_OUTPUT`" + `.
   Do not combine it with any other command. After this command, you cannot continue working on this task.

## Command Execution Rules

You are operating in an environment where

1. You issue at least one command
2. The system executes the command(s) in a subshell
3. You see the result(s)
4. You write your next command(s)

Each response should include:

1. **Reasoning text** where you explain your analysis and plan
2. At least one tool call with your command

**CRITICAL REQUIREMENTS:**

- Your response SHOULD include reasoning text explaining what you're doing
- Your response MUST include AT LEAST ONE bash tool call
- Directory or environment variable changes are not persistent. Every action is executed in a new subshell.
- Submit your changes and finish your work by issuing the following command: ` + "`echo COMPLETE_TASK_AND_SUBMIT_FINAL_OUTPUT`" + `.
  Do not combine it with any other command. After this command, you cannot continue working on this task.

System: {{.system}} {{.machine}}
`,
			"step_limit": 0,
			"cost_limit": 3.0,
			"mode":       "confirm",
		},
		"model": map[string]any{
			"observation_template": `{"returncode": {{.Output.ReturnCode}}, "output": "{{.Output.Output}}"}`,
			"format_error_template": `Tool call error:

<error>
{{.Error}}
</error>

Every response needs to use the 'bash' tool at least once to execute commands.

Call the bash tool with your command as the argument:
- Tool: bash
- Arguments: {"command": "your_command_here"}

If you want to end the task, please issue the following command: ` + "`echo COMPLETE_TASK_AND_SUBMIT_FINAL_OUTPUT`" + `
without any other command.`,
			"model_kwargs": map[string]any{},
		},
		"environment": map[string]any{
			"env": map[string]any{
				"PAGER":            "cat",
				"MANPAGER":         "cat",
				"LESS":             "-R",
				"PIP_PROGRESS_BAR": "off",
				"TQDM_DISABLE":     "1",
			},
		},
	}
}

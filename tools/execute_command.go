package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func NewExecuteCommandTool(workspace *Workspace, defaultTimeout time.Duration) ToolDefinition {
	return ToolDefinition{
		Name:        "execute_command",
		Description: "Run a local shell command inside the workspace. Use this to verify builds, tests, or inspect the project state after edits.",
		InputSchema: ExecuteCommandInputSchema,
		Function: func(ctx context.Context, input json.RawMessage) (string, error) {
			return ExecuteCommand(workspace, defaultTimeout, ctx, input)
		},
	}
}

type ExecuteCommandInput struct {
	Command        string `json:"command" jsonschema_description:"The shell command to run."`
	Workdir        string `json:"workdir,omitempty" jsonschema_description:"Optional relative working directory inside the workspace."`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty" jsonschema_description:"Optional timeout in seconds. Defaults to the agent command timeout."`
}

var ExecuteCommandInputSchema = GenerateSchema[ExecuteCommandInput]()

func ExecuteCommand(workspace *Workspace, defaultTimeout time.Duration, ctx context.Context, input json.RawMessage) (string, error) {
	var params ExecuteCommandInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}
	if strings.TrimSpace(params.Command) == "" {
		return "", fmt.Errorf("command is required")
	}

	workdir := "."
	if params.Workdir != "" {
		workdir = params.Workdir
	}
	absDir, relDir, err := workspace.Resolve(workdir)
	if err != nil {
		return "", err
	}

	timeout := defaultTimeout
	if params.TimeoutSeconds > 0 {
		timeout = time.Duration(params.TimeoutSeconds) * time.Second
	}
	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(commandCtx, "sh", "-lc", params.Command)
	cmd.Dir = absDir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	runErr := cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	stdoutText, stdoutTruncated := truncateOutput(stdout.String(), 12000)
	stderrText, stderrTruncated := truncateOutput(stderr.String(), 12000)

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("command: %s\n", params.Command))
	builder.WriteString(fmt.Sprintf("workdir: %s\n", relDir))
	builder.WriteString(fmt.Sprintf("duration_ms: %d\n", duration.Milliseconds()))
	builder.WriteString(fmt.Sprintf("exit_code: %d\n", exitCode))
	builder.WriteString(fmt.Sprintf("stdout_truncated: %t\n", stdoutTruncated))
	builder.WriteString(fmt.Sprintf("stderr_truncated: %t\n", stderrTruncated))
	builder.WriteString("--- stdout ---\n")
	builder.WriteString(stdoutText)
	if !strings.HasSuffix(stdoutText, "\n") {
		builder.WriteString("\n")
	}
	builder.WriteString("--- stderr ---\n")
	builder.WriteString(stderrText)
	if !strings.HasSuffix(stderrText, "\n") {
		builder.WriteString("\n")
	}

	if commandCtx.Err() == context.DeadlineExceeded {
		return builder.String(), fmt.Errorf("command timed out after %s", timeout)
	}
	if runErr != nil {
		return builder.String(), fmt.Errorf("command failed: %w", runErr)
	}

	return builder.String(), nil
}

func truncateOutput(content string, limit int) (string, bool) {
	if len(content) <= limit {
		return content, false
	}
	return content[:limit] + "\n... output truncated ...\n", true
}

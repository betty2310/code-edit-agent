package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

func NewGitDiffTool(workspace *Workspace) ToolDefinition {
	return ToolDefinition{
		Name:        "git_diff",
		Description: "Show a git diff for the current workspace. Use this before committing so you can inspect changed files and draft a good commit message.",
		InputSchema: GitDiffInputSchema,
		Function: func(ctx context.Context, input json.RawMessage) (string, error) {
			return GitDiff(workspace, ctx, input)
		},
	}
}

type GitDiffInput struct {
	Path     string `json:"path,omitempty" jsonschema_description:"Optional file or directory path to diff. Supports workspace-relative or absolute paths inside the workspace."`
	Cached   bool   `json:"cached,omitempty" jsonschema_description:"Set to true to diff staged changes instead of unstaged changes."`
	StatOnly bool   `json:"stat_only,omitempty" jsonschema_description:"Set to true to return a summary stat diff only."`
	BaseRef  string `json:"base_ref,omitempty" jsonschema_description:"Optional git ref to diff against, for example main or HEAD~1."`
}

var GitDiffInputSchema = GenerateSchema[GitDiffInput]()

func GitDiff(workspace *Workspace, ctx context.Context, input json.RawMessage) (string, error) {
	var params GitDiffInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	if err := ensureGitWorkspace(ctx, workspace); err != nil {
		return "", err
	}

	args := []string{"diff"}
	if params.StatOnly {
		args = append(args, "--stat")
	}
	if params.Cached {
		args = append(args, "--cached")
	}
	if strings.TrimSpace(params.BaseRef) != "" {
		args = append(args, params.BaseRef)
	}

	pathLabel := "."
	if strings.TrimSpace(params.Path) != "" {
		_, relPath, err := workspace.Resolve(params.Path)
		if err != nil {
			return "", err
		}
		if relPath != "." {
			args = append(args, "--", relPath)
			pathLabel = relPath
		}
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = workspace.Root
	output, err := cmd.CombinedOutput()
	text, truncated := truncateOutput(string(output), 16000)

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("path: %s\n", pathLabel))
	builder.WriteString(fmt.Sprintf("cached: %t\n", params.Cached))
	builder.WriteString(fmt.Sprintf("stat_only: %t\n", params.StatOnly))
	if strings.TrimSpace(params.BaseRef) != "" {
		builder.WriteString(fmt.Sprintf("base_ref: %s\n", params.BaseRef))
	}
	builder.WriteString(fmt.Sprintf("truncated: %t\n", truncated))
	builder.WriteString("--- diff ---\n")
	builder.WriteString(text)
	if !strings.HasSuffix(text, "\n") {
		builder.WriteString("\n")
	}

	if err != nil {
		return builder.String(), fmt.Errorf("git diff failed: %w", err)
	}

	if strings.TrimSpace(text) == "" {
		return builder.String() + "working tree clean for requested diff\n", nil
	}

	return builder.String(), nil
}

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func NewGitCommitTool(workspace *Workspace) ToolDefinition {
	return ToolDefinition{
		Name:        "git_commit",
		Description: "Stage workspace changes and create a git commit with the provided message. Use git_diff first so the commit message reflects why the changes were made.",
		InputSchema: GitCommitInputSchema,
		Function: func(ctx context.Context, input json.RawMessage) (string, error) {
			return GitCommit(workspace, ctx, input)
		},
	}
}

type GitCommitInput struct {
	Message string `json:"message" jsonschema_description:"A concise commit message that explains why the change was made."`
}

var GitCommitInputSchema = GenerateSchema[GitCommitInput]()

func GitCommit(workspace *Workspace, ctx context.Context, input json.RawMessage) (string, error) {
	var params GitCommitInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}
	if strings.TrimSpace(params.Message) == "" {
		return "", fmt.Errorf("message is required")
	}

	if err := ensureGitWorkspace(ctx, workspace); err != nil {
		return "", err
	}

	statusOutput, err := runGit(ctx, workspace.Root, "status", "--porcelain")
	if err != nil {
		return statusOutput, err
	}
	if strings.TrimSpace(statusOutput) == "" {
		return "", fmt.Errorf("there are no changes to commit")
	}

	if blocked := findSensitiveGitPaths(statusOutput); len(blocked) > 0 {
		return "", fmt.Errorf("refusing to commit sensitive files: %s", strings.Join(blocked, ", "))
	}

	if _, err := runGit(ctx, workspace.Root, "add", "-A", "--", "."); err != nil {
		return "", err
	}

	commitOutput, err := runGit(ctx, workspace.Root, "commit", "-m", params.Message)
	if err != nil {
		return commitOutput, err
	}

	statusAfter, statusErr := runGit(ctx, workspace.Root, "status", "--short")
	if statusErr != nil {
		return commitOutput, statusErr
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("message: %s\n", params.Message))
	builder.WriteString("--- commit ---\n")
	builder.WriteString(commitOutput)
	if !strings.HasSuffix(commitOutput, "\n") {
		builder.WriteString("\n")
	}
	builder.WriteString("--- status ---\n")
	builder.WriteString(statusAfter)
	if !strings.HasSuffix(statusAfter, "\n") {
		builder.WriteString("\n")
	}

	return builder.String(), nil
}

func ensureGitWorkspace(ctx context.Context, workspace *Workspace) error {
	output, err := runGit(ctx, workspace.Root, "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}

	root := strings.TrimSpace(output)
	if root == "" {
		return fmt.Errorf("workspace is not inside a git repository")
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve git root: %w", err)
	}
	if filepath.Clean(absRoot) != filepath.Clean(workspace.Root) {
		return fmt.Errorf("workspace root %s does not match git root %s", workspace.Root, absRoot)
	}

	return nil
}

func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	text := string(output)
	if err != nil {
		if strings.TrimSpace(text) == "" {
			return "", fmt.Errorf("git %s failed: %w", strings.Join(args, " "), err)
		}
		return text, fmt.Errorf("git %s failed: %w", strings.Join(args, " "), err)
	}
	return text, nil
}

func findSensitiveGitPaths(statusOutput string) []string {
	blocked := make([]string, 0)
	seen := make(map[string]struct{})
	for _, line := range strings.Split(statusOutput, "\n") {
		line = strings.TrimSpace(line)
		if len(line) < 4 {
			continue
		}

		path := strings.TrimSpace(line[3:])
		if index := strings.LastIndex(path, " -> "); index >= 0 {
			path = path[index+4:]
		}
		if path == "" {
			continue
		}

		lower := strings.ToLower(path)
		if strings.Contains(lower, ".env") || strings.Contains(lower, "credentials") || strings.Contains(lower, "secret") {
			if _, exists := seen[path]; !exists {
				seen[path] = struct{}{}
				blocked = append(blocked, path)
			}
		}
	}
	return blocked
}

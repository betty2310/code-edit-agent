package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func NewReadFileTool(workspace *Workspace) ToolDefinition {
	return ToolDefinition{
		Name:        "read_file",
		Description: "Read a text file from the workspace. Supports optional line ranges so the agent can inspect large files in smaller slices.",
		InputSchema: ReadFileInputSchema,
		Function: func(ctx context.Context, input json.RawMessage) (string, error) {
			_ = ctx
			return ReadFile(workspace, input)
		},
	}
}

type ReadFileInput struct {
	Path      string `json:"path" jsonschema_description:"The relative path of a file in the workspace."`
	StartLine int    `json:"start_line,omitempty" jsonschema_description:"Optional 1-based first line to include. Defaults to the start of the file."`
	EndLine   int    `json:"end_line,omitempty" jsonschema_description:"Optional 1-based last line to include. Defaults to the end of the file."`
}

var ReadFileInputSchema = GenerateSchema[ReadFileInput]()

func ReadFile(workspace *Workspace, input json.RawMessage) (string, error) {
	var params ReadFileInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}
	if params.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	content, relPath, err := workspace.ReadFile(params.Path)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(content), "\n")
	totalLines := len(lines)
	if totalLines > 0 && lines[totalLines-1] == "" {
		totalLines--
		lines = lines[:totalLines]
	}

	start := params.StartLine
	if start <= 0 {
		start = 1
	}
	end := params.EndLine
	if end <= 0 || end > totalLines {
		end = totalLines
	}
	if start > end && totalLines > 0 {
		return "", fmt.Errorf("start_line must be less than or equal to end_line")
	}

	if totalLines == 0 {
		return fmt.Sprintf("path: %s\nlines: 0\n---\n", relPath), nil
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("path: %s\nlines: %d-%d of %d\n---\n", relPath, start, end, totalLines))
	for i := start - 1; i < end; i++ {
		builder.WriteString(fmt.Sprintf("%d | %s\n", i+1, lines[i]))
	}

	return builder.String(), nil
}

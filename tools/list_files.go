package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func NewListFilesTool(workspace *Workspace) ToolDefinition {
	return ToolDefinition{
		Name:        "list_files",
		Description: "List files and directories in the workspace. Use recursive=true for a project overview. Ignores noisy directories like .git and node_modules.",
		InputSchema: ListFilesInputSchema,
		Function: func(ctx context.Context, input json.RawMessage) (string, error) {
			_ = ctx
			return ListFiles(workspace, input)
		},
	}
}

type ListFilesInput struct {
	Path      string `json:"path,omitempty" jsonschema_description:"Optional relative path to list files from. Defaults to the workspace root."`
	Recursive bool   `json:"recursive,omitempty" jsonschema_description:"Whether to walk subdirectories recursively."`
	Limit     int    `json:"limit,omitempty" jsonschema_description:"Maximum number of entries to return. Defaults to 200."`
}

var ListFilesInputSchema = GenerateSchema[ListFilesInput]()

func ListFiles(workspace *Workspace, input json.RawMessage) (string, error) {
	var params ListFilesInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	entries, relPath, truncated, err := workspace.List(params.Path, params.Recursive, params.Limit)
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("path: %s\nentries: %d\ntruncated: %t\n---\n", relPath, len(entries), truncated))
	for _, entry := range entries {
		builder.WriteString(entry)
		builder.WriteString("\n")
	}

	return builder.String(), nil
}

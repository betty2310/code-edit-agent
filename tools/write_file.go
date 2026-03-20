package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

func NewWriteFileTool(workspace *Workspace) ToolDefinition {
	return ToolDefinition{
		Name:        "write_file",
		Description: "Create a new text file or overwrite an existing text file with the provided content.",
		InputSchema: WriteFileInputSchema,
		Function: func(ctx context.Context, input json.RawMessage) (string, error) {
			_ = ctx
			return WriteFile(workspace, input)
		},
	}
}

type WriteFileInput struct {
	Path    string `json:"path" jsonschema_description:"The relative path of the file to create or overwrite."`
	Content string `json:"content" jsonschema_description:"The full file contents to write."`
}

var WriteFileInputSchema = GenerateSchema[WriteFileInput]()

func WriteFile(workspace *Workspace, input json.RawMessage) (string, error) {
	var params WriteFileInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}
	if params.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	relPath, created, err := workspace.WriteFile(params.Path, []byte(params.Content))
	if err != nil {
		return "", err
	}

	action := "updated"
	if created {
		action = "created"
	}

	return fmt.Sprintf("%s %s (%d bytes)", action, relPath, len(params.Content)), nil
}

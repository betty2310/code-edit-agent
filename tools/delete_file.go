package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

func NewDeleteFileTool(workspace *Workspace) ToolDefinition {
	return ToolDefinition{
		Name:        "delete_file",
		Description: "Delete a single file from the workspace.",
		InputSchema: DeleteFileInputSchema,
		Function: func(ctx context.Context, input json.RawMessage) (string, error) {
			_ = ctx
			return DeleteFile(workspace, input)
		},
	}
}

type DeleteFileInput struct {
	Path string `json:"path" jsonschema_description:"The relative path of the file to delete."`
}

var DeleteFileInputSchema = GenerateSchema[DeleteFileInput]()

func DeleteFile(workspace *Workspace, input json.RawMessage) (string, error) {
	var params DeleteFileInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}
	if params.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	relPath, err := workspace.DeleteFile(params.Path)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("deleted %s", relPath), nil
}

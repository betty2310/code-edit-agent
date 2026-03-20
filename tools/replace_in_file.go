package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

func NewReplaceInFileTool(workspace *Workspace) ToolDefinition {
	return ToolDefinition{
		Name:        "replace_in_file",
		Description: "Replace exact text in a file. By default the old text must match exactly once; set replace_all=true to replace every match.",
		InputSchema: ReplaceInFileInputSchema,
		Function: func(ctx context.Context, input json.RawMessage) (string, error) {
			_ = ctx
			return ReplaceInFile(workspace, input)
		},
	}
}

type ReplaceInFileInput struct {
	Path       string `json:"path" jsonschema_description:"The relative path of the file to edit."`
	OldStr     string `json:"old_str" jsonschema_description:"The exact text to replace."`
	NewStr     string `json:"new_str" jsonschema_description:"The text that should replace old_str."`
	ReplaceAll bool   `json:"replace_all,omitempty" jsonschema_description:"Set to true to replace every exact match. Defaults to false."`
}

var ReplaceInFileInputSchema = GenerateSchema[ReplaceInFileInput]()

func ReplaceInFile(workspace *Workspace, input json.RawMessage) (string, error) {
	var params ReplaceInFileInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}
	if params.Path == "" {
		return "", fmt.Errorf("path is required")
	}
	if params.OldStr == params.NewStr {
		return "", fmt.Errorf("old_str and new_str must be different")
	}
	if params.OldStr == "" {
		return "", fmt.Errorf("old_str is required")
	}

	relPath, replacements, err := workspace.ReplaceInFile(params.Path, params.OldStr, params.NewStr, params.ReplaceAll)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("updated %s (%d replacement(s))", relPath, replacements), nil
}

package tools

import "time"

func NewDefinitions(workspace *Workspace, commandTimeout time.Duration) []ToolDefinition {
	return []ToolDefinition{
		NewListFilesTool(workspace),
		NewGrepTextTool(workspace),
		NewReadFileTool(workspace),
		NewWriteFileTool(workspace),
		NewReplaceInFileTool(workspace),
		NewDeleteFileTool(workspace),
		NewGitDiffTool(workspace),
		NewGitCommitTool(workspace),
		NewExecuteCommandTool(workspace, commandTimeout),
	}
}

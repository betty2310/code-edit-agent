package main

import (
	"fmt"
	"os"

	"github.com/betty/agent/lib"
	"github.com/betty/agent/tools"
	"github.com/betty/agent/ui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	config, err := lib.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	workspace, err := tools.NewWorkspace(config.Root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	client, model, err := lib.NewClientFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	toolDefinitions := tools.NewDefinitions(workspace, config.CommandTimeout)

	agent := lib.NewAgent(
		&client,
		model,
		toolDefinitions,
		config.SystemPrompt,
		config.MaxTokens,
		config.RequestTimeout,
	)

	session := lib.NewSession(agent)
	program := tea.NewProgram(
		ui.NewModel(session, workspace.Root, string(model), toolNames(toolDefinitions)),
		tea.WithMouseCellMotion(),
	)

	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

func toolNames(toolDefinitions []tools.ToolDefinition) []string {
	names := make([]string, 0, len(toolDefinitions))
	for _, tool := range toolDefinitions {
		names = append(names, tool.Name)
	}
	return names
}

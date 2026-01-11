package main

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/betty/agent/lib"
	"github.com/betty/agent/tools"
)

func main() {
	client := anthropic.NewClient(option.WithAPIKey("hmmmm"))

	scanner := bufio.NewScanner(os.Stdin)
	getUserMessage := func() (string, bool) {
		if !scanner.Scan() {
			return "", false
		}
		if scanner.Text() == "exit" || scanner.Text() == "quit" || scanner.Text() == "\n" {
			return "", false
		}
		return scanner.Text(), true
	}
	tools := []tools.ToolDefinition{
		tools.ReadFileDefinition,
		tools.ListFilesDefinition,
		tools.EditFileDefinition,
	}

	agent := lib.NewAgent(&client, getUserMessage, tools)

	err := agent.Run(context.TODO())

	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
	}
}

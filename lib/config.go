package lib

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const openRouterBaseURL = "https://openrouter.ai/api/"

var (
	defaultAnthropicModel  = anthropic.ModelClaudeHaiku4_5
	defaultOpenRouterModel = anthropic.Model("anthropic/claude-haiku-4.5")
)

type Config struct {
	Root           string
	SystemPrompt   string
	MaxTokens      int64
	RequestTimeout time.Duration
	CommandTimeout time.Duration
}

func LoadConfig() (Config, error) {
	root, err := os.Getwd()
	if err != nil {
		return Config{}, fmt.Errorf("resolve working directory: %w", err)
	}

	return Config{
		Root:           root,
		SystemPrompt:   defaultSystemPrompt(root),
		MaxTokens:      2048,
		RequestTimeout: 2 * time.Minute,
		CommandTimeout: 30 * time.Second,
	}, nil
}

func NewClientFromEnv() (anthropic.Client, anthropic.Model, error) {
	openRouterAPIKey := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
	anthropicAPIKey := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	model := strings.TrimSpace(os.Getenv("ANTHROPIC_MODEL"))

	var opts []option.RequestOption

	switch {
	case openRouterAPIKey != "":
		opts = append(opts,
			option.WithBaseURL(openRouterBaseURL),
			option.WithAuthToken(openRouterAPIKey),
		)

		if referer := strings.TrimSpace(os.Getenv("OPENROUTER_HTTP_REFERER")); referer != "" {
			opts = append(opts, option.WithHeader("HTTP-Referer", referer))
		}

		if title := strings.TrimSpace(os.Getenv("OPENROUTER_APP_TITLE")); title != "" {
			opts = append(opts, option.WithHeader("X-OpenRouter-Title", title))
		}

		if model == "" {
			model = string(defaultOpenRouterModel)
		}
	case anthropicAPIKey != "":
		opts = append(opts, option.WithAPIKey(anthropicAPIKey))

		if model == "" {
			model = string(defaultAnthropicModel)
		}
	default:
		return anthropic.Client{}, "", fmt.Errorf("set OPENROUTER_API_KEY or ANTHROPIC_API_KEY before running the agent")
	}

	return anthropic.NewClient(opts...), anthropic.Model(model), nil
}

func defaultSystemPrompt(root string) string {
	return fmt.Sprintf(`You are a local coding agent working inside the workspace at %s.

Your goal is to inspect the project, make file changes, and verify them with local commands when appropriate.

Tool rules:
- Use list_files and grep_text to understand the project before editing.
- Use read_file to inspect specific files.
- Use write_file to create or fully rewrite files.
- Use replace_in_file for focused edits when exact replacement is safer.
- Use git_diff to review pending changes before writing a commit message.
- Use git_commit only after you have reviewed the diff and the user asked for a commit.
- Use execute_command to run builds, tests, or other local verification commands.
- Prefer small, targeted changes and verify with commands after edits when feasible.

Response rules:
- Be concise and practical.
- Explain what you changed and what you verified.
- If a command fails, use the output to debug and continue when possible.`, root)
}

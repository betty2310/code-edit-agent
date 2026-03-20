# Code Edit Agent

A Codex-style coding agent for the terminal, built in Go with the [Anthropic SDK](https://github.com/anthropics/anthropic-sdk-go) and [Bubble Tea](https://github.com/charmbracelet/bubbletea).

It gives you a Codex-style terminal UI, project-aware file tools, git-aware workflow helpers, and local command execution so the agent can inspect a repo, edit files, verify changes with builds or tests, and create commits.

## Features

- **Bubble Tea TUI** — full-screen chat, activity timeline, prompt box, and status bar
- **Project understanding tools** — recursive file listing, regex search, ranged file reads
- **File CRUD** — create, overwrite, replace exact text, and delete files
- **Git workflow** — inspect diffs and create commits from inside the agent loop
- **Local verification** — run commands like `go build`, `go test`, `npm test`, or project-specific scripts
- **Workspace safety** — tools stay inside the current workspace root, accept workspace-relative or in-workspace absolute paths, and ignore noisy directories like `.git` and `node_modules`

## Tools

| Tool | Description |
|------|-------------|
| `list_files` | List files or directories, optionally recursively |
| `grep_text` | Search project files with a regular expression |
| `read_file` | Read a file, optionally by line range |
| `write_file` | Create or overwrite a file |
| `replace_in_file` | Replace exact text in a file |
| `delete_file` | Delete a single file |
| `git_diff` | Show pending git changes for the repo or a path |
| `git_commit` | Stage changes and create a commit with a provided message |
| `execute_command` | Run a local shell command in the workspace |

## Prerequisites

- Go 1.25+
- An [Anthropic API key](https://console.anthropic.com/) or an [OpenRouter API key](https://openrouter.ai/)

## Getting Started

```bash
# Clone the repo
git clone https://github.com/betty2310/code-edit-agent.git
cd code-edit-agent

# Install dependencies
go mod download

# Choose one provider
export OPENROUTER_API_KEY="your-openrouter-key"
# or: export ANTHROPIC_API_KEY="your-anthropic-key"

# Run the agent
go run main.go
```

> **Defaults:** The agent uses the cheapest Anthropic Haiku model with tool support by default.
> - Anthropic API: `claude-3-haiku-20240307`
> - OpenRouter API: `anthropic/claude-3-haiku`
>
> Override the model with `ANTHROPIC_MODEL` if needed.

### Optional OpenRouter headers

If you use OpenRouter, you can also set these optional headers:

```bash
export OPENROUTER_HTTP_REFERER="https://your-app.example"
export OPENROUTER_APP_TITLE="Code Edit Agent"
```

## Usage

Launch the app and use the prompt pane to ask for changes like:

- `inspect this Go project and explain the main packages`
- `update the README and then run go build`
- `find the tool registry and add a new file delete tool`
- `review the diff and create a concise commit message`

### Key bindings

- `ctrl+s` — send prompt
- `ctrl+l` — clear transcript
- `ctrl+r` — reset session history
- `ctrl+c` — quit

### Local slash commands

- `/help`
- `/tools`
- `/clear`
- `/reset`

## Project Structure

```
├── main.go          # Entrypoint — loads config, workspace, tools, agent, and TUI
├── lib/
	│   ├── agent.go     # Agent session loop, model calls, and tool dispatch
	│   └── config.go    # Environment-based client and runtime config
	├── ui/
	│   └── model.go     # Bubble Tea interface model and rendering
└── tools/
	    ├── tool.go             # ToolDefinition type and schema generation helper
	    ├── workspace.go        # Workspace boundary and safe file helpers
	    ├── registry.go         # Tool registration
	    ├── read_file.go        # File reading tool
	    ├── list_files.go       # File and directory listing tool
	    ├── grep_text.go        # Regex project search tool
	    ├── write_file.go       # Create/overwrite tool
	    ├── replace_in_file.go  # Exact text replacement tool
	    ├── delete_file.go      # File deletion tool
	    ├── git_diff.go         # Git diff inspection tool
	    ├── git_commit.go       # Git commit creation tool
	    └── execute_command.go  # Local command execution tool
```

## License

MIT

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func NewGrepTextTool(workspace *Workspace) ToolDefinition {
	return ToolDefinition{
		Name:        "grep_text",
		Description: "Search file contents with a regular expression across the workspace. Use this to locate code, symbols, or text before editing.",
		InputSchema: GrepTextInputSchema,
		Function: func(ctx context.Context, input json.RawMessage) (string, error) {
			_ = ctx
			return GrepText(workspace, input)
		},
	}
}

type GrepTextInput struct {
	Pattern       string `json:"pattern" jsonschema_description:"A regular expression to search for."`
	Path          string `json:"path,omitempty" jsonschema_description:"Optional relative directory or file path to search within. Defaults to the workspace root."`
	Limit         int    `json:"limit,omitempty" jsonschema_description:"Maximum number of matches to return. Defaults to 50."`
	CaseSensitive bool   `json:"case_sensitive,omitempty" jsonschema_description:"Whether the regular expression should be case-sensitive. Defaults to false."`
}

var GrepTextInputSchema = GenerateSchema[GrepTextInput]()

func GrepText(workspace *Workspace, input json.RawMessage) (string, error) {
	var params GrepTextInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}
	if params.Pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}
	if params.Limit <= 0 {
		params.Limit = 50
	}

	pattern := params.Pattern
	if !params.CaseSensitive {
		pattern = "(?i)" + pattern
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regular expression: %w", err)
	}

	absPath, relPath, err := workspace.Resolve(params.Path)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", err
	}

	matches := make([]string, 0, min(params.Limit, 16))
	appendMatches := func(path string) error {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if !utf8Likely(content) {
			return nil
		}

		fileRel, err := filepath.Rel(workspace.Root, path)
		if err != nil {
			return err
		}

		lines := strings.Split(string(content), "\n")
		for index, line := range lines {
			if re.MatchString(line) {
				matches = append(matches, fmt.Sprintf("%s:%d: %s", filepath.ToSlash(fileRel), index+1, strings.TrimSpace(line)))
				if len(matches) >= params.Limit {
					return fs.SkipAll
				}
			}
		}

		return nil
	}

	if !info.IsDir() {
		if err := appendMatches(absPath); err != nil && err != fs.SkipAll {
			return "", err
		}
	} else {
		walkErr := filepath.WalkDir(absPath, func(currentPath string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}

			currentRel, err := filepath.Rel(workspace.Root, currentPath)
			if err != nil {
				return err
			}
			if currentRel != "." && workspace.shouldIgnore(currentRel) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				return nil
			}

			return appendMatches(currentPath)
		})
		if walkErr != nil && walkErr != fs.SkipAll {
			return "", walkErr
		}
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("path: %s\nmatches: %d\n---\n", relPath, len(matches)))
	for _, match := range matches {
		builder.WriteString(match)
		builder.WriteString("\n")
	}

	return builder.String(), nil
}

func utf8Likely(content []byte) bool {
	for _, b := range content {
		if b == 0 {
			return false
		}
	}
	return true
}

package tools

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var ignoredDirectories = map[string]struct{}{
	".git":         {},
	".hg":          {},
	".svn":         {},
	"node_modules": {},
	"dist":         {},
	"build":        {},
	".idea":        {},
	".vscode":      {},
}

type Workspace struct {
	Root string
}

func NewWorkspace(root string) (*Workspace, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root: %w", err)
	}

	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("stat workspace root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("workspace root must be a directory")
	}

	return &Workspace{Root: absRoot}, nil
}

func (w *Workspace) Resolve(path string) (string, string, error) {
	if path == "" {
		path = "."
	}

	cleanPath := filepath.Clean(path)
	var absPath string
	if filepath.IsAbs(cleanPath) {
		absPath = cleanPath
	} else {
		absPath = filepath.Join(w.Root, cleanPath)
	}
	absPath = filepath.Clean(absPath)

	relPath, err := filepath.Rel(w.Root, absPath)
	if err != nil {
		return "", "", fmt.Errorf("resolve relative path: %w", err)
	}

	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("path %q escapes the workspace root", path)
	}

	if relPath == "." {
		return absPath, ".", nil
	}

	return absPath, filepath.ToSlash(relPath), nil
}

func (w *Workspace) shouldIgnore(relPath string) bool {
	if relPath == "." || relPath == "" {
		return false
	}

	parts := strings.Split(filepath.ToSlash(relPath), "/")
	for _, part := range parts {
		if _, ignored := ignoredDirectories[part]; ignored {
			return true
		}
	}

	return false
}

func (w *Workspace) ReadFile(path string) ([]byte, string, error) {
	absPath, relPath, err := w.Resolve(path)
	if err != nil {
		return nil, "", err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, "", err
	}
	if info.IsDir() {
		return nil, "", fmt.Errorf("%s is a directory", relPath)
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, "", err
	}

	return content, relPath, nil
}

func (w *Workspace) WriteFile(path string, content []byte) (string, bool, error) {
	absPath, relPath, err := w.Resolve(path)
	if err != nil {
		return "", false, err
	}

	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return "", false, fmt.Errorf("create parent directories: %w", err)
	}

	_, statErr := os.Stat(absPath)
	created := os.IsNotExist(statErr)
	if statErr != nil && !created {
		return "", false, statErr
	}

	if err := writeAtomically(absPath, content, 0o644); err != nil {
		return "", false, err
	}

	return relPath, created, nil
}

func (w *Workspace) DeleteFile(path string) (string, error) {
	absPath, relPath, err := w.Resolve(path)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory; delete_file only removes files", relPath)
	}

	if err := os.Remove(absPath); err != nil {
		return "", err
	}

	return relPath, nil
}

func (w *Workspace) ReplaceInFile(path, oldStr, newStr string, replaceAll bool) (string, int, error) {
	content, relPath, err := w.ReadFile(path)
	if err != nil {
		return "", 0, err
	}

	oldContent := string(content)
	matches := strings.Count(oldContent, oldStr)
	if matches == 0 {
		return "", 0, fmt.Errorf("target text not found in %s", relPath)
	}
	if !replaceAll && matches > 1 {
		return "", 0, fmt.Errorf("target text matched %d times in %s; set replace_all=true or provide more specific text", matches, relPath)
	}

	replacementCount := 1
	if replaceAll {
		replacementCount = -1
	}

	newContent := strings.Replace(oldContent, oldStr, newStr, replacementCount)
	actualReplacements := 1
	if replaceAll {
		actualReplacements = matches
	}

	absPath, _, err := w.Resolve(path)
	if err != nil {
		return "", 0, err
	}

	if err := writeAtomically(absPath, []byte(newContent), 0o644); err != nil {
		return "", 0, err
	}

	return relPath, actualReplacements, nil
}

func (w *Workspace) List(path string, recursive bool, limit int) ([]string, string, bool, error) {
	if limit <= 0 {
		limit = 200
	}

	absPath, relPath, err := w.Resolve(path)
	if err != nil {
		return nil, "", false, err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, "", false, err
	}

	entries := make([]string, 0, min(limit, 32))
	truncated := false

	appendEntry := func(entry string) bool {
		entries = append(entries, filepath.ToSlash(entry))
		if len(entries) >= limit {
			truncated = true
			return false
		}
		return true
	}

	if !info.IsDir() {
		return []string{relPath}, relPath, false, nil
	}

	if recursive {
		walkErr := filepath.WalkDir(absPath, func(currentPath string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}

			currentRel, err := filepath.Rel(w.Root, currentPath)
			if err != nil {
				return err
			}

			if currentRel == "." {
				return nil
			}

			if w.shouldIgnore(currentRel) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			entry := currentRel
			if d.IsDir() {
				entry += "/"
			}

			if !appendEntry(entry) {
				return filepath.SkipAll
			}

			return nil
		})
		if walkErr != nil && walkErr != filepath.SkipAll {
			return nil, "", false, walkErr
		}

		return entries, relPath, truncated, nil
	}

	dirEntries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, "", false, err
	}

	for _, entry := range dirEntries {
		entryRel := entry.Name()
		if relPath != "." {
			entryRel = filepath.Join(relPath, entryRel)
		}
		if w.shouldIgnore(entryRel) {
			continue
		}
		if entry.IsDir() {
			entryRel += "/"
		}
		if !appendEntry(entryRel) {
			break
		}
	}

	sort.Strings(entries)
	return entries, relPath, truncated, nil
}

func writeAtomically(path string, content []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tempFile, err := os.CreateTemp(dir, ".agent-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tempName := tempFile.Name()

	defer func() {
		_ = os.Remove(tempName)
	}()

	if _, err := tempFile.Write(content); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tempFile.Chmod(perm); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("replace file: %w", err)
	}

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

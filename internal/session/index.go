package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// RenameSession appends custom-title and agent-name events to rename a session.
// custom-title controls the session title in listings and /resume picker.
// agent-name controls the green label in Claude Code's input box.
func RenameSession(meta *SessionMeta, newTitle string) error {
	f, err := os.OpenFile(meta.FilePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.Encode(map[string]any{
		"type":        "custom-title",
		"customTitle": newTitle,
		"sessionId":   meta.ID,
	})
	return enc.Encode(map[string]any{
		"type":      "agent-name",
		"agentName": newTitle,
		"sessionId": meta.ID,
	})
}

// RebuildIndex rebuilds sessions-index.json for a project directory.
func RebuildIndex(projDir string) (int, error) {
	jsonlFiles, err := filepath.Glob(filepath.Join(projDir, "*.jsonl"))
	if err != nil {
		return 0, err
	}

	// Decode original path from dir name
	dirName := filepath.Base(projDir)
	originalPath := decodeProjectPath(dirName)

	var entries []IndexEntry
	for _, fp := range jsonlFiles {
		meta, err := quickExtract(fp)
		if err != nil {
			continue
		}
		info, _ := os.Stat(fp)

		entry := IndexEntry{
			SessionID:   meta.ID,
			FullPath:    fp,
			FileMtime:   info.ModTime().UnixMilli(),
			FirstPrompt: meta.Title,
			Summary:     meta.Title,
			MsgCount:    meta.Messages,
			Created:     meta.Created.Format("2006-01-02T15:04:05.000Z"),
			Modified:    meta.Modified.Format("2006-01-02T15:04:05.000Z"),
			GitBranch:   meta.Branch,
			ProjectPath: originalPath,
			IsSidechain: false,
		}
		entries = append(entries, entry)
	}

	idx := IndexFile{
		Version:      1,
		Entries:      entries,
		OriginalPath: originalPath,
	}

	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return 0, err
	}

	return len(entries), os.WriteFile(filepath.Join(projDir, "sessions-index.json"), data, 0644)
}

// quickExtract does a minimal metadata extraction for reindexing.
func quickExtract(filePath string) (*SessionMeta, error) {
	s := NewScanner(filepath.Dir(filepath.Dir(filePath)))
	activeIDs := make(map[string]bool)
	return s.extractMeta(filePath, filepath.Base(filepath.Dir(filePath)), activeIDs)
}

// DeleteSession removes a session JSONL and all associated artifacts.
func DeleteSession(claudeDir string, meta *SessionMeta) ([]string, error) {
	var deleted []string

	// 1. JSONL file
	if err := os.Remove(meta.FilePath); err != nil && !os.IsNotExist(err) {
		return deleted, err
	}
	deleted = append(deleted, meta.FilePath)

	// 2. Subagent directory
	subDir := strings.TrimSuffix(meta.FilePath, ".jsonl")
	if info, err := os.Stat(subDir); err == nil && info.IsDir() {
		os.RemoveAll(subDir)
		deleted = append(deleted, subDir+"/")
	}

	// 3. Task directory
	taskDir := filepath.Join(claudeDir, "tasks", meta.ID)
	if info, err := os.Stat(taskDir); err == nil && info.IsDir() {
		os.RemoveAll(taskDir)
		deleted = append(deleted, taskDir+"/")
	}

	// 4. Session-env directory
	envDir := filepath.Join(claudeDir, "session-env", meta.ID)
	if info, err := os.Stat(envDir); err == nil && info.IsDir() {
		os.RemoveAll(envDir)
		deleted = append(deleted, envDir+"/")
	}

	// 5. Update sessions-index.json if it exists
	projDir := filepath.Dir(meta.FilePath)
	idxPath := filepath.Join(projDir, "sessions-index.json")
	if data, err := os.ReadFile(idxPath); err == nil {
		var idx IndexFile
		if json.Unmarshal(data, &idx) == nil {
			var filtered []IndexEntry
			for _, e := range idx.Entries {
				if e.SessionID != meta.ID {
					filtered = append(filtered, e)
				}
			}
			idx.Entries = filtered
			if updated, err := json.MarshalIndent(idx, "", "  "); err == nil {
				os.WriteFile(idxPath, updated, 0644)
			}
		}
	}

	return deleted, nil
}

// MoveSession moves a session JSONL (and subagent dir) from one project to another.
func MoveSession(claudeDir string, meta *SessionMeta, destProject string) (string, error) {
	// Encode destination project path to directory name
	destDirName := encodeProjectPath(destProject)
	destDir := filepath.Join(claudeDir, "projects", destDirName)

	// Create destination project dir if needed
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", err
	}

	destPath := filepath.Join(destDir, filepath.Base(meta.FilePath))

	// Move JSONL file
	if err := os.Rename(meta.FilePath, destPath); err != nil {
		return "", err
	}

	// Move subagent directory if it exists
	subDir := strings.TrimSuffix(meta.FilePath, ".jsonl")
	destSubDir := strings.TrimSuffix(destPath, ".jsonl")
	if info, err := os.Stat(subDir); err == nil && info.IsDir() {
		os.Rename(subDir, destSubDir)
	}

	// Remove from source index
	srcDir := filepath.Dir(meta.FilePath)
	removeFromIndex(srcDir, meta.ID)

	// Invalidate cache
	os.Remove(filepath.Join(claudeDir, "csm-cache.json"))

	return destPath, nil
}

func encodeProjectPath(path string) string {
	// /home/ahmed/code/cercli -> -home-ahmed-code-cercli
	path = strings.TrimPrefix(path, "/")
	return "-" + strings.ReplaceAll(path, "/", "-")
}

func removeFromIndex(projDir, sessionID string) {
	idxPath := filepath.Join(projDir, "sessions-index.json")
	data, err := os.ReadFile(idxPath)
	if err != nil {
		return
	}
	var idx IndexFile
	if json.Unmarshal(data, &idx) != nil {
		return
	}
	var filtered []IndexEntry
	for _, e := range idx.Entries {
		if e.SessionID != sessionID {
			filtered = append(filtered, e)
		}
	}
	idx.Entries = filtered
	if updated, err := json.MarshalIndent(idx, "", "  "); err == nil {
		os.WriteFile(idxPath, updated, 0644)
	}
}

// ListProjects returns all project paths that have sessions.
func ListProjects(claudeDir string) ([]string, error) {
	projectsDir := filepath.Join(claudeDir, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, err
	}
	var projects []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if strings.Contains(e.Name(), "claude-mem-observer") {
			continue
		}
		// Check it has JSONL files
		jsonlFiles, _ := filepath.Glob(filepath.Join(projectsDir, e.Name(), "*.jsonl"))
		if len(jsonlFiles) > 0 {
			projects = append(projects, decodeProjectPath(e.Name()))
		}
	}
	return projects, nil
}

// CleanOrphanedArtifacts finds and removes session-env and task dirs with no matching JSONL.
func CleanOrphanedArtifacts(claudeDir string, dryRun bool) ([]string, error) {
	// Build set of known session IDs from JSONL files
	knownIDs := make(map[string]bool)
	projectsDir := filepath.Join(claudeDir, "projects")
	projEntries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, err
	}
	for _, pe := range projEntries {
		if !pe.IsDir() {
			continue
		}
		jsonlFiles, _ := filepath.Glob(filepath.Join(projectsDir, pe.Name(), "*.jsonl"))
		for _, fp := range jsonlFiles {
			base := filepath.Base(fp)
			id := strings.TrimSuffix(base, ".jsonl")
			knownIDs[id] = true
		}
	}

	var orphans []string

	// Check session-env
	envDir := filepath.Join(claudeDir, "session-env")
	if entries, err := os.ReadDir(envDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if !knownIDs[e.Name()] {
				path := filepath.Join(envDir, e.Name())
				orphans = append(orphans, path)
				if !dryRun {
					os.RemoveAll(path)
				}
			}
		}
	}

	// Check tasks
	taskDir := filepath.Join(claudeDir, "tasks")
	if entries, err := os.ReadDir(taskDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if !knownIDs[e.Name()] {
				path := filepath.Join(taskDir, e.Name())
				orphans = append(orphans, path)
				if !dryRun {
					os.RemoveAll(path)
				}
			}
		}
	}

	return orphans, nil
}

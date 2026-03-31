package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ash0x0/csm/internal/testutil"
	"github.com/google/uuid"
)

func TestRenameSession(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)
	projDir := createProjectDir(t, claudeDir, "/home/user/myproject")
	sessionID := uuid.New().String()

	events := testutil.BuildSimpleSession(sessionID, "Original Title", "main", 1, "2026-03-20T10:00:00Z")
	filePath := testutil.WriteSession(t, projDir, sessionID, events)

	meta := &SessionMeta{
		ID:       sessionID,
		FilePath: filePath,
	}

	if err := RenameSession(meta, "New Title"); err != nil {
		t.Fatalf("RenameSession: %v", err)
	}

	// Read back and verify custom-title + agent-name events appended
	rawEvents, err := ReadRawEvents(filePath)
	if err != nil {
		t.Fatalf("ReadRawEvents: %v", err)
	}

	n := len(rawEvents)
	titleEv := rawEvents[n-2]
	nameEv := rawEvents[n-1]

	if titleEv["type"] != "custom-title" {
		t.Errorf("second-to-last event type = %q, want %q", titleEv["type"], "custom-title")
	}
	if titleEv["customTitle"] != "New Title" {
		t.Errorf("customTitle = %q, want %q", titleEv["customTitle"], "New Title")
	}
	if nameEv["type"] != "agent-name" {
		t.Errorf("last event type = %q, want %q", nameEv["type"], "agent-name")
	}
	if nameEv["agentName"] != "New Title" {
		t.Errorf("agentName = %q, want %q", nameEv["agentName"], "New Title")
	}
}

func TestRebuildIndex(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)
	projDir := createProjectDir(t, claudeDir, "/home/user/myproject")

	id1 := uuid.New().String()
	id2 := uuid.New().String()
	events1 := testutil.BuildSimpleSession(id1, "Session One", "main", 2, "2026-03-20T10:00:00Z")
	events2 := testutil.BuildSimpleSession(id2, "Session Two", "dev", 1, "2026-03-21T12:00:00Z")
	testutil.WriteSession(t, projDir, id1, events1)
	testutil.WriteSession(t, projDir, id2, events2)

	count, err := RebuildIndex(projDir)
	if err != nil {
		t.Fatalf("RebuildIndex: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}

	// Read and verify sessions-index.json
	idxPath := filepath.Join(projDir, "sessions-index.json")
	data, err := os.ReadFile(idxPath)
	if err != nil {
		t.Fatalf("reading index: %v", err)
	}

	var idx IndexFile
	if err := json.Unmarshal(data, &idx); err != nil {
		t.Fatalf("unmarshal index: %v", err)
	}

	if idx.Version != 1 {
		t.Errorf("version = %d, want 1", idx.Version)
	}
	if len(idx.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(idx.Entries))
	}

	ids := map[string]bool{}
	for _, e := range idx.Entries {
		ids[e.SessionID] = true
	}
	if !ids[id1] {
		t.Errorf("index missing session %s", id1)
	}
	if !ids[id2] {
		t.Errorf("index missing session %s", id2)
	}
}

func TestDeleteSession(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)
	projDir := createProjectDir(t, claudeDir, "/home/user/myproject")
	sessionID := uuid.New().String()

	events := testutil.BuildSimpleSession(sessionID, "Delete Me", "main", 1, "2026-03-20T10:00:00Z")
	filePath := testutil.WriteSession(t, projDir, sessionID, events)

	// Create subagent directory (same name as JSONL minus extension)
	subDir := filepath.Join(projDir, sessionID)
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "sub.jsonl"), []byte("{}"), 0644)

	// Create task directory
	taskDir := filepath.Join(claudeDir, "tasks", sessionID)
	os.MkdirAll(taskDir, 0755)
	os.WriteFile(filepath.Join(taskDir, "task.json"), []byte("{}"), 0644)

	// Create session-env directory
	envDir := filepath.Join(claudeDir, "session-env", sessionID)
	os.MkdirAll(envDir, 0755)
	os.WriteFile(filepath.Join(envDir, "env.json"), []byte("{}"), 0644)

	meta := &SessionMeta{
		ID:       sessionID,
		FilePath: filePath,
	}

	deleted, err := DeleteSession(claudeDir, meta)
	if err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	// Should have deleted 4 items: JSONL, subagent dir, task dir, session-env dir
	if len(deleted) != 4 {
		t.Errorf("deleted count = %d, want 4; items: %v", len(deleted), deleted)
	}

	// Verify all are gone
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("JSONL file still exists")
	}
	if _, err := os.Stat(subDir); !os.IsNotExist(err) {
		t.Error("subagent dir still exists")
	}
	if _, err := os.Stat(taskDir); !os.IsNotExist(err) {
		t.Error("task dir still exists")
	}
	if _, err := os.Stat(envDir); !os.IsNotExist(err) {
		t.Error("session-env dir still exists")
	}
}

func TestDeleteSessionUpdatesIndex(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)
	projDir := createProjectDir(t, claudeDir, "/home/user/myproject")

	id1 := uuid.New().String()
	id2 := uuid.New().String()
	events1 := testutil.BuildSimpleSession(id1, "Keep This", "main", 1, "2026-03-20T10:00:00Z")
	events2 := testutil.BuildSimpleSession(id2, "Delete This", "main", 1, "2026-03-21T12:00:00Z")
	testutil.WriteSession(t, projDir, id1, events1)
	filePath2 := testutil.WriteSession(t, projDir, id2, events2)

	// Build the index first
	if _, err := RebuildIndex(projDir); err != nil {
		t.Fatalf("RebuildIndex: %v", err)
	}

	meta := &SessionMeta{
		ID:       id2,
		FilePath: filePath2,
	}

	if _, err := DeleteSession(claudeDir, meta); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	// Read index and verify id2 is gone but id1 remains
	idxPath := filepath.Join(projDir, "sessions-index.json")
	data, err := os.ReadFile(idxPath)
	if err != nil {
		t.Fatalf("reading index: %v", err)
	}

	var idx IndexFile
	if err := json.Unmarshal(data, &idx); err != nil {
		t.Fatalf("unmarshal index: %v", err)
	}

	if len(idx.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(idx.Entries))
	}
	if idx.Entries[0].SessionID != id1 {
		t.Errorf("remaining session = %s, want %s", idx.Entries[0].SessionID, id1)
	}
}

func TestMoveSession(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)
	projDirA := createProjectDir(t, claudeDir, "/home/user/projectA")
	_ = createProjectDir(t, claudeDir, "/home/user/projectB")

	sessionID := uuid.New().String()
	events := testutil.BuildSimpleSession(sessionID, "Move Me", "main", 1, "2026-03-20T10:00:00Z")
	filePath := testutil.WriteSession(t, projDirA, sessionID, events)

	meta := &SessionMeta{
		ID:       sessionID,
		FilePath: filePath,
	}

	destPath, err := MoveSession(claudeDir, meta, "/home/user/projectB")
	if err != nil {
		t.Fatalf("MoveSession: %v", err)
	}

	// Source should not exist
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("source file still exists after move")
	}

	// Destination should exist
	if _, err := os.Stat(destPath); err != nil {
		t.Errorf("dest file does not exist: %v", err)
	}

	// Verify content is intact
	rawEvents, err := ReadRawEvents(destPath)
	if err != nil {
		t.Fatalf("ReadRawEvents: %v", err)
	}
	if len(rawEvents) != len(events) {
		t.Errorf("event count = %d, want %d", len(rawEvents), len(events))
	}
}

func TestMoveSessionCreatesDestDir(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)
	projDirA := createProjectDir(t, claudeDir, "/home/user/projectA")

	sessionID := uuid.New().String()
	events := testutil.BuildSimpleSession(sessionID, "Move Me", "main", 1, "2026-03-20T10:00:00Z")
	filePath := testutil.WriteSession(t, projDirA, sessionID, events)

	meta := &SessionMeta{
		ID:       sessionID,
		FilePath: filePath,
	}

	// /home/user/newproject does not exist yet as a project dir
	destPath, err := MoveSession(claudeDir, meta, "/home/user/newproject")
	if err != nil {
		t.Fatalf("MoveSession: %v", err)
	}

	// Destination directory should have been created
	destDir := filepath.Dir(destPath)
	info, err := os.Stat(destDir)
	if err != nil {
		t.Fatalf("dest dir does not exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("dest path is not a directory")
	}

	// File should exist at destination
	if _, err := os.Stat(destPath); err != nil {
		t.Errorf("dest file does not exist: %v", err)
	}
}

func TestCleanOrphanedArtifacts(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)
	projDir := createProjectDir(t, claudeDir, "/home/user/myproject")

	// Create one real session
	realID := uuid.New().String()
	events := testutil.BuildSimpleSession(realID, "Real Session", "main", 1, "2026-03-20T10:00:00Z")
	testutil.WriteSession(t, projDir, realID, events)

	// Create matching artifacts for real session (should NOT be cleaned)
	os.MkdirAll(filepath.Join(claudeDir, "session-env", realID), 0755)
	os.MkdirAll(filepath.Join(claudeDir, "tasks", realID), 0755)

	// Create orphaned artifacts (no matching JSONL)
	orphanID1 := uuid.New().String()
	orphanID2 := uuid.New().String()
	orphanEnv := filepath.Join(claudeDir, "session-env", orphanID1)
	orphanTask := filepath.Join(claudeDir, "tasks", orphanID2)
	os.MkdirAll(orphanEnv, 0755)
	os.MkdirAll(orphanTask, 0755)

	orphans, err := CleanOrphanedArtifacts(claudeDir, false)
	if err != nil {
		t.Fatalf("CleanOrphanedArtifacts: %v", err)
	}

	if len(orphans) != 2 {
		t.Errorf("orphan count = %d, want 2; items: %v", len(orphans), orphans)
	}

	// Orphaned dirs should be removed
	if _, err := os.Stat(orphanEnv); !os.IsNotExist(err) {
		t.Error("orphaned session-env dir still exists")
	}
	if _, err := os.Stat(orphanTask); !os.IsNotExist(err) {
		t.Error("orphaned tasks dir still exists")
	}

	// Real session artifacts should remain
	if _, err := os.Stat(filepath.Join(claudeDir, "session-env", realID)); err != nil {
		t.Error("real session-env dir was incorrectly removed")
	}
	if _, err := os.Stat(filepath.Join(claudeDir, "tasks", realID)); err != nil {
		t.Error("real tasks dir was incorrectly removed")
	}
}

func TestListProjects(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)

	// Create projects with sessions
	proj1 := createProjectDir(t, claudeDir, "/home/user/proj1")
	proj2 := createProjectDir(t, claudeDir, "/home/user/proj2")

	id1 := uuid.New().String()
	id2 := uuid.New().String()
	testutil.WriteSession(t, proj1, id1, testutil.BuildSimpleSession(id1, "S1", "main", 1, "2026-03-20T10:00:00Z"))
	testutil.WriteSession(t, proj2, id2, testutil.BuildSimpleSession(id2, "S2", "main", 1, "2026-03-20T10:00:00Z"))

	// Create an observer project (should be excluded)
	obsDir := createProjectDir(t, claudeDir, "/home/user/claude-mem-observer-data")
	obsID := uuid.New().String()
	testutil.WriteSession(t, obsDir, obsID, testutil.BuildSimpleSession(obsID, "Observer", "main", 1, "2026-03-20T10:00:00Z"))

	// Create an empty project dir (no JSONL — should be excluded)
	createProjectDir(t, claudeDir, "/home/user/emptyproj")

	projects, err := ListProjects(claudeDir)
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}

	if len(projects) != 2 {
		t.Fatalf("project count = %d, want 2; projects: %v", len(projects), projects)
	}

	found := map[string]bool{}
	for _, p := range projects {
		found[p] = true
	}
	if !found["/home/user/proj1"] {
		t.Error("missing /home/user/proj1")
	}
	if !found["/home/user/proj2"] {
		t.Error("missing /home/user/proj2")
	}
}

package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func TestIsProcessRunning(t *testing.T) {
	// Current process should be running
	if !isProcessRunning(os.Getpid()) {
		t.Error("isProcessRunning(os.Getpid()) = false, want true")
	}

	// A very high PID should not be running
	if isProcessRunning(999999999) {
		t.Error("isProcessRunning(999999999) = true, want false")
	}
}

func TestActiveSessionIDs(t *testing.T) {
	// Create a temp dir mimicking ~/.claude with a sessions/ subdir
	tmpDir := t.TempDir()
	sessDir := filepath.Join(tmpDir, "sessions")
	if err := os.MkdirAll(sessDir, 0755); err != nil {
		t.Fatal(err)
	}

	runningSessionID := uuid.New().String()
	deadSessionID := uuid.New().String()

	// Write a session file with the current PID (running)
	runningData, _ := json.Marshal(activeSession{
		PID:       os.Getpid(),
		SessionID: runningSessionID,
		Cwd:       "/tmp",
	})
	if err := os.WriteFile(filepath.Join(sessDir, "running.json"), runningData, 0644); err != nil {
		t.Fatal(err)
	}

	// Write a session file with a dead PID
	deadData, _ := json.Marshal(activeSession{
		PID:       999999999,
		SessionID: deadSessionID,
		Cwd:       "/tmp",
	})
	if err := os.WriteFile(filepath.Join(sessDir, "dead.json"), deadData, 0644); err != nil {
		t.Fatal(err)
	}

	// Write a non-JSON file (should be skipped gracefully)
	if err := os.WriteFile(filepath.Join(sessDir, "garbage.json"), []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	// Write a non-.json file (should be ignored)
	if err := os.WriteFile(filepath.Join(sessDir, "readme.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	active, err := ActiveSessionIDs(tmpDir)
	if err != nil {
		t.Fatalf("ActiveSessionIDs: %v", err)
	}

	if !active[runningSessionID] {
		t.Errorf("running session %s not in active set", runningSessionID)
	}
	if active[deadSessionID] {
		t.Errorf("dead session %s should not be in active set", deadSessionID)
	}
	if len(active) != 1 {
		t.Errorf("active count = %d, want 1", len(active))
	}
}

func TestActiveSessionIDsNoDir(t *testing.T) {
	// When sessions/ directory doesn't exist, should return nil without error
	tmpDir := t.TempDir()
	active, err := ActiveSessionIDs(tmpDir)
	if err != nil {
		t.Fatalf("expected nil error for missing sessions dir, got: %v", err)
	}
	if active != nil {
		t.Errorf("expected nil map, got %v", active)
	}
}

package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type activeSession struct {
	PID       int    `json:"pid"`
	SessionID string `json:"sessionId"`
	Cwd       string `json:"cwd"`
}

// ActiveSessionIDs returns a set of session IDs that are currently running.
func ActiveSessionIDs(claudeDir string) (map[string]bool, error) {
	sessDir := filepath.Join(claudeDir, "sessions")
	entries, err := os.ReadDir(sessDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	active := make(map[string]bool)
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(sessDir, e.Name()))
		if err != nil {
			continue
		}

		var s activeSession
		if err := json.Unmarshal(data, &s); err != nil {
			continue
		}

		if s.PID > 0 && isProcessRunning(s.PID) {
			active[s.SessionID] = true
		}
	}
	return active, nil
}

func isProcessRunning(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

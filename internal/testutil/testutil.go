// Package testutil provides helpers for testing csm packages.
package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// CreateTempClaudeDir creates a temporary directory mimicking ~/.claude/projects/ structure.
func CreateTempClaudeDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, sub := range []string{"projects", "sessions", "tasks", "session-env"} {
		os.MkdirAll(filepath.Join(dir, sub), 0755)
	}
	return dir
}

// CreateProject creates a project directory and returns its path.
func CreateProject(t *testing.T, claudeDir, projectPath string) string {
	t.Helper()
	encoded := encodeProjectPath(projectPath)
	dir := filepath.Join(claudeDir, "projects", encoded)
	os.MkdirAll(dir, 0755)
	return dir
}

// WriteSession writes a JSONL session file and returns its path.
func WriteSession(t *testing.T, projDir, sessionID string, events []map[string]any) string {
	t.Helper()
	path := filepath.Join(projDir, sessionID+".jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, ev := range events {
		enc.Encode(ev)
	}
	return path
}

// MakeEvent creates a test event with the given type and fields merged with defaults.
func MakeEvent(typ string, fields map[string]any) map[string]any {
	ev := map[string]any{"type": typ}
	for k, v := range fields {
		ev[k] = v
	}
	return ev
}

// MakeCustomTitle creates a custom-title event.
func MakeCustomTitle(sessionID, title string) map[string]any {
	return map[string]any{
		"type":        "custom-title",
		"customTitle": title,
		"sessionId":   sessionID,
	}
}

// MakeAgentName creates an agent-name event.
func MakeAgentName(sessionID, name string) map[string]any {
	return map[string]any{
		"type":      "agent-name",
		"agentName": name,
		"sessionId": sessionID,
	}
}

// MakeSystemInit creates a system init event.
func MakeSystemInit(sessionID, timestamp string) map[string]any {
	return map[string]any{
		"type":        "system",
		"subtype":     "init",
		"content":     "",
		"level":       "info",
		"timestamp":   timestamp,
		"uuid":        uuid.New().String(),
		"sessionId":   sessionID,
		"isSidechain": false,
	}
}

// MakeUserMessage creates a user message event with string content.
func MakeUserMessage(sessionID, parentUUID, text, timestamp string) map[string]any {
	return map[string]any{
		"type":       "user",
		"userType":   "external",
		"entrypoint": "cli",
		"message": map[string]any{
			"role":    "user",
			"content": text,
		},
		"timestamp":   timestamp,
		"uuid":        uuid.New().String(),
		"parentUuid":  parentUUID,
		"sessionId":   sessionID,
		"isSidechain": false,
		"gitBranch":   "main",
	}
}

// MakeToolResultUser creates a user event that is a tool_result (not a real human prompt).
func MakeToolResultUser(sessionID, parentUUID, timestamp string) map[string]any {
	return map[string]any{
		"type":     "user",
		"userType": "external",
		"message": map[string]any{
			"role": "user",
			"content": []any{
				map[string]any{
					"type":        "tool_result",
					"tool_use_id": "toolu_test",
					"content":     "some tool output",
				},
			},
		},
		"timestamp":   timestamp,
		"uuid":        uuid.New().String(),
		"parentUuid":  parentUUID,
		"sessionId":   sessionID,
		"isSidechain": false,
	}
}

// MakeAssistant creates an assistant message event.
func MakeAssistant(sessionID, parentUUID, text, timestamp string) map[string]any {
	return map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"role": "assistant",
			"content": []any{
				map[string]any{"type": "text", "text": text},
			},
		},
		"timestamp":   timestamp,
		"uuid":        uuid.New().String(),
		"parentUuid":  parentUUID,
		"sessionId":   sessionID,
		"isSidechain": false,
	}
}

// MakeFileHistorySnapshot creates a file-history-snapshot event (no sessionId).
func MakeFileHistorySnapshot(messageID string) map[string]any {
	return map[string]any{
		"type":      "file-history-snapshot",
		"messageId": messageID,
		"snapshot": map[string]any{
			"messageId":          messageID,
			"trackedFileBackups": map[string]any{},
			"timestamp":          "2026-03-20T10:00:00Z",
		},
		"isSnapshotUpdate": false,
	}
}

// MakeSystemInjectedUser creates a user event with system-injected content.
func MakeSystemInjectedUser(sessionID, parentUUID, content, timestamp string) map[string]any {
	return map[string]any{
		"type":     "user",
		"userType": "external",
		"message": map[string]any{
			"role":    "user",
			"content": content,
		},
		"timestamp":   timestamp,
		"uuid":        uuid.New().String(),
		"parentUuid":  parentUUID,
		"sessionId":   sessionID,
		"isSidechain": false,
	}
}

// BuildSimpleSession creates a minimal session with title, init, and N user/assistant turns.
func BuildSimpleSession(sessionID, title, branch string, turns int, baseTimestamp string) []map[string]any {
	var events []map[string]any
	events = append(events, MakeCustomTitle(sessionID, title))
	events = append(events, MakeAgentName(sessionID, title))

	initEv := MakeSystemInit(sessionID, baseTimestamp)
	events = append(events, initEv)
	lastUUID := initEv["uuid"].(string)

	for i := 0; i < turns; i++ {
		userEv := MakeUserMessage(sessionID, lastUUID, "user prompt "+string(rune('A'+i)), baseTimestamp)
		events = append(events, userEv)
		lastUUID = userEv["uuid"].(string)

		assistEv := MakeAssistant(sessionID, lastUUID, "response "+string(rune('A'+i)), baseTimestamp)
		events = append(events, assistEv)
		lastUUID = assistEv["uuid"].(string)
	}
	return events
}

func encodeProjectPath(path string) string {
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}
	return "-" + strings.ReplaceAll(path, "/", "-")
}

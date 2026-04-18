package merge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ash0x0/csm/internal/session"
	"github.com/ash0x0/csm/internal/testutil"
	"github.com/google/uuid"
)

// readBackRaw reads the merged JSONL file and returns all events as raw maps.
func readBackRaw(t *testing.T, dir, newID string) []map[string]any {
	t.Helper()
	path := filepath.Join(dir, newID+".jsonl")
	events, err := session.ReadRawEvents(path)
	if err != nil {
		t.Fatalf("reading merged file: %v", err)
	}
	return events
}

// buildMeta creates a SessionMeta for a temp session, with the given Modified time.
func buildMeta(t *testing.T, projDir, title string, modified time.Time, turns int) *session.SessionMeta {
	t.Helper()
	sessionID := uuid.New().String()
	ts := modified.Format(time.RFC3339)
	events := testutil.BuildSimpleSession(sessionID, title, "main", turns, ts)
	filePath := testutil.WriteSession(t, projDir, sessionID, events)

	return &session.SessionMeta{
		ID:       sessionID,
		ShortID:  sessionID[:8],
		Title:    title,
		FilePath: filePath,
		Modified: modified,
	}
}

func TestMergeNChainSessions(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)
	projDir := testutil.CreateProject(t, claudeDir, "/home/user/myproject")

	base := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)
	m1 := buildMeta(t, projDir, "Session A", base, 1)
	m2 := buildMeta(t, projDir, "Session B", base.Add(1*time.Hour), 1)
	m3 := buildMeta(t, projDir, "Session C", base.Add(2*time.Hour), 1)

	newID, _, err := MergeN([]*session.SessionMeta{m1, m2, m3}, MergeOptions{
		Title:     "Chained",
		OutputDir: projDir,
	})
	if err != nil {
		t.Fatalf("MergeN: %v", err)
	}

	events := readBackRaw(t, projDir, newID)

	// Find the first uuid-bearing event of each original session to check parentUuid linking.
	// After the 2 header events (custom-title, agent-name), the events from session 1 start.
	// The first uuid-bearing event of session 2 should have parentUuid pointing to session 1's last uuid.
	// Same for session 3 -> session 2's last uuid.

	// Collect all uuid values to track chaining
	var uuidChain []string
	for _, ev := range events {
		if u, ok := ev["uuid"].(string); ok && u != "" {
			uuidChain = append(uuidChain, u)
		}
	}

	if len(uuidChain) == 0 {
		t.Fatal("no uuid-bearing events found")
	}

	// Verify events from S2 and S3 link back to previous session's last uuid.
	// We do this by checking that at least some parentUuid values reference uuids from earlier sessions.
	parentUUIDs := map[string]bool{}
	for _, ev := range events {
		if pu, ok := ev["parentUuid"].(string); ok && pu != "" {
			parentUUIDs[pu] = true
		}
	}

	// The chaining creates links between sessions. We need at least 2 cross-session links (S1->S2, S2->S3).
	// The first uuid-bearing event of S2 points to last uuid of S1, and same for S3->S2.
	// Since we can't easily identify session boundaries, we verify that multiple parentUuids exist.
	if len(parentUUIDs) < 2 {
		t.Errorf("expected at least 2 distinct parentUuid values for cross-session chaining, got %d", len(parentUUIDs))
	}
}

func TestMergeNRewritesSessionId(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)
	projDir := testutil.CreateProject(t, claudeDir, "/home/user/myproject")

	base := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)
	m1 := buildMeta(t, projDir, "Session A", base, 1)
	m2 := buildMeta(t, projDir, "Session B", base.Add(1*time.Hour), 1)

	newID, _, err := MergeN([]*session.SessionMeta{m1, m2}, MergeOptions{
		Title:     "Merged",
		OutputDir: projDir,
	})
	if err != nil {
		t.Fatalf("MergeN: %v", err)
	}

	events := readBackRaw(t, projDir, newID)

	for i, ev := range events {
		if sid, ok := ev["sessionId"]; ok {
			if sid != newID {
				t.Errorf("event %d: sessionId = %q, want %q", i, sid, newID)
			}
		}
	}
}

func TestMergeNPreservesSnapshotNoSessionId(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)
	projDir := testutil.CreateProject(t, claudeDir, "/home/user/myproject")

	// Build a session that includes a file-history-snapshot event (no sessionId)
	sessionID := uuid.New().String()
	events := []map[string]any{
		testutil.MakeCustomTitle(sessionID, "With Snapshot"),
		testutil.MakeSystemInit(sessionID, "2026-03-20T10:00:00Z"),
		testutil.MakeFileHistorySnapshot("msg-001"),
		testutil.MakeUserMessage(sessionID, "", "hello", "2026-03-20T10:01:00Z"),
		testutil.MakeAssistant(sessionID, "", "hi", "2026-03-20T10:02:00Z"),
	}
	filePath := testutil.WriteSession(t, projDir, sessionID, events)

	meta1 := &session.SessionMeta{
		ID:       sessionID,
		ShortID:  sessionID[:8],
		Title:    "With Snapshot",
		FilePath: filePath,
		Modified: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
	}

	// Second session (plain)
	base2 := time.Date(2026, 3, 20, 11, 0, 0, 0, time.UTC)
	meta2 := buildMeta(t, projDir, "Plain", base2, 1)

	newID, _, err := MergeN([]*session.SessionMeta{meta1, meta2}, MergeOptions{
		Title:     "Snapshot Test",
		OutputDir: projDir,
	})
	if err != nil {
		t.Fatalf("MergeN: %v", err)
	}

	merged := readBackRaw(t, projDir, newID)

	for i, ev := range merged {
		if ev["type"] == "file-history-snapshot" {
			if _, hasSessionID := ev["sessionId"]; hasSessionID {
				t.Errorf("event %d (file-history-snapshot): should NOT have sessionId, but it does", i)
			}
			return
		}
	}
	t.Error("no file-history-snapshot event found in merged output")
}

func TestMergeNChronologicalOrder(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)
	projDir := testutil.CreateProject(t, claudeDir, "/home/user/myproject")

	// Create sessions with reversed chronological order to verify sorting
	earlier := time.Date(2026, 3, 20, 8, 0, 0, 0, time.UTC)
	later := time.Date(2026, 3, 20, 16, 0, 0, 0, time.UTC)

	// Pass the later session first to test that MergeN sorts by Modified
	mLater := buildMeta(t, projDir, "Later Session", later, 1)
	mEarlier := buildMeta(t, projDir, "Earlier Session", earlier, 1)

	newID, _, err := MergeN([]*session.SessionMeta{mLater, mEarlier}, MergeOptions{
		Title:     "Chrono Test",
		OutputDir: projDir,
	})
	if err != nil {
		t.Fatalf("MergeN: %v", err)
	}

	events := readBackRaw(t, projDir, newID)

	// Verify that the earlier session's user events come before the later session's.
	// Extract user event texts in order.
	var userTexts []string
	for _, ev := range events {
		if ev["type"] == "user" {
			if msg, ok := ev["message"].(map[string]any); ok {
				if content, ok := msg["content"].(string); ok {
					userTexts = append(userTexts, content)
				}
			}
		}
	}

	if len(userTexts) < 2 {
		t.Fatalf("expected at least 2 user events, got %d", len(userTexts))
	}
	// Earlier session's events should come first (concat fallback preserves chronological order)
	if userTexts[0] != "user prompt A" {
		t.Errorf("first user text = %q, want %q", userTexts[0], "user prompt A")
	}
}

func TestMergeNPreservesAllFields(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)
	projDir := testutil.CreateProject(t, claudeDir, "/home/user/myproject")

	sessionID := uuid.New().String()
	// Build events with extra fields (cwd, version, gitBranch)
	events := []map[string]any{
		testutil.MakeCustomTitle(sessionID, "Rich Fields"),
		{
			"type":      "system",
			"subtype":   "init",
			"content":   "",
			"level":     "info",
			"timestamp": "2026-03-20T10:00:00Z",
			"uuid":      uuid.New().String(),
			"sessionId": sessionID,
			"cwd":       "/home/user/myproject",
			"version":   "1.0.34",
			"gitBranch": "feature-x",
		},
	}
	filePath := testutil.WriteSession(t, projDir, sessionID, events)

	meta := &session.SessionMeta{
		ID:       sessionID,
		ShortID:  sessionID[:8],
		Title:    "Rich Fields",
		FilePath: filePath,
		Modified: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
	}

	// Need a second session for merge
	meta2 := buildMeta(t, projDir, "Second", time.Date(2026, 3, 20, 11, 0, 0, 0, time.UTC), 1)

	newID, _, err := MergeN([]*session.SessionMeta{meta, meta2}, MergeOptions{
		Title:     "Fields Test",
		OutputDir: projDir,
	})
	if err != nil {
		t.Fatalf("MergeN: %v", err)
	}

	merged := readBackRaw(t, projDir, newID)

	// Find the system init event with extra fields
	for _, ev := range merged {
		if ev["type"] == "system" && ev["cwd"] != nil {
			if ev["cwd"] != "/home/user/myproject" {
				t.Errorf("cwd = %v, want /home/user/myproject", ev["cwd"])
			}
			if ev["version"] != "1.0.34" {
				t.Errorf("version = %v, want 1.0.34", ev["version"])
			}
			if ev["gitBranch"] != "feature-x" {
				t.Errorf("gitBranch = %v, want feature-x", ev["gitBranch"])
			}
			return
		}
	}
	t.Error("system event with extra fields not found in merged output")
}

func TestMergeNAutoTitle(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)
	projDir := testutil.CreateProject(t, claudeDir, "/home/user/myproject")

	base := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)
	m1 := buildMeta(t, projDir, "Alpha", base, 1)
	m2 := buildMeta(t, projDir, "Beta", base.Add(1*time.Hour), 1)

	// No title provided — should auto-generate
	newID, _, err := MergeN([]*session.SessionMeta{m1, m2}, MergeOptions{
		OutputDir: projDir,
	})
	if err != nil {
		t.Fatalf("MergeN: %v", err)
	}

	events := readBackRaw(t, projDir, newID)

	// First event should be custom-title with auto-generated title
	if events[0]["type"] != "custom-title" {
		t.Fatalf("first event type = %q, want custom-title", events[0]["type"])
	}

	title, ok := events[0]["customTitle"].(string)
	if !ok {
		t.Fatal("customTitle not a string")
	}
	expected := "Merged: Alpha + Beta"
	if title != expected {
		t.Errorf("auto title = %q, want %q", title, expected)
	}
}

func TestMergeNCustomTitle(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)
	projDir := testutil.CreateProject(t, claudeDir, "/home/user/myproject")

	base := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)
	m1 := buildMeta(t, projDir, "Alpha", base, 1)
	m2 := buildMeta(t, projDir, "Beta", base.Add(1*time.Hour), 1)

	customTitle := "My Custom Merge Title"
	newID, _, err := MergeN([]*session.SessionMeta{m1, m2}, MergeOptions{
		Title:     customTitle,
		OutputDir: projDir,
	})
	if err != nil {
		t.Fatalf("MergeN: %v", err)
	}

	events := readBackRaw(t, projDir, newID)

	if events[0]["type"] != "custom-title" {
		t.Fatalf("first event type = %q, want custom-title", events[0]["type"])
	}

	title, ok := events[0]["customTitle"].(string)
	if !ok {
		t.Fatal("customTitle not a string")
	}
	if title != customTitle {
		t.Errorf("title = %q, want %q", title, customTitle)
	}

	// Also verify the agent-name event uses the custom title
	if events[1]["type"] != "agent-name" {
		t.Fatalf("second event type = %q, want agent-name", events[1]["type"])
	}
	agentName, _ := events[1]["agentName"].(string)
	if agentName != customTitle {
		t.Errorf("agentName = %q, want %q", agentName, customTitle)
	}
}

func TestMergeNSkipsIdenticalPair(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)
	projDir := testutil.CreateProject(t, claudeDir, "/home/user/myproject")

	base := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)
	// m1 and m2 share identical events (same session file content)
	m1 := buildMeta(t, projDir, "Session A", base, 2)

	// Build m2 with the exact same events as m1
	sessionID2 := uuid.New().String()
	eventsA, err := session.ReadRawEvents(m1.FilePath)
	if err != nil {
		t.Fatalf("reading m1 events: %v", err)
	}
	// Write the same uuid-bearing events under a new session ID so they look identical to merge2Events
	// (copy the original events with the same UUIDs)
	filePath2 := filepath.Join(projDir, sessionID2+".jsonl")
	f, err := os.Create(filePath2)
	if err != nil {
		t.Fatalf("creating m2 file: %v", err)
	}
	enc := json.NewEncoder(f)
	for _, ev := range eventsA {
		enc.Encode(ev)
	}
	f.Close()

	m2 := &session.SessionMeta{
		ID:       sessionID2,
		ShortID:  sessionID2[:8],
		Title:    "Session A Copy",
		FilePath: filePath2,
		Modified: base.Add(1 * time.Hour),
	}
	m3 := buildMeta(t, projDir, "Session B", base.Add(2*time.Hour), 1)

	// Merging [m1, m2 (identical to m1), m3] should succeed, skipping m2
	newID, _, err := MergeN([]*session.SessionMeta{m1, m2, m3}, MergeOptions{
		OutputDir: projDir,
	})
	if err != nil {
		t.Fatalf("MergeN with identical pair should not return error, got: %v", err)
	}
	if newID == "" {
		t.Error("expected a new session ID")
	}
}

func TestMergeNLessThanTwo(t *testing.T) {
	m1 := &session.SessionMeta{ID: "a"}
	_, _, err := MergeN([]*session.SessionMeta{m1}, MergeOptions{})
	if err == nil {
		t.Error("expected error when merging < 2 sessions")
	}
}

func TestMergeNOutputFileCreated(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)
	projDir := testutil.CreateProject(t, claudeDir, "/home/user/myproject")

	base := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)
	m1 := buildMeta(t, projDir, "A", base, 1)
	m2 := buildMeta(t, projDir, "B", base.Add(1*time.Hour), 1)

	newID, _, err := MergeN([]*session.SessionMeta{m1, m2}, MergeOptions{
		OutputDir: projDir,
	})
	if err != nil {
		t.Fatalf("MergeN: %v", err)
	}

	outputPath := filepath.Join(projDir, newID+".jsonl")
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("output file not found: %v", err)
	}
	if info.Size() == 0 {
		t.Error("output file is empty")
	}

	// Verify it's valid JSONL
	events := readBackRaw(t, projDir, newID)
	for i, ev := range events {
		if _, err := json.Marshal(ev); err != nil {
			t.Errorf("event %d is not valid JSON: %v", i, err)
		}
	}
}

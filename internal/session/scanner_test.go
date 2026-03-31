package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ash0x0/csm/internal/testutil"
)

// createProjectDir creates a project directory using the dash-encoded format
// that the scanner expects (e.g. /home/ahmed/code/proj -> -home-ahmed-code-proj).
func createProjectDir(t *testing.T, claudeDir, projectPath string) string {
	t.Helper()
	encoded := strings.ReplaceAll(projectPath, "/", "-")
	dir := filepath.Join(claudeDir, "projects", encoded)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestDecodeProjectPath(t *testing.T) {
	tests := []struct {
		name    string
		encoded string
		want    string
	}{
		{
			name:    "typical multi-segment path",
			encoded: "-home-ahmed-code-cercli",
			want:    "/home/ahmed/code/cercli",
		},
		{
			name:    "empty string",
			encoded: "",
			want:    "",
		},
		{
			name:    "single segment",
			encoded: "-tmp",
			want:    "/tmp",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeProjectPath(tt.encoded)
			if got != tt.want {
				t.Errorf("decodeProjectPath(%q) = %q, want %q", tt.encoded, got, tt.want)
			}
		})
	}
}

func TestExtractUserText(t *testing.T) {
	tests := []struct {
		name string
		msg  any
		want string
	}{
		{
			name: "string content (old format)",
			msg: map[string]any{
				"role":    "user",
				"content": "hello world",
			},
			want: "hello world",
		},
		{
			name: "list content with text blocks",
			msg: map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "block text here"},
				},
			},
			want: "block text here",
		},
		{
			name: "nil message",
			msg:  nil,
			want: "",
		},
		{
			name: "tool_result content returns empty",
			msg: map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "tool_result", "tool_use_id": "toolu_1", "content": "output"},
				},
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractUserText(tt.msg)
			if got != tt.want {
				t.Errorf("extractUserText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsToolResult(t *testing.T) {
	tests := []struct {
		name string
		msg  any
		want bool
	}{
		{
			name: "list with tool_result block",
			msg: map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "tool_result", "tool_use_id": "toolu_1", "content": "output"},
				},
			},
			want: true,
		},
		{
			name: "list with only text blocks",
			msg: map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "hello"},
				},
			},
			want: false,
		},
		{
			name: "string content",
			msg: map[string]any{
				"role":    "user",
				"content": "plain string",
			},
			want: false,
		},
		{
			name: "nil message",
			msg:  nil,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isToolResult(tt.msg)
			if got != tt.want {
				t.Errorf("isToolResult() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsSystemInjected(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{name: "local-command prefix", text: "<local-command>ls</local-command>", want: true},
		{name: "command-name prefix", text: "<command-name>git status</command-name>", want: true},
		{name: "session continuation", text: "This session is being continued from a previous conversation", want: true},
		{name: "skill base directory", text: "Base directory for this skill: /home/user", want: true},
		{name: "request interrupted", text: "[Request interrupted by user]", want: true},
		{name: "normal user text", text: "Fix the bug in main.go", want: false},
		{name: "empty string", text: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSystemInjected(tt.text)
			if got != tt.want {
				t.Errorf("isSystemInjected(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestScanBasic(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)

	proj1Dir := createProjectDir(t, claudeDir, "/home/ahmed/code/alpha")
	proj2Dir := createProjectDir(t, claudeDir, "/home/ahmed/code/beta")

	testutil.WriteSession(t, proj1Dir, "sess-aaa-111", testutil.BuildSimpleSession(
		"sess-aaa-111", "Alpha session", "main", 2, "2026-03-20T10:00:00Z",
	))
	testutil.WriteSession(t, proj2Dir, "sess-bbb-222", testutil.BuildSimpleSession(
		"sess-bbb-222", "Beta session", "main", 3, "2026-03-21T12:00:00Z",
	))

	sc := NewScanner(claudeDir)
	results, err := sc.Scan(ScanOptions{})
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(results))
	}

	// Results are sorted most-recent first
	if results[0].ID != "sess-bbb-222" {
		t.Errorf("expected first result ID = %q, got %q", "sess-bbb-222", results[0].ID)
	}
	if results[1].ID != "sess-aaa-111" {
		t.Errorf("expected second result ID = %q, got %q", "sess-aaa-111", results[1].ID)
	}

	// Verify metadata
	if results[0].Title != "Beta session" {
		t.Errorf("expected title %q, got %q", "Beta session", results[0].Title)
	}
	if results[0].Project != "/home/ahmed/code/beta" {
		t.Errorf("expected project %q, got %q", "/home/ahmed/code/beta", results[0].Project)
	}
}

func TestScanExcludesObservers(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)

	// Regular project
	projDir := createProjectDir(t, claudeDir, "/home/ahmed/code/myapp")
	testutil.WriteSession(t, projDir, "sess-real", testutil.BuildSimpleSession(
		"sess-real", "Real session", "main", 1, "2026-03-20T10:00:00Z",
	))

	// Observer project (contains the observer token in the encoded path)
	obsEncoded := "-home-ahmed-code-myapp-claude-mem-observer"
	obsDir := filepath.Join(claudeDir, "projects", obsEncoded)
	os.MkdirAll(obsDir, 0755)
	testutil.WriteSession(t, obsDir, "sess-obs", testutil.BuildSimpleSession(
		"sess-obs", "Observer session", "main", 1, "2026-03-20T10:00:00Z",
	))

	sc := NewScanner(claudeDir)

	// Default: observers excluded
	results, err := sc.Scan(ScanOptions{})
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 session (observers excluded), got %d", len(results))
	}
	if results[0].ID != "sess-real" {
		t.Errorf("expected ID %q, got %q", "sess-real", results[0].ID)
	}

	// IncludeObservers: both included
	results, err = sc.Scan(ScanOptions{IncludeObservers: true})
	if err != nil {
		t.Fatalf("Scan(IncludeObservers) error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 sessions (observers included), got %d", len(results))
	}
}

func TestMessageCountExcludesToolResults(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)
	projDir := createProjectDir(t, claudeDir, "/home/ahmed/code/proj")

	sid := "sess-msg-count"
	events := []map[string]any{
		testutil.MakeSystemInit(sid, "2026-03-20T10:00:00Z"),
		testutil.MakeUserMessage(sid, "", "first real prompt", "2026-03-20T10:01:00Z"),
		testutil.MakeAssistant(sid, "", "response 1", "2026-03-20T10:02:00Z"),
		testutil.MakeToolResultUser(sid, "", "2026-03-20T10:03:00Z"),
		testutil.MakeUserMessage(sid, "", "second real prompt", "2026-03-20T10:04:00Z"),
		testutil.MakeAssistant(sid, "", "response 2", "2026-03-20T10:05:00Z"),
		testutil.MakeToolResultUser(sid, "", "2026-03-20T10:06:00Z"),
		testutil.MakeToolResultUser(sid, "", "2026-03-20T10:07:00Z"),
	}

	testutil.WriteSession(t, projDir, sid, events)

	sc := NewScanner(claudeDir)
	results, err := sc.Scan(ScanOptions{})
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 session, got %d", len(results))
	}

	// Only 2 real user prompts, not 5 (which would include 3 tool_results)
	if results[0].Messages != 2 {
		t.Errorf("expected 2 messages (excluding tool_results), got %d", results[0].Messages)
	}
}

func TestTitleFromLastCustomTitle(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)
	projDir := createProjectDir(t, claudeDir, "/home/ahmed/code/proj")

	sid := "sess-title-rename"
	events := []map[string]any{
		testutil.MakeCustomTitle(sid, "Original Title"),
		testutil.MakeSystemInit(sid, "2026-03-20T10:00:00Z"),
		testutil.MakeUserMessage(sid, "", "hello", "2026-03-20T10:01:00Z"),
		testutil.MakeAssistant(sid, "", "hi", "2026-03-20T10:02:00Z"),
		testutil.MakeCustomTitle(sid, "Renamed Title"),
		testutil.MakeUserMessage(sid, "", "more work", "2026-03-20T10:03:00Z"),
		testutil.MakeAssistant(sid, "", "done", "2026-03-20T10:04:00Z"),
		testutil.MakeCustomTitle(sid, "Final Title"),
	}

	testutil.WriteSession(t, projDir, sid, events)

	sc := NewScanner(claudeDir)
	results, err := sc.Scan(ScanOptions{})
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 session, got %d", len(results))
	}
	if results[0].Title != "Final Title" {
		t.Errorf("expected title %q, got %q", "Final Title", results[0].Title)
	}
}

func TestFindSessionByPrefix(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)
	projDir := createProjectDir(t, claudeDir, "/home/ahmed/code/proj")

	testutil.WriteSession(t, projDir, "abc12345-def", testutil.BuildSimpleSession(
		"abc12345-def", "ABC session", "main", 1, "2026-03-20T10:00:00Z",
	))
	testutil.WriteSession(t, projDir, "xyz99999-ghi", testutil.BuildSimpleSession(
		"xyz99999-ghi", "XYZ session", "main", 1, "2026-03-20T11:00:00Z",
	))

	sc := NewScanner(claudeDir)

	found, err := sc.FindSessionByPrefix("abc12")
	if err != nil {
		t.Fatalf("FindSessionByPrefix() error: %v", err)
	}
	if found == nil {
		t.Fatal("expected match, got nil")
	}
	if found.ID != "abc12345-def" {
		t.Errorf("expected ID %q, got %q", "abc12345-def", found.ID)
	}
}

func TestFindSessionByTitle(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)
	projDir := createProjectDir(t, claudeDir, "/home/ahmed/code/proj")

	testutil.WriteSession(t, projDir, "sess-find-title", testutil.BuildSimpleSession(
		"sess-find-title", "Unique Flamingo Title", "main", 1, "2026-03-20T10:00:00Z",
	))
	testutil.WriteSession(t, projDir, "sess-other", testutil.BuildSimpleSession(
		"sess-other", "Other Title", "main", 1, "2026-03-20T11:00:00Z",
	))

	sc := NewScanner(claudeDir)

	found, err := sc.FindSessionByPrefix("flamingo")
	if err != nil {
		t.Fatalf("FindSessionByPrefix() error: %v", err)
	}
	if found == nil {
		t.Fatal("expected match by title, got nil")
	}
	if found.ID != "sess-find-title" {
		t.Errorf("expected ID %q, got %q", "sess-find-title", found.ID)
	}
}

func TestAmbiguousMatch(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)
	projDir := createProjectDir(t, claudeDir, "/home/ahmed/code/proj")

	// Two sessions with IDs starting with "dup"
	testutil.WriteSession(t, projDir, "dup-aaa-111", testutil.BuildSimpleSession(
		"dup-aaa-111", "Session A", "main", 1, "2026-03-20T10:00:00Z",
	))
	testutil.WriteSession(t, projDir, "dup-bbb-222", testutil.BuildSimpleSession(
		"dup-bbb-222", "Session B", "main", 1, "2026-03-20T11:00:00Z",
	))

	sc := NewScanner(claudeDir)

	_, err := sc.FindSessionByPrefix("dup")
	if err == nil {
		t.Fatal("expected AmbiguousMatchError, got nil")
	}
	ambErr, ok := err.(*AmbiguousMatchError)
	if !ok {
		t.Fatalf("expected *AmbiguousMatchError, got %T: %v", err, err)
	}
	if len(ambErr.Matches) != 2 {
		t.Errorf("expected 2 matches, got %d", len(ambErr.Matches))
	}
	if ambErr.Query != "dup" {
		t.Errorf("expected query %q, got %q", "dup", ambErr.Query)
	}
}

func TestReadRawEvents(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)
	projDir := createProjectDir(t, claudeDir, "/home/ahmed/code/proj")

	sid := "sess-raw-events"
	events := []map[string]any{
		testutil.MakeSystemInit(sid, "2026-03-20T10:00:00Z"),
		testutil.MakeUserMessage(sid, "", "hello raw", "2026-03-20T10:01:00Z"),
		testutil.MakeAssistant(sid, "", "hi raw", "2026-03-20T10:02:00Z"),
	}
	fp := testutil.WriteSession(t, projDir, sid, events)

	raw, err := ReadRawEvents(fp)
	if err != nil {
		t.Fatalf("ReadRawEvents() error: %v", err)
	}
	if len(raw) != 3 {
		t.Fatalf("expected 3 raw events, got %d", len(raw))
	}

	// Verify fields preserved
	if raw[0]["type"] != "system" {
		t.Errorf("expected first event type %q, got %v", "system", raw[0]["type"])
	}
	if raw[1]["type"] != "user" {
		t.Errorf("expected second event type %q, got %v", "user", raw[1]["type"])
	}
	if raw[2]["type"] != "assistant" {
		t.Errorf("expected third event type %q, got %v", "assistant", raw[2]["type"])
	}

	// Verify sessionId preserved as map[string]any
	if raw[0]["sessionId"] != sid {
		t.Errorf("expected sessionId %q, got %v", sid, raw[0]["sessionId"])
	}
}

func TestCacheInvalidation(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)
	projDir := createProjectDir(t, claudeDir, "/home/ahmed/code/proj")

	sid := "sess-cache-inv"
	fp := testutil.WriteSession(t, projDir, sid, testutil.BuildSimpleSession(
		sid, "Cache Test", "main", 1, "2026-03-20T10:00:00Z",
	))

	sc := NewScanner(claudeDir)

	// First scan: populates cache
	results1, err := sc.Scan(ScanOptions{})
	if err != nil {
		t.Fatalf("first Scan() error: %v", err)
	}
	if len(results1) != 1 {
		t.Fatalf("expected 1 session, got %d", len(results1))
	}
	origMessages := results1[0].Messages

	// Modify the file: append new user messages
	f, err := os.OpenFile(fp, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("open for append: %v", err)
	}
	// Ensure mtime actually changes (some filesystems have 1s granularity)
	time.Sleep(10 * time.Millisecond)

	appendEvents := []map[string]any{
		testutil.MakeUserMessage(sid, "", "new prompt after cache", "2026-03-20T11:00:00Z"),
		testutil.MakeAssistant(sid, "", "new response", "2026-03-20T11:01:00Z"),
	}
	for _, ev := range appendEvents {
		data, _ := json.Marshal(ev)
		f.Write(data)
		f.Write([]byte("\n"))
	}
	f.Close()

	// Force mtime change by touching the file with a future time
	futureTime := time.Now().Add(1 * time.Hour)
	os.Chtimes(fp, futureTime, futureTime)

	// Second scan: should detect changed mtime and re-read
	results2, err := sc.Scan(ScanOptions{})
	if err != nil {
		t.Fatalf("second Scan() error: %v", err)
	}
	if len(results2) != 1 {
		t.Fatalf("expected 1 session, got %d", len(results2))
	}
	if results2[0].Messages <= origMessages {
		t.Errorf("expected message count to increase after modification: was %d, now %d", origMessages, results2[0].Messages)
	}
}

func TestReadUserPrompts(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)
	projDir := createProjectDir(t, claudeDir, "/home/test/prompts")
	sid := "sess-prompts"

	events := []map[string]any{
		testutil.MakeCustomTitle(sid, "test"),
		testutil.MakeSystemInit(sid, "2026-03-20T10:00:00Z"),
		testutil.MakeUserMessage(sid, "", "hello world", "2026-03-20T10:01:00Z"),
		testutil.MakeToolResultUser(sid, "", "2026-03-20T10:02:00Z"),
		testutil.MakeSystemInjectedUser(sid, "", "<local-command-stdout>done</local-command-stdout>", "2026-03-20T10:03:00Z"),
		testutil.MakeUserMessage(sid, "", "second real prompt", "2026-03-20T10:04:00Z"),
		testutil.MakeSystemInjectedUser(sid, "", "This session is being continued from a previous conversation", "2026-03-20T10:05:00Z"),
		testutil.MakeUserMessage(sid, "", "third real prompt", "2026-03-20T10:06:00Z"),
	}
	fp := testutil.WriteSession(t, projDir, sid, events)

	prompts, err := ReadUserPrompts(fp, 10)
	if err != nil {
		t.Fatalf("ReadUserPrompts() error: %v", err)
	}
	if len(prompts) != 3 {
		t.Fatalf("expected 3 prompts, got %d: %v", len(prompts), prompts)
	}
	if prompts[0] != "hello world" {
		t.Errorf("prompt[0] = %q, want %q", prompts[0], "hello world")
	}
	if prompts[1] != "second real prompt" {
		t.Errorf("prompt[1] = %q, want %q", prompts[1], "second real prompt")
	}
}

func TestReadUserPromptsMaxLimit(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)
	projDir := createProjectDir(t, claudeDir, "/home/test/limit")
	sid := "sess-limit"

	events := []map[string]any{testutil.MakeCustomTitle(sid, "test")}
	for i := 0; i < 10; i++ {
		events = append(events, testutil.MakeUserMessage(sid, "", "prompt", "2026-03-20T10:00:00Z"))
	}
	fp := testutil.WriteSession(t, projDir, sid, events)

	prompts, err := ReadUserPrompts(fp, 3)
	if err != nil {
		t.Fatalf("ReadUserPrompts() error: %v", err)
	}
	if len(prompts) != 3 {
		t.Fatalf("expected 3 prompts (max), got %d", len(prompts))
	}
}

func TestReadAssistantTexts(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)
	projDir := createProjectDir(t, claudeDir, "/home/test/assist")
	sid := "sess-assist"

	events := []map[string]any{
		testutil.MakeCustomTitle(sid, "test"),
		testutil.MakeAssistant(sid, "", "first response", "2026-03-20T10:01:00Z"),
		testutil.MakeAssistant(sid, "", "second response", "2026-03-20T10:02:00Z"),
	}
	fp := testutil.WriteSession(t, projDir, sid, events)

	texts, err := ReadAssistantTexts(fp, 10)
	if err != nil {
		t.Fatalf("ReadAssistantTexts() error: %v", err)
	}
	if len(texts) != 2 {
		t.Fatalf("expected 2 assistant texts, got %d", len(texts))
	}
	if texts[0] != "first response" {
		t.Errorf("text[0] = %q, want %q", texts[0], "first response")
	}
}

func TestExtractTimestamp(t *testing.T) {
	line := []byte(`{"type":"user","timestamp":"2026-03-20T10:00:00Z","uuid":"abc"}`)
	ts := extractTimestamp(line)
	if ts != "2026-03-20T10:00:00Z" {
		t.Errorf("extractTimestamp = %q, want %q", ts, "2026-03-20T10:00:00Z")
	}

	noTs := []byte(`{"type":"user","uuid":"abc"}`)
	if ts := extractTimestamp(noTs); ts != "" {
		t.Errorf("expected empty timestamp, got %q", ts)
	}
}

func TestExtractCustomTitle(t *testing.T) {
	line := []byte(`{"type":"custom-title","customTitle":"my-session","sessionId":"abc"}`)
	title := extractCustomTitle(line)
	if title != "my-session" {
		t.Errorf("extractCustomTitle = %q, want %q", title, "my-session")
	}

	noTitle := []byte(`{"type":"user","message":"hello"}`)
	if title := extractCustomTitle(noTitle); title != "" {
		t.Errorf("expected empty title, got %q", title)
	}
}

func TestContainsBytes(t *testing.T) {
	data := []byte(`{"type":"user","message":"hello"}`)
	if !containsBytes(data, `"type":"user"`) {
		t.Error("expected containsBytes to find user type")
	}
	if containsBytes(data, `"type":"assistant"`) {
		t.Error("expected containsBytes to not find assistant type")
	}
}

func TestDecodeProjectPathExported(t *testing.T) {
	result := DecodeProjectPath("-home-ahmed-code")
	if result != "/home/ahmed/code" {
		t.Errorf("DecodeProjectPath = %q, want %q", result, "/home/ahmed/code")
	}
}

func TestReadSessionEvents(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)
	projDir := createProjectDir(t, claudeDir, "/home/test/events")
	sid := "sess-events"
	events := testutil.BuildSimpleSession(sid, "test", "main", 5, "2026-03-20T10:00:00Z")
	fp := testutil.WriteSession(t, projDir, sid, events)

	head, tail, total, err := ReadSessionEvents(fp, 3, 2)
	if err != nil {
		t.Fatalf("ReadSessionEvents() error: %v", err)
	}
	if total != len(events) {
		t.Errorf("total = %d, want %d", total, len(events))
	}
	if len(head) != 3 {
		t.Errorf("head length = %d, want 3", len(head))
	}
	if len(tail) != 2 {
		t.Errorf("tail length = %d, want 2", len(tail))
	}
}

func TestAmbiguousMatchError(t *testing.T) {
	err := &AmbiguousMatchError{
		Query:   "test",
		Matches: []SessionMeta{{ShortID: "abc"}, {ShortID: "def"}},
	}
	msg := err.Error()
	if !strings.Contains(msg, "ambiguous") || !strings.Contains(msg, "test") {
		t.Errorf("error message = %q, expected to contain 'ambiguous' and 'test'", msg)
	}
}

package format

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ash0x0/csm/internal/session"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{name: "zero bytes", bytes: 0, want: "0 B"},
		{name: "500 bytes", bytes: 500, want: "500 B"},
		{name: "1500 bytes as KB", bytes: 1500, want: "1.5 KB"},
		{name: "2 MB", bytes: 2 * 1024 * 1024, want: "2.0 MB"},
		{name: "1.5 GB", bytes: int64(1.5 * 1024 * 1024 * 1024), want: "1.5 GB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSize(tt.bytes)
			if got != tt.want {
				t.Errorf("formatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestNormalizeBranch(t *testing.T) {
	tests := []struct {
		name   string
		branch string
		want   string
	}{
		{name: "HEAD returns empty", branch: "HEAD", want: ""},
		{name: "main unchanged", branch: "main", want: "main"},
		{name: "empty unchanged", branch: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeBranch(tt.branch)
			if got != tt.want {
				t.Errorf("normalizeBranch(%q) = %q, want %q", tt.branch, got, tt.want)
			}
		})
	}
}

func TestFormatDate(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{
			name: "within last 24h shows HH:MM",
			t:    now.Add(-2 * time.Hour),
			want: now.Add(-2 * time.Hour).Format("15:04"),
		},
		{
			name: "within last year shows Mon DD",
			t:    now.Add(-30 * 24 * time.Hour),
			want: now.Add(-30 * 24 * time.Hour).Format("Jan 02"),
		},
		{
			name: "older than a year shows YYYY-MM-DD",
			t:    now.Add(-400 * 24 * time.Hour),
			want: now.Add(-400 * 24 * time.Hour).Format("2006-01-02"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDate(tt.t)
			if got != tt.want {
				t.Errorf("formatDate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTruncateStr(t *testing.T) {
	tests := []struct {
		name string
		s    string
		max  int
		want string
	}{
		{
			name: "short string unchanged",
			s:    "hello",
			max:  10,
			want: "hello",
		},
		{
			name: "exact length unchanged",
			s:    "hello",
			max:  5,
			want: "hello",
		},
		{
			name: "long string truncated with ellipsis",
			s:    "this is a long title that should be truncated",
			max:  20,
			want: "this is a long ti...",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateStr(tt.s, tt.max)
			if got != tt.want {
				t.Errorf("truncateStr(%q, %d) = %q, want %q", tt.s, tt.max, got, tt.want)
			}
		})
	}
}

func TestGroupByProject(t *testing.T) {
	now := time.Now()

	sessions := []session.SessionMeta{
		{ID: "s1", Project: "/home/ahmed/code/alpha", Modified: now.Add(-1 * time.Hour)},
		{ID: "s2", Project: "/home/ahmed/code/beta", Modified: now.Add(-30 * time.Minute)},
		{ID: "s3", Project: "/home/ahmed/code/alpha", Modified: now.Add(-2 * time.Hour)},
		{ID: "s4", Project: "/home/ahmed/code/gamma", Modified: now.Add(-3 * time.Hour)},
		{ID: "s5", Project: "/home/ahmed/code/beta", Modified: now.Add(-4 * time.Hour)},
	}

	groups := groupByProject(sessions)

	// Should have 3 projects
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}

	// Most recently modified group first: beta (s2 at -30min)
	if groups[0].project != "/home/ahmed/code/beta" {
		t.Errorf("expected first group %q, got %q", "/home/ahmed/code/beta", groups[0].project)
	}
	if len(groups[0].sessions) != 2 {
		t.Errorf("expected 2 sessions in beta, got %d", len(groups[0].sessions))
	}

	// Second: alpha (s1 at -1h)
	if groups[1].project != "/home/ahmed/code/alpha" {
		t.Errorf("expected second group %q, got %q", "/home/ahmed/code/alpha", groups[1].project)
	}
	if len(groups[1].sessions) != 2 {
		t.Errorf("expected 2 sessions in alpha, got %d", len(groups[1].sessions))
	}

	// Third: gamma (s4 at -3h)
	if groups[2].project != "/home/ahmed/code/gamma" {
		t.Errorf("expected third group %q, got %q", "/home/ahmed/code/gamma", groups[2].project)
	}
	if len(groups[2].sessions) != 1 {
		t.Errorf("expected 1 session in gamma, got %d", len(groups[2].sessions))
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestPrintTableEmpty(t *testing.T) {
	out := captureStdout(t, func() { PrintTable(nil) })
	if !strings.Contains(out, "No sessions found") {
		t.Errorf("expected 'No sessions found', got %q", out)
	}
}

func TestPrintTableGrouped(t *testing.T) {
	sessions := []session.SessionMeta{
		{ShortID: "abc", Title: "first", Project: "/proj/a", Messages: 5, Modified: time.Now()},
		{ShortID: "def", Title: "second", Project: "/proj/b", Messages: 3, Modified: time.Now().Add(-time.Hour), IsActive: true},
	}
	out := captureStdout(t, func() { PrintTable(sessions) })
	if !strings.Contains(out, "/proj/a") || !strings.Contains(out, "/proj/b") {
		t.Errorf("expected project paths in output, got %q", out)
	}
	if !strings.Contains(out, "2 sessions across 2 projects") {
		t.Errorf("expected summary line, got %q", out)
	}
	if !strings.Contains(out, "(* = active)") {
		t.Errorf("expected active marker note, got %q", out)
	}
}

func TestPrintFzf(t *testing.T) {
	sessions := []session.SessionMeta{
		{ShortID: "abc12345", Title: "test session", Project: "/proj", Messages: 10, Modified: time.Now()},
	}
	out := captureStdout(t, func() { PrintFzf(sessions) })
	if !strings.Contains(out, "abc12345") {
		t.Errorf("expected shortID in output, got %q", out)
	}
	if !strings.Contains(out, "10 msgs") {
		t.Errorf("expected message count in output, got %q", out)
	}
}

func TestPrintJSON(t *testing.T) {
	sessions := []session.SessionMeta{
		{ShortID: "abc", Title: "test", Messages: 5},
	}
	out := captureStdout(t, func() { PrintJSON(sessions) })
	if !strings.Contains(out, `"short_id"`) || !strings.Contains(out, `"abc"`) {
		t.Errorf("expected JSON output with short_id, got %q", out)
	}
}

func TestPrintDetail(t *testing.T) {
	meta := &session.SessionMeta{
		ID: "abc-123", Title: "detail test", Project: "/home/test",
		Branch: "feat/x", Created: time.Now(), Modified: time.Now(),
		Messages: 10, FileSize: 1024, FilePath: "/tmp/test.jsonl", IsActive: true,
	}
	out := captureStdout(t, func() { PrintDetail(meta, []string{"prompt 1", "prompt 2"}) })
	if !strings.Contains(out, "detail test") {
		t.Errorf("expected title in output")
	}
	if !strings.Contains(out, "ACTIVE") {
		t.Errorf("expected ACTIVE status")
	}
	if !strings.Contains(out, "prompt 1") || !strings.Contains(out, "prompt 2") {
		t.Errorf("expected prompts in output")
	}
}

func TestPrintDeletePreview(t *testing.T) {
	sessions := []session.SessionMeta{
		{ShortID: "abc", Title: "deleteme", FileSize: 1024, Modified: time.Now()},
		{ShortID: "def", Title: "active", FileSize: 2048, Modified: time.Now(), IsActive: true},
	}
	out := captureStdout(t, func() { PrintDeletePreview(sessions) })
	if !strings.Contains(out, "ACTIVE - SKIPPED") {
		t.Errorf("expected active session skipped note")
	}
	if !strings.Contains(out, "1 sessions to delete") {
		t.Errorf("expected delete count, got %q", out)
	}
}

func TestPrintDeletePreviewEmpty(t *testing.T) {
	out := captureStdout(t, func() { PrintDeletePreview(nil) })
	if !strings.Contains(out, "No sessions matched") {
		t.Errorf("expected no match message")
	}
}

func TestPrintStats(t *testing.T) {
	stats := map[string]ProjectStats{
		"/proj/a": {Count: 5, Size: 1024 * 1024},
	}
	total := TotalStats{Sessions: 5, SessionSize: 1024 * 1024, TotalSize: 2 * 1024 * 1024}
	out := captureStdout(t, func() { PrintStats(stats, total) })
	if !strings.Contains(out, "/proj/a") {
		t.Errorf("expected project in stats output")
	}
	if !strings.Contains(out, "1.0 MB") {
		t.Errorf("expected formatted size")
	}
}

func TestFormatSizeExported(t *testing.T) {
	if FormatSize(1024) != "1.0 KB" {
		t.Errorf("FormatSize(1024) = %q, want %q", FormatSize(1024), "1.0 KB")
	}
}

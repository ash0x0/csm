package summarize

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ash0x0/csm/internal/session"
	"github.com/google/uuid"
)

// Options controls summarize behavior.
type Options struct {
	Model  string // claude model alias (default: "haiku")
	Print  bool   // print summary to stdout instead of writing a session
}

const defaultModel = "haiku"

const summarizePrompt = `Summarize this conversation concisely. Focus on: decisions made, code written/changed, key context needed to continue the work. Be specific — include file names, function names, and key facts. Format as plain prose, not bullet points.

Conversation transcript:

`

// Summarize reads a session, generates a summary via the claude CLI,
// and writes a new compact session JSONL. Returns the new session ID.
func Summarize(meta *session.SessionMeta, opts Options) (string, error) {
	if opts.Model == "" {
		opts.Model = defaultModel
	}

	timeline, err := session.ReadTimeline(meta.FilePath)
	if err != nil {
		return "", fmt.Errorf("reading timeline: %w", err)
	}
	if len(timeline) == 0 {
		return "", fmt.Errorf("session has no conversation events to summarize")
	}

	transcript := buildTranscript(timeline)
	summary, err := callClaude(transcript, opts.Model)
	if err != nil {
		return "", fmt.Errorf("calling claude: %w", err)
	}

	if opts.Print {
		fmt.Println(summary)
		return "", nil
	}

	newID, err := writeSession(meta, summary)
	if err != nil {
		return "", fmt.Errorf("writing summary session: %w", err)
	}
	return newID, nil
}

// buildTranscript converts timeline events into a readable transcript string.
func buildTranscript(events []session.TimelineEvent) string {
	var sb strings.Builder
	for _, ev := range events {
		switch ev.Type {
		case "user":
			sb.WriteString("User: ")
			sb.WriteString(ev.Summary)
			sb.WriteString("\n\n")
		case "assistant":
			sb.WriteString("Claude: ")
			sb.WriteString(ev.Summary)
			sb.WriteString("\n\n")
		case "compact":
			sb.WriteString("[Session was auto-compacted here]\n\n")
		}
	}
	return strings.TrimSpace(sb.String())
}

// callClaude runs `claude --print` with the transcript as stdin input.
func callClaude(transcript, model string) (string, error) {
	prompt := summarizePrompt + transcript

	cmd := exec.Command("claude",
		"--print",
		"--model", model,
		"--no-session-persistence",
		"--bare",
		prompt,
	)
	cmd.Stdin = bytes.NewBufferString("")

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("claude exited with %d: %s", exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// writeSession writes a new JSONL session containing the summary as a user message.
func writeSession(orig *session.SessionMeta, summary string) (string, error) {
	newID := uuid.New().String()
	title := "Summary: " + orig.Title

	outputDir := filepath.Dir(orig.FilePath)
	outputPath := filepath.Join(outputDir, newID+".jsonl")
	tmpPath := outputPath + ".tmp"

	f, err := os.Create(tmpPath)
	if err != nil {
		return "", err
	}

	succeeded := false
	defer func() {
		if f != nil {
			f.Close()
		}
		if !succeeded {
			os.Remove(tmpPath)
		}
	}()

	enc := json.NewEncoder(f)

	now := time.Now().UTC().Format(time.RFC3339)
	msgUUID := uuid.New().String()

	events := []map[string]any{
		{
			"type":        "custom-title",
			"customTitle": title,
			"sessionId":   newID,
		},
		{
			"type":      "agent-name",
			"agentName": title,
			"sessionId": newID,
		},
		{
			"type":      "user",
			"sessionId": newID,
			"uuid":      msgUUID,
			"timestamp": now,
			"message": map[string]any{
				"role": "user",
				"content": fmt.Sprintf(
					"[Session summary from %s — original session: %s]\n\n%s\n\nPlease continue from where we left off.",
					orig.Modified.Format("2006-01-02"),
					orig.ShortID,
					summary,
				),
			},
		},
	}

	for _, ev := range events {
		if err := enc.Encode(ev); err != nil {
			return "", err
		}
	}

	if err := f.Sync(); err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	f = nil
	if err := os.Rename(tmpPath, outputPath); err != nil {
		return "", err
	}
	succeeded = true

	return newID, nil
}

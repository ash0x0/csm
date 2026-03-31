package merge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ash0x0/csm/internal/session"
	"github.com/google/uuid"
)

// MergeOptions controls the merge behavior.
type MergeOptions struct {
	Title     string
	OutputDir string // project directory to write into
}

// MergeN creates a new session JSONL by concatenating all events from N sessions,
// chaining them via parentUuid so Claude Code can resume with complete history.
func MergeN(metas []*session.SessionMeta, opts MergeOptions) (string, error) {
	if len(metas) < 2 {
		return "", fmt.Errorf("need at least 2 sessions to merge, got %d", len(metas))
	}

	// Sort sessions chronologically by last Modified time (earliest first)
	sort.Slice(metas, func(i, j int) bool {
		return metas[i].Modified.Before(metas[j].Modified)
	})

	newID := uuid.New().String()

	title := opts.Title
	if title == "" {
		var titles []string
		for _, m := range metas {
			titles = append(titles, truncTitle(m.Title))
		}
		title = "Merged: " + strings.Join(titles, " + ")
	}

	outputDir := opts.OutputDir
	if outputDir == "" {
		outputDir = filepath.Dir(metas[0].FilePath)
	}
	outputPath := filepath.Join(outputDir, newID+".jsonl")

	f, err := os.Create(outputPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	enc := json.NewEncoder(f)

	// Write header events
	enc.Encode(map[string]any{
		"type":        "custom-title",
		"customTitle": title,
		"sessionId":   newID,
	})
	enc.Encode(map[string]any{
		"type":      "agent-name",
		"agentName": title,
		"sessionId": newID,
	})

	var lastUUID string // tracks the last uuid for chaining sessions

	for i, meta := range metas {
		events, err := session.ReadRawEvents(meta.FilePath)
		if err != nil {
			return "", fmt.Errorf("reading session %d (%s): %w", i+1, meta.ShortID, err)
		}

		firstLinked := false
		for _, ev := range events {
			// Rewrite sessionId on events that have it
			// file-history-snapshot events have no sessionId — leave them alone
			if _, has := ev["sessionId"]; has {
				ev["sessionId"] = newID
			}

			// Chain: rewrite the first uuid-bearing event of S2+ to point to previous session's last uuid
			if i > 0 && !firstLinked {
				if _, hasUUID := ev["uuid"]; hasUUID {
					ev["parentUuid"] = lastUUID
					firstLinked = true
				}
			}

			// Track the last uuid for chaining
			if u, ok := ev["uuid"].(string); ok && u != "" {
				lastUUID = u
			}

			if err := enc.Encode(ev); err != nil {
				return "", fmt.Errorf("writing event: %w", err)
			}
		}
	}

	return newID, nil
}

func truncTitle(s string) string {
	if len(s) > 30 {
		return s[:27] + "..."
	}
	return s
}

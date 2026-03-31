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

// MergeN creates a new session JSONL by git-style merging N sessions.
// It finds common event prefixes (by UUID) and keeps shared history once,
// merging only divergent tails. Falls back to concatenation when sessions
// share no common prefix.
//
// Returns the new session ID and merge statistics.
func MergeN(metas []*session.SessionMeta, opts MergeOptions) (string, error) {
	if len(metas) < 2 {
		return "", fmt.Errorf("need at least 2 sessions to merge, got %d", len(metas))
	}

	if err := validateNoDuplicates(metas); err != nil {
		return "", err
	}

	// Sort sessions chronologically by last Modified time (earliest first)
	sort.Slice(metas, func(i, j int) bool {
		return metas[i].Modified.Before(metas[j].Modified)
	})

	// Load first session's events as the accumulator
	accumulated, err := session.ReadRawEvents(metas[0].FilePath)
	if err != nil {
		return "", fmt.Errorf("reading session 1 (%s): %w", metas[0].ShortID, err)
	}

	var lastStats mergeStats

	// Pairwise merge each subsequent session
	for i := 1; i < len(metas); i++ {
		next, err := session.ReadRawEvents(metas[i].FilePath)
		if err != nil {
			return "", fmt.Errorf("reading session %d (%s): %w", i+1, metas[i].ShortID, err)
		}

		merged, stats, err := merge2Events(accumulated, next)
		if err != nil {
			return "", fmt.Errorf("merging session %d (%s): %w", i+1, metas[i].ShortID, err)
		}
		accumulated = merged
		lastStats = stats
	}

	// Generate new session ID and title
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

	// Write merged events, rewriting sessionId
	for _, ev := range accumulated {
		if _, has := ev["sessionId"]; has {
			ev["sessionId"] = newID
		}
		if err := enc.Encode(ev); err != nil {
			return "", fmt.Errorf("writing event: %w", err)
		}
	}

	// Print merge stats to stderr
	fmt.Fprintf(os.Stderr, "Merge strategy: %s | shared: %d, branch-A: %d, branch-B: %d\n",
		lastStats.Strategy, lastStats.CommonCount, lastStats.BranchAOnly, lastStats.BranchBOnly)

	return newID, nil
}

func validateNoDuplicates(metas []*session.SessionMeta) error {
	seen := make(map[string]bool)
	for _, m := range metas {
		if seen[m.ID] {
			return fmt.Errorf("session %s appears more than once; cannot merge a session with itself", m.ShortID)
		}
		seen[m.ID] = true
	}
	return nil
}

func truncTitle(s string) string {
	if len(s) > 30 {
		return s[:27] + "..."
	}
	return s
}

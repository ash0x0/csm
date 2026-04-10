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
	accumulated, skipped, err := session.ReadRawEventsWithStats(metas[0].FilePath)
	if err != nil {
		return "", fmt.Errorf("reading session 1 (%s): %w", metas[0].ShortID, err)
	}
	if skipped > 0 {
		fmt.Fprintf(os.Stderr, "warning: session 1 (%s): skipped %d corrupt lines\n", metas[0].ShortID, skipped)
	}

	// Accumulate stats across pairwise merges
	totalStats := mergeStats{}

	for i := 1; i < len(metas); i++ {
		next, sk, err := session.ReadRawEventsWithStats(metas[i].FilePath)
		if err != nil {
			return "", fmt.Errorf("reading session %d (%s): %w", i+1, metas[i].ShortID, err)
		}
		if sk > 0 {
			fmt.Fprintf(os.Stderr, "warning: session %d (%s): skipped %d corrupt lines\n", i+1, metas[i].ShortID, sk)
		}

		merged, stats, err := merge2Events(accumulated, next)
		if err != nil {
			if stats.Strategy == "identical" {
				fmt.Fprintf(os.Stderr, "skipping session %d (%s): identical to accumulated result\n", i+1, metas[i].ShortID)
				continue
			}
			return "", fmt.Errorf("merging session %d (%s): %w", i+1, metas[i].ShortID, err)
		}
		accumulated = merged
		totalStats.CommonCount += stats.CommonCount
		totalStats.BranchAOnly += stats.BranchAOnly
		totalStats.BranchBOnly += stats.BranchBOnly
		totalStats.Strategy = stats.Strategy // last strategy wins for display
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

	// Atomic write: write to temp file, rename on success
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

	// Write header events
	if err := enc.Encode(map[string]any{
		"type":        "custom-title",
		"customTitle": title,
		"sessionId":   newID,
	}); err != nil {
		return "", fmt.Errorf("writing custom-title header: %w", err)
	}
	if err := enc.Encode(map[string]any{
		"type":      "agent-name",
		"agentName": title,
		"sessionId": newID,
	}); err != nil {
		return "", fmt.Errorf("writing agent-name header: %w", err)
	}

	// Write merged events, rewriting sessionId
	for _, ev := range accumulated {
		if _, has := ev["sessionId"]; has {
			ev["sessionId"] = newID
		}
		if err := enc.Encode(ev); err != nil {
			return "", fmt.Errorf("writing event: %w", err)
		}
	}

	// Flush and rename
	if err := f.Sync(); err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	f = nil // prevent double-close in defer
	if err := os.Rename(tmpPath, outputPath); err != nil {
		return "", err
	}
	succeeded = true

	// Print merge stats to stderr
	fmt.Fprintf(os.Stderr, "Merge strategy: %s | shared: %d, branch-A: %d, branch-B: %d\n",
		totalStats.Strategy, totalStats.CommonCount, totalStats.BranchAOnly, totalStats.BranchBOnly)

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

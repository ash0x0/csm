package merge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ash0x0/csm/internal/session"
	"github.com/google/uuid"
)

// MergeOptions controls the merge behavior.
type MergeOptions struct {
	Title     string
	OutputDir string // project directory to write into
	DryRun    bool   // skip writing output file
}

// MergeReport describes the outcome of a merge operation.
type MergeReport struct {
	Strategy    string
	SharedCount int
	BranchAOnly int
	BranchBOnly int
	TotalEvents int
}

// MergeN creates a new session JSONL by git-style merging N sessions.
// It finds common event prefixes (by UUID) and keeps shared history once,
// merging only divergent tails. Falls back to concatenation when sessions
// share no common prefix.
//
// Returns the new session ID, a merge report, and any error.
func MergeN(metas []*session.SessionMeta, opts MergeOptions) (string, MergeReport, error) {
	if len(metas) < 2 {
		return "", MergeReport{}, fmt.Errorf("need at least 2 sessions to merge, got %d", len(metas))
	}

	if err := validateNoDuplicates(metas); err != nil {
		return "", MergeReport{}, err
	}

	// Warn about compacted sessions before merging
	for _, meta := range metas {
		events, _, _ := session.ReadRawEventsWithStats(meta.FilePath)
		for _, ev := range events {
			if ev["type"] == "system" {
				if sub, _ := ev["subtype"].(string); sub == "compact_boundary" {
					fmt.Fprintf(os.Stderr,
						"warning: session %s has been auto-compacted — pre-compaction history is lost; merge quality may be reduced\n",
						meta.ShortID)
					break
				}
			}
		}
	}

	// Sort sessions chronologically by first uuid-bearing event timestamp.
	// Fall back to file mtime if no timestamped events are found.
	sort.Slice(metas, func(i, j int) bool {
		ti := firstEventTime(metas[i].FilePath)
		if ti.IsZero() {
			ti = metas[i].Modified
		}
		tj := firstEventTime(metas[j].FilePath)
		if tj.IsZero() {
			tj = metas[j].Modified
		}
		return ti.Before(tj)
	})

	// Load first session's events as the accumulator
	accumulated, skipped, err := session.ReadRawEventsWithStats(metas[0].FilePath)
	if err != nil {
		return "", MergeReport{}, fmt.Errorf("reading session 1 (%s): %w", metas[0].ShortID, err)
	}
	if skipped > 0 {
		fmt.Fprintf(os.Stderr, "warning: session 1 (%s): skipped %d corrupt lines\n", metas[0].ShortID, skipped)
	}

	// Accumulate stats across pairwise merges
	totalStats := mergeStats{}

	for i := 1; i < len(metas); i++ {
		next, sk, err := session.ReadRawEventsWithStats(metas[i].FilePath)
		if err != nil {
			return "", MergeReport{}, fmt.Errorf("reading session %d (%s): %w", i+1, metas[i].ShortID, err)
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
			return "", MergeReport{}, fmt.Errorf("merging session %d (%s): %w", i+1, metas[i].ShortID, err)
		}
		accumulated = merged
		totalStats.CommonCount += stats.CommonCount
		totalStats.BranchAOnly += stats.BranchAOnly
		totalStats.BranchBOnly += stats.BranchBOnly
		totalStats.Strategy = stats.Strategy // last strategy wins for display
	}

	// Count uuid-bearing events in the final output
	totalUUID := 0
	for _, ev := range accumulated {
		if u, _ := ev["uuid"].(string); u != "" {
			totalUUID++
		}
	}

	report := MergeReport{
		Strategy:    totalStats.Strategy,
		SharedCount: totalStats.CommonCount,
		BranchAOnly: totalStats.BranchAOnly,
		BranchBOnly: totalStats.BranchBOnly,
		TotalEvents: totalUUID,
	}

	// Print merge stats to stderr
	fmt.Fprintf(os.Stderr, "Merge strategy: %s | shared: %d, branch-A: %d, branch-B: %d\n",
		totalStats.Strategy, totalStats.CommonCount, totalStats.BranchAOnly, totalStats.BranchBOnly)

	if opts.DryRun {
		return "", report, nil
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
		return "", report, err
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
		return "", report, fmt.Errorf("writing custom-title header: %w", err)
	}
	if err := enc.Encode(map[string]any{
		"type":      "agent-name",
		"agentName": title,
		"sessionId": newID,
	}); err != nil {
		return "", report, fmt.Errorf("writing agent-name header: %w", err)
	}

	// Write merged events, rewriting sessionId
	for _, ev := range accumulated {
		if _, has := ev["sessionId"]; has {
			ev["sessionId"] = newID
		}
		if err := enc.Encode(ev); err != nil {
			return "", report, fmt.Errorf("writing event: %w", err)
		}
	}

	// Flush and rename
	if err := f.Sync(); err != nil {
		return "", report, err
	}
	if err := f.Close(); err != nil {
		return "", report, err
	}
	f = nil // prevent double-close in defer
	if err := os.Rename(tmpPath, outputPath); err != nil {
		return "", report, err
	}
	succeeded = true

	return newID, report, nil
}

// firstEventTime returns the timestamp of the first uuid-bearing event in a session file.
func firstEventTime(filePath string) time.Time {
	events, _, _ := session.ReadRawEventsWithStats(filePath)
	for _, ev := range events {
		if _, hasUUID := ev["uuid"]; hasUUID {
			if ts, _ := ev["timestamp"].(string); ts != "" {
				if t, err := time.Parse(time.RFC3339, ts); err == nil {
					return t
				}
				if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
					return t
				}
			}
		}
	}
	return time.Time{}
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

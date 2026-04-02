package merge

import (
	"fmt"
	"sort"
	"time"
)

// mergeStats reports the outcome of a 2-way merge.
type mergeStats struct {
	CommonCount int
	BranchAOnly int
	BranchBOnly int
	Strategy    string // "git-merge", "prefix-extend", "concat", "identical"
}

// splitEvents classifies events into uuid-bearing and metadata-only.
func splitEvents(events []map[string]any) (uuidBearing, metadata []map[string]any) {
	for _, ev := range events {
		if _, has := ev["uuid"]; has {
			uuidBearing = append(uuidBearing, ev)
		} else {
			metadata = append(metadata, ev)
		}
	}
	return
}

// alignment describes a contiguous block of shared UUIDs between two event lists.
type alignment struct {
	OffsetA int // where the shared block starts in A
	OffsetB int // where the shared block starts in B
	Length  int // number of shared events
}

// findAlignment finds the longest contiguous block of matching UUIDs between
// two event lists. Unlike a simple prefix check, this handles cases where one
// session has extra events prepended before the shared history.
//
// Algorithm: index the shorter list's UUIDs by value, then for each UUID in
// the longer list, check if it starts a contiguous run.
func findAlignment(eventsA, eventsB []map[string]any) alignment {
	if len(eventsA) == 0 || len(eventsB) == 0 {
		return alignment{}
	}

	// Build UUID index for A (maps uuid -> position)
	indexA := make(map[string]int, len(eventsA))
	for i, ev := range eventsA {
		if u, _ := ev["uuid"].(string); u != "" {
			if _, exists := indexA[u]; !exists {
				indexA[u] = i // keep first occurrence
			}
		}
	}

	// Walk B looking for the start of a long contiguous match
	best := alignment{}
	for j := 0; j < len(eventsB); j++ {
		uB, _ := eventsB[j]["uuid"].(string)
		if uB == "" {
			continue
		}
		iA, found := indexA[uB]
		if !found {
			continue
		}

		// Count consecutive matches from this point
		matchLen := 0
		for iA+matchLen < len(eventsA) && j+matchLen < len(eventsB) {
			uA, _ := eventsA[iA+matchLen]["uuid"].(string)
			uB2, _ := eventsB[j+matchLen]["uuid"].(string)
			if uA == "" || uB2 == "" || uA != uB2 {
				break
			}
			matchLen++
		}

		if matchLen > best.Length {
			best = alignment{OffsetA: iA, OffsetB: j, Length: matchLen}
			// If we matched everything in the shorter list, can't do better
			if matchLen == len(eventsA) || matchLen == len(eventsB) {
				break
			}
		}

		// Skip past this match to avoid redundant checks
		j += matchLen - 1
	}

	return best
}

// findCommonPrefix is a convenience wrapper for backward compatibility.
// Returns the prefix match length (only when both start at offset 0).
func findCommonPrefix(eventsA, eventsB []map[string]any) int {
	a := findAlignment(eventsA, eventsB)
	if a.OffsetA == 0 && a.OffsetB == 0 {
		return a.Length
	}
	return 0
}

// rechainEvents rewrites parentUuid on conversation events (user, assistant,
// system) to form a linear chain starting from anchorUUID. Progress and
// queue-operation events form parallel sub-chains and are left untouched —
// linearizing them would destroy the conversation tree that Claude Code walks.
func rechainEvents(events []map[string]any, anchorUUID string) string {
	last := anchorUUID
	for _, ev := range events {
		typ, _ := ev["type"].(string)
		if typ != "user" && typ != "assistant" && typ != "system" {
			continue
		}
		if u, ok := ev["uuid"].(string); ok && u != "" {
			ev["parentUuid"] = last
			last = u
		}
	}
	return last
}

// deduplicateMetadata merges metadata events from two sessions.
// - custom-title/agent-name: discarded (caller writes fresh headers)
// - file-history-snapshot: deduplicated by messageId
// - queue-operation: dropped (per-session runtime state, meaningless after merge)
// - last-prompt: only the last one is kept
func deduplicateMetadata(metaA, metaB []map[string]any) []map[string]any {
	seen := make(map[string]bool)
	var out []map[string]any
	var lastPromptEvent map[string]any

	add := func(events []map[string]any) {
		for _, ev := range events {
			typ, _ := ev["type"].(string)
			if typ == "custom-title" || typ == "agent-name" {
				continue
			}
			if typ == "queue-operation" {
				continue
			}
			if typ == "last-prompt" {
				lastPromptEvent = ev
				continue
			}
			if typ == "file-history-snapshot" {
				mid, _ := ev["messageId"].(string)
				if mid != "" {
					if seen[mid] {
						continue
					}
					seen[mid] = true
				}
			}
			out = append(out, ev)
		}
	}

	add(metaA)
	add(metaB)
	if lastPromptEvent != nil {
		out = append(out, lastPromptEvent)
	}
	return out
}

// parseTimestamp extracts and parses a timestamp from an event.
func parseTimestamp(ev map[string]any) time.Time {
	ts, ok := ev["timestamp"].(string)
	if !ok {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		t, _ = time.Parse(time.RFC3339, ts)
	}
	return t
}

// DiffResult describes the relationship between two sessions.
type DiffResult struct {
	Relationship string `json:"relationship"` // "identical", "a-contains-b", "b-contains-a", "diverged", "unrelated"
	CommonCount  int    `json:"commonCount"`
	OnlyACount   int    `json:"onlyACount"`
	OnlyBCount   int    `json:"onlyBCount"`
}

// Diff2 compares two sessions' events by UUID and returns their relationship.
// It finds the longest contiguous shared block, which may not start at position 0
// in either session.
func Diff2(eventsA, eventsB []map[string]any) DiffResult {
	uuidA, _ := splitEvents(eventsA)
	uuidB, _ := splitEvents(eventsB)

	a := findAlignment(uuidA, uuidB)
	lenA := len(uuidA)
	lenB := len(uuidB)

	r := DiffResult{
		CommonCount: a.Length,
		OnlyACount:  lenA - a.Length,
		OnlyBCount:  lenB - a.Length,
	}

	switch {
	case a.Length == 0:
		r.Relationship = "unrelated"
	case a.Length == lenA && a.Length == lenB:
		r.Relationship = "identical"
	case a.Length == lenA && lenB > lenA:
		r.Relationship = "b-contains-a"
	case a.Length == lenB && lenA > lenB:
		r.Relationship = "a-contains-b"
	default:
		r.Relationship = "diverged"
	}

	return r
}

// merge2Events performs an in-memory git-style 2-way merge of two event lists.
// It finds the longest contiguous shared block (by UUID), keeps it once, and
// merges the unique segments from both sides by timestamp.
func merge2Events(eventsA, eventsB []map[string]any) ([]map[string]any, mergeStats, error) {
	uuidA, metaA := splitEvents(eventsA)
	uuidB, metaB := splitEvents(eventsB)

	a := findAlignment(uuidA, uuidB)

	// Identical sessions
	if a.Length == len(uuidA) && a.Length == len(uuidB) && a.Length > 0 {
		return nil, mergeStats{Strategy: "identical", CommonCount: a.Length}, fmt.Errorf("sessions are identical (%d shared events, 0 divergent)", a.Length)
	}

	onlyA := len(uuidA) - a.Length
	onlyB := len(uuidB) - a.Length
	stats := mergeStats{CommonCount: a.Length}

	// One fully contains the other
	if a.Length == len(uuidA) && len(uuidB) > a.Length {
		stats.Strategy = "prefix-extend"
		stats.BranchBOnly = onlyB
		dedupMeta := deduplicateMetadata(metaA, metaB)
		result := append(dedupMeta, uuidB...)
		rechainEvents(result, "")
		return result, stats, nil
	}
	if a.Length == len(uuidB) && len(uuidA) > a.Length {
		stats.Strategy = "prefix-extend"
		stats.BranchAOnly = onlyA
		dedupMeta := deduplicateMetadata(metaA, metaB)
		result := append(dedupMeta, uuidA...)
		rechainEvents(result, "")
		return result, stats, nil
	}

	// No shared events — concatenate
	if a.Length == 0 {
		stats.Strategy = "concat"
		stats.BranchAOnly = len(uuidA)
		stats.BranchBOnly = len(uuidB)
		dedupMeta := deduplicateMetadata(metaA, metaB)
		all := make([]map[string]any, 0, len(dedupMeta)+len(uuidA)+len(uuidB))
		all = append(all, dedupMeta...)
		all = append(all, uuidA...)
		all = append(all, uuidB...)
		// Rechain ALL uuid events linearly so Claude Code can walk the
		// entire conversation from last event to first. Sessions may have
		// internal branching (tree-shaped parentUuid) which would leave
		// earlier events unreachable if not linearized.
		rechainEvents(all, "")
		return all, stats, nil
	}

	// Git-style merge: shared block + unique segments from both sides
	stats.Strategy = "git-merge"
	stats.BranchAOnly = onlyA
	stats.BranchBOnly = onlyB

	// The shared block from A (identical to the corresponding block in B)
	sharedBlock := uuidA[a.OffsetA : a.OffsetA+a.Length]

	// Collect unique events from both sessions (before and after the shared block)
	var divergent []map[string]any
	divergent = append(divergent, uuidA[:a.OffsetA]...)                // A before shared
	divergent = append(divergent, uuidA[a.OffsetA+a.Length:]...)       // A after shared
	divergent = append(divergent, uuidB[:a.OffsetB]...)                // B before shared
	divergent = append(divergent, uuidB[a.OffsetB+a.Length:]...)       // B after shared

	sort.SliceStable(divergent, func(i, j int) bool {
		return parseTimestamp(divergent[i]).Before(parseTimestamp(divergent[j]))
	})

	dedupMeta := deduplicateMetadata(metaA, metaB)

	result := make([]map[string]any, 0, len(dedupMeta)+len(sharedBlock)+len(divergent))
	result = append(result, dedupMeta...)
	result = append(result, sharedBlock...)
	result = append(result, divergent...)

	// Rechain ALL uuid events linearly so Claude Code can walk the
	// entire conversation from last event to first.
	rechainEvents(result, "")

	return result, stats, nil
}

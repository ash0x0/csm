package merge

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/ash0x0/csm/internal/session"
	"github.com/ash0x0/csm/internal/testutil"
	"github.com/google/uuid"
)

func TestFindCommonPrefix(t *testing.T) {
	mkEv := func(uid string) map[string]any {
		return map[string]any{"uuid": uid, "type": "user"}
	}

	tests := []struct {
		name string
		a, b []map[string]any
		want int
	}{
		{"both empty", nil, nil, 0},
		{"a empty", nil, []map[string]any{mkEv("x")}, 0},
		{"b empty", []map[string]any{mkEv("x")}, nil, 0},
		{"no match", []map[string]any{mkEv("a")}, []map[string]any{mkEv("b")}, 0},
		{"full match", []map[string]any{mkEv("a"), mkEv("b")}, []map[string]any{mkEv("a"), mkEv("b")}, 2},
		{"partial match", []map[string]any{mkEv("a"), mkEv("b"), mkEv("c")}, []map[string]any{mkEv("a"), mkEv("b"), mkEv("d")}, 2},
		{"one common", []map[string]any{mkEv("x"), mkEv("y")}, []map[string]any{mkEv("x"), mkEv("z")}, 1},
		{"a shorter", []map[string]any{mkEv("a")}, []map[string]any{mkEv("a"), mkEv("b")}, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := findCommonPrefix(tc.a, tc.b)
			if got != tc.want {
				t.Errorf("findCommonPrefix = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestSplitEvents(t *testing.T) {
	events := []map[string]any{
		{"type": "custom-title", "customTitle": "T"},
		{"type": "system", "uuid": "u1"},
		{"type": "agent-name", "agentName": "A"},
		{"type": "user", "uuid": "u2"},
		{"type": "file-history-snapshot", "messageId": "m1"},
	}

	uuidBearing, metadata := splitEvents(events)

	if len(uuidBearing) != 2 {
		t.Errorf("uuid-bearing count = %d, want 2", len(uuidBearing))
	}
	if len(metadata) != 3 {
		t.Errorf("metadata count = %d, want 3", len(metadata))
	}
}

func TestRechainEvents(t *testing.T) {
	events := []map[string]any{
		{"uuid": "a", "parentUuid": "old1"},
		{"uuid": "b", "parentUuid": "old2"},
		{"uuid": "c", "parentUuid": "old3"},
	}

	last := rechainEvents(events, "anchor")

	if last != "c" {
		t.Errorf("last uuid = %q, want %q", last, "c")
	}
	if events[0]["parentUuid"] != "anchor" {
		t.Errorf("event 0 parentUuid = %q, want %q", events[0]["parentUuid"], "anchor")
	}
	if events[1]["parentUuid"] != "a" {
		t.Errorf("event 1 parentUuid = %q, want %q", events[1]["parentUuid"], "a")
	}
	if events[2]["parentUuid"] != "b" {
		t.Errorf("event 2 parentUuid = %q, want %q", events[2]["parentUuid"], "b")
	}
}

func TestDeduplicateMetadata(t *testing.T) {
	metaA := []map[string]any{
		{"type": "custom-title", "customTitle": "Title A"},
		{"type": "agent-name", "agentName": "Agent A"},
		{"type": "file-history-snapshot", "messageId": "m1"},
		{"type": "file-history-snapshot", "messageId": "m2"},
	}
	metaB := []map[string]any{
		{"type": "custom-title", "customTitle": "Title B"},
		{"type": "agent-name", "agentName": "Agent B"},
		{"type": "file-history-snapshot", "messageId": "m1"}, // duplicate
		{"type": "file-history-snapshot", "messageId": "m3"},
	}

	result := deduplicateMetadata(metaA, metaB)

	// Should have 3 file-history-snapshot (m1, m2, m3), no custom-title/agent-name
	snapshots := 0
	for _, ev := range result {
		typ, _ := ev["type"].(string)
		if typ == "custom-title" || typ == "agent-name" {
			t.Errorf("unexpected metadata event type %q in output", typ)
		}
		if typ == "file-history-snapshot" {
			snapshots++
		}
	}
	if snapshots != 3 {
		t.Errorf("snapshot count = %d, want 3", snapshots)
	}
}

func TestMerge2GitMerge(t *testing.T) {
	idA := uuid.New().String()
	idB := uuid.New().String()

	eventsA, eventsB := testutil.BuildForkedSessions(idA, idB, "Branch A", "Branch B", 3, 2, 2, "2026-03-20T10:00:00Z")

	merged, stats, err := merge2Events(eventsA, eventsB)
	if err != nil {
		t.Fatalf("merge2Events: %v", err)
	}

	if stats.Strategy != "git-merge" {
		t.Errorf("strategy = %q, want %q", stats.Strategy, "git-merge")
	}
	if stats.CommonCount != 7 { // 1 init + 3 turns * 2 events
		t.Errorf("common count = %d, want 7", stats.CommonCount)
	}
	if stats.BranchAOnly != 4 { // 2 turns * 2 events
		t.Errorf("branch A only = %d, want 4", stats.BranchAOnly)
	}
	if stats.BranchBOnly != 4 {
		t.Errorf("branch B only = %d, want 4", stats.BranchBOnly)
	}

	// Count uuid-bearing events in merged output
	uuidCount := 0
	for _, ev := range merged {
		if _, has := ev["uuid"]; has {
			uuidCount++
		}
	}
	// Should be 7 (common) + 4 (A) + 4 (B) = 15
	if uuidCount != 15 {
		t.Errorf("uuid-bearing event count = %d, want 15", uuidCount)
	}
}

func TestMerge2NoCommonPrefix(t *testing.T) {
	idA := uuid.New().String()
	idB := uuid.New().String()

	// Build completely independent sessions (no shared UUIDs)
	eventsA := testutil.BuildSimpleSession(idA, "A", "main", 2, "2026-03-20T10:00:00Z")
	eventsB := testutil.BuildSimpleSession(idB, "B", "main", 3, "2026-03-20T11:00:00Z")

	_, stats, err := merge2Events(eventsA, eventsB)
	if err != nil {
		t.Fatalf("merge2Events: %v", err)
	}

	if stats.Strategy != "concat" {
		t.Errorf("strategy = %q, want %q", stats.Strategy, "concat")
	}
}

func TestMerge2IdenticalSessions(t *testing.T) {
	idA := uuid.New().String()
	idB := uuid.New().String()

	// Same UUIDs, no extra turns on either side
	eventsA, eventsB := testutil.BuildForkedSessions(idA, idB, "A", "B", 3, 0, 0, "2026-03-20T10:00:00Z")

	_, stats, err := merge2Events(eventsA, eventsB)
	if err == nil {
		t.Fatal("expected error for identical sessions")
	}
	if stats.Strategy != "identical" {
		t.Errorf("strategy = %q, want %q", stats.Strategy, "identical")
	}
}

func TestMerge2SelfMerge(t *testing.T) {
	id := uuid.New().String()
	meta := &session.SessionMeta{ID: id, ShortID: id[:8]}

	err := validateNoDuplicates([]*session.SessionMeta{meta, meta})
	if err == nil {
		t.Error("expected error for duplicate session ID")
	}
}

func TestMerge2PrefixExtend(t *testing.T) {
	idA := uuid.New().String()
	idB := uuid.New().String()

	// A has 3 shared turns + 0 extra, B has 3 shared + 2 extra → A is prefix of B
	eventsA, eventsB := testutil.BuildForkedSessions(idA, idB, "Short", "Long", 3, 0, 2, "2026-03-20T10:00:00Z")

	merged, stats, err := merge2Events(eventsA, eventsB)
	if err != nil {
		t.Fatalf("merge2Events: %v", err)
	}

	if stats.Strategy != "prefix-extend" {
		t.Errorf("strategy = %q, want %q", stats.Strategy, "prefix-extend")
	}

	// Result should contain all of B's uuid events
	uuidCount := 0
	for _, ev := range merged {
		if _, has := ev["uuid"]; has {
			uuidCount++
		}
	}
	// 7 common (1 init + 3*2) + 4 extra from B = 11
	if uuidCount != 11 {
		t.Errorf("uuid count = %d, want 11", uuidCount)
	}
}

func TestMerge2ParentUUIDChainValid(t *testing.T) {
	idA := uuid.New().String()
	idB := uuid.New().String()

	eventsA, eventsB := testutil.BuildForkedSessions(idA, idB, "A", "B", 3, 2, 2, "2026-03-20T10:00:00Z")

	merged, _, err := merge2Events(eventsA, eventsB)
	if err != nil {
		t.Fatalf("merge2Events: %v", err)
	}

	// Extract uuid-bearing events and verify the parentUuid chain is linear
	var chain []map[string]any
	for _, ev := range merged {
		if _, has := ev["uuid"]; has {
			chain = append(chain, ev)
		}
	}

	for i := 1; i < len(chain); i++ {
		prevUUID, _ := chain[i-1]["uuid"].(string)
		parentUUID, _ := chain[i]["parentUuid"].(string)
		if parentUUID != prevUUID {
			t.Errorf("event %d: parentUuid = %q, want %q (previous uuid)", i, parentUUID, prevUUID)
		}
	}
}

func TestMerge2SessionIdRewrite(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)
	projDir := testutil.CreateProject(t, claudeDir, "/home/user/proj")

	idA := uuid.New().String()
	idB := uuid.New().String()

	eventsA, eventsB := testutil.BuildForkedSessions(idA, idB, "A", "B", 2, 1, 1, "2026-03-20T10:00:00Z")
	fpA := testutil.WriteSession(t, projDir, idA, eventsA)
	fpB := testutil.WriteSession(t, projDir, idB, eventsB)

	base := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)
	metaA := &session.SessionMeta{ID: idA, ShortID: idA[:8], Title: "A", FilePath: fpA, Modified: base}
	metaB := &session.SessionMeta{ID: idB, ShortID: idB[:8], Title: "B", FilePath: fpB, Modified: base.Add(time.Hour)}

	newID, err := MergeN([]*session.SessionMeta{metaA, metaB}, MergeOptions{OutputDir: projDir})
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

func TestMerge2PreservesFields(t *testing.T) {
	idA := uuid.New().String()
	idB := uuid.New().String()

	eventsA, eventsB := testutil.BuildForkedSessions(idA, idB, "A", "B", 1, 1, 1, "2026-03-20T10:00:00Z")

	// Add a custom field to a shared event
	for _, ev := range eventsA {
		if ev["type"] == "system" {
			ev["cwd"] = "/home/user/project"
			ev["version"] = "1.0.42"
			break
		}
	}
	// Same shared event in B should also have it (same uuid)
	for _, ev := range eventsB {
		if ev["type"] == "system" {
			ev["cwd"] = "/home/user/project"
			ev["version"] = "1.0.42"
			break
		}
	}

	merged, _, err := merge2Events(eventsA, eventsB)
	if err != nil {
		t.Fatalf("merge2Events: %v", err)
	}

	for _, ev := range merged {
		if ev["type"] == "system" && ev["cwd"] != nil {
			if ev["cwd"] != "/home/user/project" {
				t.Errorf("cwd = %v, want /home/user/project", ev["cwd"])
			}
			if ev["version"] != "1.0.42" {
				t.Errorf("version = %v, want 1.0.42", ev["version"])
			}
			return
		}
	}
	t.Error("system event with custom fields not found")
}

func TestMerge2FileHistorySnapshot(t *testing.T) {
	idA := uuid.New().String()
	idB := uuid.New().String()

	eventsA, eventsB := testutil.BuildForkedSessions(idA, idB, "A", "B", 2, 1, 1, "2026-03-20T10:00:00Z")

	// Add shared file-history-snapshot to both
	snap := testutil.MakeFileHistorySnapshot("msg-shared")
	eventsA = append(eventsA, snap)
	eventsB = append(eventsB, testutil.MakeFileHistorySnapshot("msg-shared")) // same messageId

	// Add unique snapshot to B only
	eventsB = append(eventsB, testutil.MakeFileHistorySnapshot("msg-unique"))

	merged, _, err := merge2Events(eventsA, eventsB)
	if err != nil {
		t.Fatalf("merge2Events: %v", err)
	}

	snapCount := 0
	for _, ev := range merged {
		if ev["type"] == "file-history-snapshot" {
			snapCount++
			if _, has := ev["sessionId"]; has {
				t.Error("file-history-snapshot should not have sessionId")
			}
		}
	}
	if snapCount != 2 { // msg-shared (deduped) + msg-unique
		t.Errorf("snapshot count = %d, want 2", snapCount)
	}
}

func TestMerge2TimestampOrdering(t *testing.T) {
	idA := uuid.New().String()
	idB := uuid.New().String()

	// Build sessions with shared prefix, then divergent events at different times
	eventsA, eventsB := testutil.BuildForkedSessions(idA, idB, "A", "B", 1, 0, 0, "2026-03-20T10:00:00Z")

	// Manually add divergent events with specific timestamps
	// A's event at T+20min, B's event at T+10min
	eventsA = append(eventsA, testutil.MakeUserMessageWithUUID(idA, "", uuid.New().String(), "A-late", "2026-03-20T10:20:00Z"))
	eventsB = append(eventsB, testutil.MakeUserMessageWithUUID(idB, "", uuid.New().String(), "B-early", "2026-03-20T10:10:00Z"))

	merged, stats, err := merge2Events(eventsA, eventsB)
	if err != nil {
		t.Fatalf("merge2Events: %v", err)
	}

	if stats.Strategy != "git-merge" {
		t.Errorf("strategy = %q, want %q", stats.Strategy, "git-merge")
	}

	// Find the two divergent user events and verify B-early comes before A-late
	var divergentTexts []string
	for _, ev := range merged {
		if ev["type"] == "user" {
			if msg, ok := ev["message"].(map[string]any); ok {
				if content, ok := msg["content"].(string); ok {
					if content == "B-early" || content == "A-late" {
						divergentTexts = append(divergentTexts, content)
					}
				}
			}
		}
	}

	if len(divergentTexts) != 2 {
		t.Fatalf("expected 2 divergent texts, got %d: %v", len(divergentTexts), divergentTexts)
	}
	if divergentTexts[0] != "B-early" {
		t.Errorf("first divergent = %q, want %q (earlier timestamp)", divergentTexts[0], "B-early")
	}
	if divergentTexts[1] != "A-late" {
		t.Errorf("second divergent = %q, want %q (later timestamp)", divergentTexts[1], "A-late")
	}
}

func TestMergeNThreeSessions(t *testing.T) {
	claudeDir := testutil.CreateTempClaudeDir(t)
	projDir := testutil.CreateProject(t, claudeDir, "/home/user/proj")

	idA := uuid.New().String()
	idB := uuid.New().String()
	idC := uuid.New().String()

	// A and B share 3 turns, each has 1 extra
	eventsA, eventsB := testutil.BuildForkedSessions(idA, idB, "A", "B", 3, 1, 1, "2026-03-20T10:00:00Z")

	// C is independent (no shared UUIDs)
	eventsC := testutil.BuildSimpleSession(idC, "C", "main", 2, "2026-03-20T12:00:00Z")

	fpA := testutil.WriteSession(t, projDir, idA, eventsA)
	fpB := testutil.WriteSession(t, projDir, idB, eventsB)
	fpC := testutil.WriteSession(t, projDir, idC, eventsC)

	base := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)
	metas := []*session.SessionMeta{
		{ID: idA, ShortID: idA[:8], Title: "A", FilePath: fpA, Modified: base},
		{ID: idB, ShortID: idB[:8], Title: "B", FilePath: fpB, Modified: base.Add(time.Hour)},
		{ID: idC, ShortID: idC[:8], Title: "C", FilePath: fpC, Modified: base.Add(2 * time.Hour)},
	}

	newID, err := MergeN(metas, MergeOptions{OutputDir: projDir, Title: "Three-way"})
	if err != nil {
		t.Fatalf("MergeN: %v", err)
	}

	events := readBackRaw(t, projDir, newID)

	// Verify output file exists and has events
	outputPath := filepath.Join(projDir, newID+".jsonl")
	if _, err := session.ReadRawEvents(outputPath); err != nil {
		t.Fatalf("reading merged file: %v", err)
	}

	// Count uuid-bearing events: should be less than total from all 3 sessions
	// (because A and B's common prefix is deduped)
	uuidCount := 0
	for _, ev := range events {
		if _, has := ev["uuid"]; has {
			uuidCount++
		}
	}

	// A has 7 common + 2 extra = 9 uuid events
	// B has 7 common + 2 extra = 9 uuid events
	// After A+B merge: 7 + 2 + 2 = 11 uuid events
	// C has 5 uuid events (init + 2 turns)
	// A+B result and C have no common prefix → concat → 11 + 5 = 16
	if uuidCount != 16 {
		t.Errorf("uuid-bearing event count = %d, want 16", uuidCount)
	}
}

func TestDiff2Identical(t *testing.T) {
	idA := uuid.New().String()
	idB := uuid.New().String()
	evA, evB := testutil.BuildForkedSessions(idA, idB, "A", "B", 3, 0, 0, "2026-03-20T10:00:00Z")

	r := Diff2(evA, evB)
	if r.Relationship != "identical" {
		t.Errorf("relationship = %q, want %q", r.Relationship, "identical")
	}
	if r.CommonCount != 7 { // 1 init + 3*2
		t.Errorf("common = %d, want 7", r.CommonCount)
	}
}

func TestDiff2AContainsB(t *testing.T) {
	idA := uuid.New().String()
	idB := uuid.New().String()
	evA, evB := testutil.BuildForkedSessions(idA, idB, "A", "B", 3, 2, 0, "2026-03-20T10:00:00Z")

	r := Diff2(evA, evB)
	if r.Relationship != "a-contains-b" {
		t.Errorf("relationship = %q, want %q", r.Relationship, "a-contains-b")
	}
	if r.OnlyACount != 4 {
		t.Errorf("onlyA = %d, want 4", r.OnlyACount)
	}
}

func TestDiff2BContainsA(t *testing.T) {
	idA := uuid.New().String()
	idB := uuid.New().String()
	evA, evB := testutil.BuildForkedSessions(idA, idB, "A", "B", 3, 0, 2, "2026-03-20T10:00:00Z")

	r := Diff2(evA, evB)
	if r.Relationship != "b-contains-a" {
		t.Errorf("relationship = %q, want %q", r.Relationship, "b-contains-a")
	}
	if r.OnlyBCount != 4 {
		t.Errorf("onlyB = %d, want 4", r.OnlyBCount)
	}
}

func TestDiff2Diverged(t *testing.T) {
	idA := uuid.New().String()
	idB := uuid.New().String()
	evA, evB := testutil.BuildForkedSessions(idA, idB, "A", "B", 3, 2, 1, "2026-03-20T10:00:00Z")

	r := Diff2(evA, evB)
	if r.Relationship != "diverged" {
		t.Errorf("relationship = %q, want %q", r.Relationship, "diverged")
	}
	if r.CommonCount != 7 {
		t.Errorf("common = %d, want 7", r.CommonCount)
	}
	if r.OnlyACount != 4 {
		t.Errorf("onlyA = %d, want 4", r.OnlyACount)
	}
	if r.OnlyBCount != 2 {
		t.Errorf("onlyB = %d, want 2", r.OnlyBCount)
	}
}

func TestDiff2Unrelated(t *testing.T) {
	idA := uuid.New().String()
	idB := uuid.New().String()
	evA := testutil.BuildSimpleSession(idA, "A", "main", 2, "2026-03-20T10:00:00Z")
	evB := testutil.BuildSimpleSession(idB, "B", "main", 3, "2026-03-20T11:00:00Z")

	r := Diff2(evA, evB)
	if r.Relationship != "unrelated" {
		t.Errorf("relationship = %q, want %q", r.Relationship, "unrelated")
	}
	if r.CommonCount != 0 {
		t.Errorf("common = %d, want 0", r.CommonCount)
	}
}

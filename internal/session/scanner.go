package session

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	cacheVersion  = 1
	headLines     = 50
	tailLines     = 20
	ObserverToken = "claude-mem-observer"
)

// ScanOptions controls which sessions to include.
type ScanOptions struct {
	IncludeObservers bool
	ProjectFilter    string
	BranchFilter     string
	Since            time.Duration
	MinMessages      int
	Stale            bool // <3 msgs AND older than 14d
}

// Scanner discovers and extracts metadata from Claude Code session JSONL files.
type Scanner struct {
	ClaudeDir string
	cache     *CacheFile
	cachePath string
	cacheMu   sync.RWMutex
}

// NewScanner creates a scanner for the given ~/.claude directory.
func NewScanner(claudeDir string) *Scanner {
	s := &Scanner{
		ClaudeDir: claudeDir,
		cachePath: filepath.Join(claudeDir, "csm-cache.json"),
	}
	s.loadCache()
	return s
}

func (s *Scanner) loadCache() {
	s.cache = &CacheFile{Version: cacheVersion, Entries: make(map[string]*CacheEntry)}
	data, err := os.ReadFile(s.cachePath)
	if err != nil {
		return
	}
	var cf CacheFile
	if err := json.Unmarshal(data, &cf); err != nil || cf.Version != cacheVersion {
		return
	}
	s.cache = &cf
}

func (s *Scanner) saveCache() error {
	data, err := json.Marshal(s.cache)
	if err != nil {
		return err
	}
	return os.WriteFile(s.cachePath, data, 0644)
}

// Scan finds all session JSONL files and extracts metadata, using cache where possible.
func (s *Scanner) Scan(opts ScanOptions) ([]SessionMeta, error) {
	projectsDir := filepath.Join(s.ClaudeDir, "projects")
	projEntries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, err
	}

	activeIDs, _ := ActiveSessionIDs(s.ClaudeDir)

	var mu sync.Mutex
	var wg sync.WaitGroup
	var results []SessionMeta
	sem := make(chan struct{}, 16) // limit concurrency

	for _, pe := range projEntries {
		if !pe.IsDir() {
			continue
		}
		projName := pe.Name()

		if !opts.IncludeObservers && strings.Contains(projName, ObserverToken) {
			continue
		}
		if opts.ProjectFilter != "" && !strings.Contains(decodeProjectPath(projName), opts.ProjectFilter) {
			continue
		}

		projPath := filepath.Join(projectsDir, projName)
		jsonlFiles, err := filepath.Glob(filepath.Join(projPath, "*.jsonl"))
		if err != nil {
			continue
		}

		for _, fp := range jsonlFiles {
			wg.Add(1)
			sem <- struct{}{}
			go func(filePath, proj string) {
				defer wg.Done()
				defer func() { <-sem }()

				meta, err := s.extractMeta(filePath, proj, activeIDs)
				if err != nil {
					return
				}

				if opts.BranchFilter != "" && !strings.Contains(strings.ToLower(meta.Branch), strings.ToLower(opts.BranchFilter)) {
					return
				}
				if opts.Since > 0 && time.Since(meta.Modified) > opts.Since {
					return
				}
				if opts.MinMessages > 0 && meta.Messages < opts.MinMessages {
					return
				}
				if opts.Stale && !(meta.Messages < 3 && time.Since(meta.Modified) > 14*24*time.Hour) {
					return
				}

				mu.Lock()
				results = append(results, *meta)
				mu.Unlock()
			}(fp, projName)
		}
	}

	wg.Wait()
	_ = s.saveCache()

	sort.Slice(results, func(i, j int) bool {
		return results[i].Modified.After(results[j].Modified)
	})
	return results, nil
}

func (s *Scanner) extractMeta(filePath, projName string, activeIDs map[string]bool) (*SessionMeta, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	mtime := info.ModTime().UnixMilli()

	// Check cache
	s.cacheMu.RLock()
	ce, ok := s.cache.Entries[filePath]
	s.cacheMu.RUnlock()
	if ok && ce.Mtime == mtime {
		meta := ce.Meta
		meta.IsActive = activeIDs[meta.ID]
		return &meta, nil
	}

	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	meta := &SessionMeta{
		FilePath: filePath,
		FileSize: info.Size(),
		Project:  decodeProjectPath(projName),
	}

	// Read head lines for metadata
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	lineNum := 0
	var firstTimestamp, lastTimestamp string
	userMsgCount := 0

	for scanner.Scan() && lineNum < headLines {
		line := scanner.Bytes()
		lineNum++
		s.processLine(line, meta, &firstTimestamp, &lastTimestamp, &userMsgCount)
	}

	// Scan rest for user message count, title renames, and last timestamp
	for scanner.Scan() {
		line := scanner.Bytes()
		lineNum++
		if containsBytes(line, `"type":"user"`) {
			// Skip tool_result-only user events (not real human prompts)
			if !containsBytes(line, `"tool_result"`) {
				userMsgCount++
			}
			if ts := extractTimestamp(line); ts != "" {
				lastTimestamp = ts
			}
		} else if containsBytes(line, `"custom-title"`) {
			// Pick up renames — last custom-title wins
			if title := extractCustomTitle(line); title != "" {
				meta.Title = title
			}
		} else if containsBytes(line, `"timestamp"`) {
			if ts := extractTimestamp(line); ts != "" {
				lastTimestamp = ts
			}
		}
	}

	meta.Messages = userMsgCount

	if firstTimestamp != "" {
		if t, err := time.Parse(time.RFC3339Nano, firstTimestamp); err == nil {
			meta.Created = t
		}
	}
	if lastTimestamp != "" {
		if t, err := time.Parse(time.RFC3339Nano, lastTimestamp); err == nil {
			meta.Modified = t
		}
	}
	if meta.Created.IsZero() {
		meta.Created = info.ModTime()
	}
	if meta.Modified.IsZero() {
		meta.Modified = info.ModTime()
	}

	if meta.ID == "" {
		// Derive from filename
		base := filepath.Base(filePath)
		meta.ID = strings.TrimSuffix(base, ".jsonl")
	}
	if len(meta.ID) >= 8 {
		meta.ShortID = meta.ID[:8]
	} else {
		meta.ShortID = meta.ID
	}

	meta.IsActive = activeIDs[meta.ID]

	// Update cache
	s.cacheMu.Lock()
	s.cache.Entries[filePath] = &CacheEntry{
		Mtime: mtime,
		Meta:  *meta,
	}
	s.cacheMu.Unlock()

	return meta, nil
}

func (s *Scanner) processLine(line []byte, meta *SessionMeta, firstTs, lastTs *string, userCount *int) {
	var ev Event
	if err := json.Unmarshal(line, &ev); err != nil {
		return
	}

	if ev.SessionID != "" && meta.ID == "" {
		meta.ID = ev.SessionID
	}
	if ev.GitBranch != "" && meta.Branch == "" {
		meta.Branch = ev.GitBranch
	}
	if ev.Slug != "" && meta.Slug == "" {
		meta.Slug = ev.Slug
	}
	if ev.Timestamp != "" {
		if *firstTs == "" {
			*firstTs = ev.Timestamp
		}
		*lastTs = ev.Timestamp
	}

	switch ev.Type {
	case "custom-title":
		if ev.CustomTitle != "" {
			meta.Title = ev.CustomTitle
		}
	case "user":
		if !isToolResult(ev.Message) {
			text := extractUserText(ev.Message)
			if text != "" && !isSystemInjected(text) {
				*userCount++
				if meta.Title == "" {
					meta.Title = text
				}
			}
		}
	}
}

// extractUserText extracts display text from a user message.
func extractUserText(msg any) string {
	if msg == nil {
		return ""
	}

	switch m := msg.(type) {
	case map[string]any:
		content, ok := m["content"]
		if !ok {
			return ""
		}
		switch c := content.(type) {
		case string:
			return truncate(c, 120)
		case []any:
			return extractTextFromBlocks(c)
		}
	case string:
		return truncate(m, 120)
	}
	return ""
}

func extractTextFromBlocks(blocks []any) string {
	for _, b := range blocks {
		bm, ok := b.(map[string]any)
		if !ok {
			continue
		}
		if bm["type"] == "text" {
			if text, ok := bm["text"].(string); ok {
				return truncate(text, 120)
			}
		}
	}
	return ""
}

func extractCustomTitle(line []byte) string {
	idx := findBytes(line, `"customTitle":"`)
	if idx < 0 {
		return ""
	}
	start := idx + len(`"customTitle":"`)
	end := start
	for end < len(line) && line[end] != '"' {
		end++
	}
	if end > start && end < len(line) {
		return string(line[start:end])
	}
	return ""
}

func extractTimestamp(line []byte) string {
	// Fast string search for "timestamp":"..."
	idx := findBytes(line, `"timestamp":"`)
	if idx < 0 {
		return ""
	}
	start := idx + len(`"timestamp":"`)
	end := start
	for end < len(line) && line[end] != '"' {
		end++
	}
	if end > start && end < len(line) {
		return string(line[start:end])
	}
	return ""
}

func containsBytes(haystack []byte, needle string) bool {
	return findBytes(haystack, needle) >= 0
}

func findBytes(haystack []byte, needle string) int {
	return bytes.Index(haystack, []byte(needle))
}

// DecodeProjectPath converts encoded dir name to filesystem path.
func DecodeProjectPath(encoded string) string {
	return decodeProjectPath(encoded)
}

func decodeProjectPath(encoded string) string {
	// Encoded format: -home-ahmed-code-cercli -> /home/ahmed/code/cercli
	if encoded == "" {
		return ""
	}
	parts := strings.Split(encoded, "-")
	// Skip empty first element from leading dash
	if len(parts) > 0 && parts[0] == "" {
		parts = parts[1:]
	}
	return "/" + strings.Join(parts, "/")
}

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

// ReadFullSession reads and parses all events from a session JSONL file.
func ReadFullSession(filePath string) ([]Event, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []Event
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 2*1024*1024), 2*1024*1024)
	for scanner.Scan() {
		var ev Event
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			continue
		}
		events = append(events, ev)
	}
	return events, scanner.Err()
}

// ReadRawEvents reads all JSONL lines as raw maps, preserving every field.
func ReadRawEvents(filePath string) ([]map[string]any, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []map[string]any
	dec := json.NewDecoder(f)
	for dec.More() {
		var ev map[string]any
		if err := dec.Decode(&ev); err != nil {
			continue
		}
		events = append(events, ev)
	}
	return events, nil
}

// ReadUserPrompts extracts user text from a session file, up to maxPrompts.
func ReadUserPrompts(filePath string, maxPrompts int) ([]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var prompts []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 2*1024*1024), 2*1024*1024)

	for scanner.Scan() {
		if len(prompts) >= maxPrompts {
			break
		}
		line := scanner.Bytes()
		if !containsBytes(line, `"type":"user"`) {
			continue
		}
		var ev Event
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		if ev.Type != "user" {
			continue
		}
		if isToolResult(ev.Message) {
			continue
		}
		text := extractUserText(ev.Message)
		if text != "" && !isSystemInjected(text) {
			prompts = append(prompts, text)
		}
	}
	return prompts, scanner.Err()
}

// ReadAssistantTexts extracts assistant text responses (not tool calls) from a session.
func ReadAssistantTexts(filePath string, maxTexts int) ([]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var texts []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 2*1024*1024), 2*1024*1024)

	for scanner.Scan() {
		if len(texts) >= maxTexts {
			break
		}
		line := scanner.Bytes()
		if !containsBytes(line, `"type":"assistant"`) {
			continue
		}

		var raw map[string]any
		if err := json.Unmarshal(line, &raw); err != nil {
			continue
		}
		if raw["type"] != "assistant" {
			continue
		}

		msg, ok := raw["message"]
		if !ok {
			continue
		}
		text := extractAssistantText(msg)
		if text != "" {
			texts = append(texts, text)
		}
	}
	return texts, scanner.Err()
}

func extractAssistantText(msg any) string {
	m, ok := msg.(map[string]any)
	if !ok {
		return ""
	}
	content, ok := m["content"]
	if !ok {
		return ""
	}
	blocks, ok := content.([]any)
	if !ok {
		return ""
	}
	for _, b := range blocks {
		bm, ok := b.(map[string]any)
		if !ok {
			continue
		}
		if bm["type"] == "text" {
			if text, ok := bm["text"].(string); ok {
				return truncate(text, 200)
			}
		}
	}
	return ""
}

func isToolResult(msg any) bool {
	m, ok := msg.(map[string]any)
	if !ok {
		return false
	}
	content, ok := m["content"]
	if !ok {
		return false
	}
	blocks, ok := content.([]any)
	if !ok {
		return false
	}
	for _, b := range blocks {
		bm, ok := b.(map[string]any)
		if !ok {
			continue
		}
		if bm["type"] == "tool_result" {
			return true
		}
	}
	return false
}

// isSystemInjected returns true for messages that aren't real human prompts.
func isSystemInjected(text string) bool {
	prefixes := []string{
		"<local-command",
		"<command-name>",
		"<command-message>",
		"<command-args>",
		"<task-notification>",
		"<system-reminder>",
		"[Request interrupted",
		"This session is being continued from a previous conversation",
		"Base directory for this skill:",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(text, p) {
			return true
		}
	}
	return false
}

// FindSession finds a session by ID prefix or title substring.
// Exact ID prefix match takes priority. Falls back to title search.
func (s *Scanner) FindSessionByPrefix(query string) (*SessionMeta, error) {
	sessions, err := s.Scan(ScanOptions{IncludeObservers: true})
	if err != nil {
		return nil, err
	}

	q := strings.ToLower(query)

	// First: exact ID prefix match
	var idMatches []SessionMeta
	for _, m := range sessions {
		if strings.HasPrefix(strings.ToLower(m.ID), q) {
			idMatches = append(idMatches, m)
		}
	}
	if len(idMatches) == 1 {
		return &idMatches[0], nil
	}
	if len(idMatches) > 1 {
		return nil, &AmbiguousMatchError{Query: query, Matches: idMatches}
	}

	// Second: title substring match
	var titleMatches []SessionMeta
	for _, m := range sessions {
		if strings.Contains(strings.ToLower(m.Title), q) {
			titleMatches = append(titleMatches, m)
		}
	}
	if len(titleMatches) == 1 {
		return &titleMatches[0], nil
	}
	if len(titleMatches) > 1 {
		return nil, &AmbiguousMatchError{Query: query, Matches: titleMatches}
	}

	return nil, nil
}

// AmbiguousMatchError is returned when a query matches multiple sessions.
type AmbiguousMatchError struct {
	Query   string
	Matches []SessionMeta
}

func (e *AmbiguousMatchError) Error() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "ambiguous match for '%s' (%d results):\n", e.Query, len(e.Matches))
	limit := len(e.Matches)
	if limit > 10 {
		limit = 10
	}
	for _, m := range e.Matches[:limit] {
		title := m.Title
		if len(title) > 50 {
			title = title[:47] + "..."
		}
		fmt.Fprintf(&sb, "  %s  %s\n", m.ShortID, title)
	}
	if len(e.Matches) > 10 {
		fmt.Fprintf(&sb, "  ... and %d more\n", len(e.Matches)-10)
	}
	return sb.String()
}

// ScanDir returns the path to the projects directory.
func (s *Scanner) ScanDir() string {
	return filepath.Join(s.ClaudeDir, "projects")
}

// ReadSessionEvents reads events for show/merge, returning only head and tail.
func ReadSessionEvents(filePath string, head, tail int) (headEvents, tailEvents []Event, totalLines int, err error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, nil, 0, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 2*1024*1024), 2*1024*1024)

	var allLines [][]byte
	for sc.Scan() {
		line := make([]byte, len(sc.Bytes()))
		copy(line, sc.Bytes())
		allLines = append(allLines, line)
	}
	if err := sc.Err(); err != nil && err != io.EOF {
		return nil, nil, 0, err
	}

	totalLines = len(allLines)

	headEnd := head
	if headEnd > totalLines {
		headEnd = totalLines
	}
	for _, l := range allLines[:headEnd] {
		var ev Event
		if json.Unmarshal(l, &ev) == nil {
			headEvents = append(headEvents, ev)
		}
	}

	if totalLines > head {
		tailStart := totalLines - tail
		if tailStart < head {
			tailStart = head
		}
		for _, l := range allLines[tailStart:] {
			var ev Event
			if json.Unmarshal(l, &ev) == nil {
				tailEvents = append(tailEvents, ev)
			}
		}
	}

	return headEvents, tailEvents, totalLines, nil
}

// ReadFilesModified extracts file modification info from file-history-snapshot events.
func ReadFilesModified(filePath string) ([]FileModification, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fileMap := make(map[string]*FileModification)
	dec := json.NewDecoder(f)
	for dec.More() {
		var ev map[string]any
		if dec.Decode(&ev) != nil {
			continue
		}
		if ev["type"] != "file-history-snapshot" {
			continue
		}
		snap, ok := ev["snapshot"].(map[string]any)
		if !ok {
			continue
		}
		backups, ok := snap["trackedFileBackups"].(map[string]any)
		if !ok {
			continue
		}
		for path, info := range backups {
			entry, ok := info.(map[string]any)
			if !ok {
				continue
			}
			ver := 0
			if v, ok := entry["version"].(float64); ok {
				ver = int(v)
			}
			var bt time.Time
			if ts, ok := entry["backupTime"].(string); ok {
				bt, _ = time.Parse(time.RFC3339Nano, ts)
			}

			existing, found := fileMap[path]
			if !found || ver > existing.Versions {
				fileMap[path] = &FileModification{Path: path, Versions: ver, LastBackup: bt}
			} else if bt.After(existing.LastBackup) {
				existing.LastBackup = bt
			}
		}
	}

	result := make([]FileModification, 0, len(fileMap))
	for _, fm := range fileMap {
		result = append(result, *fm)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].LastBackup.After(result[j].LastBackup)
	})
	return result, nil
}

// ReadTimeline extracts notable events from a session for timeline display.
func ReadTimeline(filePath string) ([]TimelineEvent, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []TimelineEvent
	dec := json.NewDecoder(f)
	for dec.More() {
		var ev map[string]any
		if dec.Decode(&ev) != nil {
			continue
		}

		typ, _ := ev["type"].(string)
		ts, _ := ev["timestamp"].(string)
		t, _ := time.Parse(time.RFC3339Nano, ts)

		switch typ {
		case "user":
			// Only real user prompts (not tool results, not meta)
			if ev["isMeta"] == true {
				continue
			}
			msg, ok := ev["message"].(map[string]any)
			if !ok {
				continue
			}
			summary := extractUserText(msg)
			if summary == "" {
				continue
			}
			if isSystemInjected(summary) {
				continue
			}
			if len(summary) > 80 {
				summary = summary[:77] + "..."
			}
			events = append(events, TimelineEvent{Time: t, Type: "user", Summary: summary})

		case "assistant":
			msg, ok := ev["message"].(map[string]any)
			if !ok {
				continue
			}
			// Only show end-of-turn assistant messages, not mid-turn tool_use chunks
			sr, _ := msg["stop_reason"].(string)
			if sr != "end_turn" {
				continue
			}
			tokensOut := 0
			if usage, ok := msg["usage"].(map[string]any); ok {
				if ot, ok := usage["output_tokens"].(float64); ok {
					tokensOut = int(ot)
				}
			}
			events = append(events, TimelineEvent{Time: t, Type: "assistant", TokensOut: tokensOut})

		case "system":
			sub, _ := ev["subtype"].(string)
			switch sub {
			case "turn_duration":
				dur, _ := ev["durationMs"].(float64)
				events = append(events, TimelineEvent{Time: t, Type: "turn-duration", DurationMs: int64(dur)})
			case "compact_boundary":
				trigger := ""
				preTokens := 0
				if meta, ok := ev["compactMetadata"].(map[string]any); ok {
					trigger, _ = meta["trigger"].(string)
					if pt, ok := meta["preTokens"].(float64); ok {
						preTokens = int(pt)
					}
				}
				events = append(events, TimelineEvent{Time: t, Type: "compact", Trigger: trigger, PreTokens: preTokens})
			}

		case "queue-operation":
			content, _ := ev["content"].(string)
			if len(content) > 60 {
				content = content[:57] + "..."
			}
			events = append(events, TimelineEvent{Time: t, Type: "queue", Summary: content})
		}
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Time.Before(events[j].Time)
	})
	return events, nil
}

// SearchSession searches a session JSONL for a query string.
func SearchSession(filePath, query string, deep bool) ([]SearchHit, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	lowerQuery := strings.ToLower(query)
	var hits []SearchHit
	lastPromptFound := false

	dec := json.NewDecoder(f)
	for dec.More() {
		var ev map[string]any
		if dec.Decode(&ev) != nil {
			continue
		}

		typ, _ := ev["type"].(string)

		if typ == "last-prompt" && !lastPromptFound {
			lp, _ := ev["lastPrompt"].(string)
			if lp != "" && strings.Contains(strings.ToLower(lp), lowerQuery) {
				ctx := lp
				if len(ctx) > 120 {
					ctx = ctx[:117] + "..."
				}
				hits = append(hits, SearchHit{Context: ctx, Type: "last-prompt"})
				lastPromptFound = true
			}
		}

		if deep && typ == "user" {
			if ev["isMeta"] == true {
				continue
			}
			msg, ok := ev["message"].(map[string]any)
			if !ok {
				continue
			}
			content, ok := msg["content"].(string)
			if !ok {
				continue
			}
			if strings.Contains(strings.ToLower(content), lowerQuery) {
				ctx := content
				if len(ctx) > 120 {
					ctx = ctx[:117] + "..."
				}
				hits = append(hits, SearchHit{Context: ctx, Type: "user-prompt"})
			}
		}
	}
	return hits, nil
}

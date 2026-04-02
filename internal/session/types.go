package session

import "time"

// SessionMeta holds extracted metadata about a Claude Code session.
type SessionMeta struct {
	ID       string    `json:"id"`
	ShortID  string    `json:"short_id"`
	Title    string    `json:"title"`
	Project  string    `json:"project"`
	Branch   string    `json:"branch"`
	Messages int       `json:"messages"`
	Created  time.Time `json:"created"`
	Modified time.Time `json:"modified"`
	FileSize int64     `json:"file_size"`
	FilePath string    `json:"file_path"`
	IsActive bool      `json:"is_active"`
	Slug     string    `json:"slug,omitempty"`
}

// ContentBlock represents a message content block (text, tool_use, tool_result, thinking).
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// MessageWrapper handles both old (dict with role+content string) and new (dict with role+content list) formats.
type MessageWrapper struct {
	Role    string `json:"role,omitempty"`
	Content any    `json:"content,omitempty"` // string or []ContentBlock
}

// Event represents a single JSONL line in a session file.
type Event struct {
	Type        string `json:"type"`
	UUID        string `json:"uuid,omitempty"`
	ParentUUID  string `json:"parentUuid,omitempty"`
	SessionID   string `json:"sessionId,omitempty"`
	Timestamp   string `json:"timestamp,omitempty"`
	GitBranch   string `json:"gitBranch,omitempty"`
	IsSidechain bool   `json:"isSidechain,omitempty"`

	// custom-title event
	CustomTitle string `json:"customTitle,omitempty"`
	// agent-name event
	AgentName string `json:"agentName,omitempty"`

	// user/assistant events — message can be dict or raw
	Message any `json:"message,omitempty"`

	// session slug (human-readable name)
	Slug string `json:"slug,omitempty"`

	// system events
	Subtype string `json:"subtype,omitempty"`
	Content any    `json:"content,omitempty"`
}

// IndexFile is the sessions-index.json format.
type IndexFile struct {
	Version      int          `json:"version"`
	Entries      []IndexEntry `json:"entries"`
	OriginalPath string       `json:"originalPath"`
}

// IndexEntry is a single session entry in sessions-index.json.
type IndexEntry struct {
	SessionID   string `json:"sessionId"`
	FullPath    string `json:"fullPath"`
	FileMtime   int64  `json:"fileMtime"`
	FirstPrompt string `json:"firstPrompt"`
	Summary     string `json:"summary"`
	MsgCount    int    `json:"messageCount"`
	Created     string `json:"created"`
	Modified    string `json:"modified"`
	GitBranch   string `json:"gitBranch"`
	ProjectPath string `json:"projectPath"`
	IsSidechain bool   `json:"isSidechain"`
}

// Task represents a Claude Code task from ~/.claude/tasks/<session-id>/.
type Task struct {
	ID          string   `json:"id"`
	Subject     string   `json:"subject"`
	Description string   `json:"description"`
	ActiveForm  string   `json:"activeForm"`
	Status      string   `json:"status"` // "pending", "in_progress", "completed"
	Blocks      []string `json:"blocks"`
	BlockedBy   []string `json:"blockedBy"`
}

// FileModification represents a file touched during a session.
type FileModification struct {
	Path       string    `json:"path"`
	Versions   int       `json:"versions"`
	LastBackup time.Time `json:"last_backup"`
}

// TimelineEvent represents a notable event in a session's timeline.
type TimelineEvent struct {
	Time       time.Time `json:"time"`
	Type       string    `json:"type"` // "user", "assistant", "compact", "queue", "turn-duration"
	Summary    string    `json:"summary"`
	DurationMs int64     `json:"duration_ms,omitempty"`
	TokensOut  int       `json:"tokens_out,omitempty"`
	PreTokens  int       `json:"pre_tokens,omitempty"`
	Trigger    string    `json:"trigger,omitempty"`
}

// SearchHit represents a search match within a session.
type SearchHit struct {
	Context string `json:"context"` // matched line, truncated
	Type    string `json:"type"`    // "title", "last-prompt", "user-prompt"
}

// CacheFile holds all cached session metadata.
type CacheFile struct {
	Version int                    `json:"version"`
	Entries map[string]*CacheEntry `json:"entries"` // key: filepath
}

// CacheEntry is a cached metadata entry keyed by filepath+mtime.
type CacheEntry struct {
	Mtime int64       `json:"mtime"`
	Meta  SessionMeta `json:"meta"`
}

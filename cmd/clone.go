package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ash0x0/csm/internal/session"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var cloneCmd = &cobra.Command{
	Use:   "clone <session-id>",
	Short: "Clone a session into a new session in the same project",
	Long:  "Creates a copy of an existing session with a new session ID. The clone is placed in the same project directory.",
	Args:  cobra.ExactArgs(1),
	RunE:  runClone,
}

var cloneTitle string

func init() {
	cloneCmd.Flags().StringVarP(&cloneTitle, "title", "t", "", "title for cloned session (default: 'Clone of <original>')")
	rootCmd.AddCommand(cloneCmd)
}

func runClone(cmd *cobra.Command, args []string) error {
	meta, err := findSession(args[0])
	if err != nil {
		return err
	}

	newID, err := cloneSession(meta, cloneTitle)
	if err != nil {
		return err
	}

	fmt.Printf("Cloned %s → %s\n", meta.ShortID, newID[:8])
	fmt.Printf("Resume with: claude --resume %s\n", newID[:8])
	return nil
}

// cloneSession copies a session's events into a new JSONL file with a new session ID.
// Returns the new session ID. Used by both the clone subcommand and the TUI.
func cloneSession(meta *session.SessionMeta, title string) (string, error) {
	if meta.IsActive {
		fmt.Fprintf(os.Stderr, "Note: session %s is currently active\n", meta.ShortID)
	}

	events, skipped, err := session.ReadRawEventsWithStats(meta.FilePath)
	if err != nil {
		return "", fmt.Errorf("reading source session: %w", err)
	}
	if skipped > 0 {
		fmt.Fprintf(os.Stderr, "warning: skipped %d corrupt lines in source\n", skipped)
	}

	newID := uuid.New().String()

	if title == "" {
		src := meta.Title
		if len(src) > 80 {
			src = src[:77] + "..."
		}
		title = "Clone of " + src
	}

	projDir := filepath.Dir(meta.FilePath)
	outputPath := filepath.Join(projDir, newID+".jsonl")
	tmpPath := outputPath + ".tmp"

	f, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("creating output file: %w", err)
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

	if err := enc.Encode(map[string]any{
		"type":        "custom-title",
		"customTitle": title,
		"sessionId":   newID,
	}); err != nil {
		return "", fmt.Errorf("writing custom-title: %w", err)
	}
	if err := enc.Encode(map[string]any{
		"type":      "agent-name",
		"agentName": title,
		"sessionId": newID,
	}); err != nil {
		return "", fmt.Errorf("writing agent-name: %w", err)
	}

	for _, ev := range events {
		typ, _ := ev["type"].(string)
		if typ == "custom-title" || typ == "agent-name" {
			continue
		}
		if _, has := ev["sessionId"]; has {
			ev["sessionId"] = newID
		}
		if err := enc.Encode(ev); err != nil {
			return "", fmt.Errorf("writing event: %w", err)
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

	MarkDirty(projDir)
	return newID, nil
}

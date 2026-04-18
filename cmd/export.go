package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ash0x0/csm/internal/session"
	"github.com/spf13/cobra"
)

var (
	exportOutput      string
	exportFormat      string
	exportNoToolCalls bool
)

var exportCmd = &cobra.Command{
	Use:   "export <session-id>",
	Short: "Export a session as readable text or JSON",
	Args:  cobra.ExactArgs(1),
	RunE:  runExport,
}

func init() {
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "write output to file instead of stdout")
	exportCmd.Flags().StringVar(&exportFormat, "format", "markdown", "output format: markdown or json")
	exportCmd.Flags().BoolVar(&exportNoToolCalls, "no-tool-calls", false, "skip turns that are tool use/result only")
	rootCmd.AddCommand(exportCmd)
}

type exportTurn struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

func runExport(cmd *cobra.Command, args []string) error {
	meta, err := findSession(args[0])
	if err != nil {
		return err
	}

	events, err := session.ReadRawEvents(meta.FilePath)
	if err != nil {
		return fmt.Errorf("reading events: %w", err)
	}

	var turns []exportTurn
	for _, ev := range events {
		evType, _ := ev["type"].(string)
		if evType != "user" && evType != "assistant" {
			continue
		}

		msg, _ := ev["message"].(map[string]any)
		if msg == nil {
			continue
		}

		text := extractExportText(msg)
		if text == "" {
			continue
		}

		if exportNoToolCalls && isToolHeavy(msg) {
			continue
		}

		ts, _ := ev["timestamp"].(string)
		turns = append(turns, exportTurn{
			Role:      evType,
			Content:   text,
			Timestamp: ts,
		})
	}

	out := os.Stdout
	if exportOutput != "" {
		f, err := os.Create(exportOutput)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
		}
		defer f.Close()
		out = f
	}

	switch exportFormat {
	case "json":
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(turns)
	default:
		return writeMarkdown(out, meta, turns)
	}
}

func writeMarkdown(out *os.File, meta *session.SessionMeta, turns []exportTurn) error {
	date := meta.Created.Format("2006-01-02")
	fmt.Fprintf(out, "# Session: %s\n", meta.Title)
	fmt.Fprintf(out, "_%s · %d turns_\n\n---\n\n", date, len(turns))

	for _, t := range turns {
		label := "User"
		if t.Role == "assistant" {
			label = "Claude"
		}
		timeStr := formatExportTime(t.Timestamp)
		if timeStr != "" {
			fmt.Fprintf(out, "**%s** · %s\n\n", label, timeStr)
		} else {
			fmt.Fprintf(out, "**%s**\n\n", label)
		}
		fmt.Fprintf(out, "%s\n\n", t.Content)
	}
	return nil
}

func extractExportText(msg map[string]any) string {
	content, ok := msg["content"]
	if !ok {
		return ""
	}
	switch c := content.(type) {
	case string:
		return strings.TrimSpace(c)
	case []any:
		return extractExportTextFromBlocks(c)
	}
	return ""
}

func extractExportTextFromBlocks(blocks []any) string {
	var parts []string
	for _, b := range blocks {
		bm, ok := b.(map[string]any)
		if !ok {
			continue
		}
		if bm["type"] == "text" {
			if text, ok := bm["text"].(string); ok && strings.TrimSpace(text) != "" {
				parts = append(parts, strings.TrimSpace(text))
			}
		}
	}
	return strings.Join(parts, "\n\n")
}

// isToolHeavy returns true when a message consists entirely of tool use/result blocks
// with no meaningful text content.
func isToolHeavy(msg map[string]any) bool {
	content, ok := msg["content"]
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
		if t, _ := bm["type"].(string); t == "text" {
			if text, _ := bm["text"].(string); strings.TrimSpace(text) != "" {
				return false
			}
		}
	}
	return true
}

func formatExportTime(ts string) string {
	if ts == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ""
	}
	return t.Local().Format("15:04")
}

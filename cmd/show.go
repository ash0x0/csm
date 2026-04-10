package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/ash0x0/csm/internal/format"
	"github.com/ash0x0/csm/internal/session"
	"github.com/spf13/cobra"
)

var (
	showMaxPrompts int
	showFiles      bool
)

var showCmd = &cobra.Command{
	Use:   "show <session-id-prefix>",
	Short: "Show detailed info about a session",
	Args:  cobra.ExactArgs(1),
	RunE:  runShow,
}

func init() {
	showCmd.Flags().IntVar(&showMaxPrompts, "prompts", 20, "max number of prompts to display")
	showCmd.Flags().BoolVar(&showFiles, "files", false, "show files modified during session")
	rootCmd.AddCommand(showCmd)
}

func runShow(cmd *cobra.Command, args []string) error {
	meta, err := findSession(args[0])
	if err != nil {
		return err
	}

	prompts, err := session.ReadUserPrompts(meta.FilePath, showMaxPrompts)
	if err != nil {
		return fmt.Errorf("reading prompts: %w", err)
	}

	format.PrintDetail(meta, prompts)

	// Always show files + tasks summary in detail view (used by TUI preview too)
	files, _ := session.ReadFilesModified(meta.FilePath)
	if len(files) > 0 {
		fmt.Printf("\n── Files Modified (%d) ────────────────────────────\n", len(files))
		limit := len(files)
		if !showFiles && limit > 10 {
			limit = 10
		}
		for _, f := range files[:limit] {
			path := f.Path
			if meta.Project != "" {
				path = strings.TrimPrefix(path, meta.Project+"/")
			}
			if len(path) > 60 {
				path = "..." + path[len(path)-57:]
			}
			fmt.Printf("  v%-3d %-60s  %s\n", f.Versions, path, formatModTime(f.LastBackup))
		}
		if !showFiles && len(files) > 10 {
			fmt.Printf("  ... and %d more (use --files to show all)\n", len(files)-10)
		}
	}

	tasks, _ := session.ReadTasks(claudeDir, meta.ID)
	if len(tasks) > 0 {
		pending, inProg, done := 0, 0, 0
		for _, t := range tasks {
			switch t.Status {
			case "completed":
				done++
			case "in_progress":
				inProg++
			default:
				pending++
			}
		}
		fmt.Printf("\n── Tasks (%d) ─────────────────────────────────────\n", len(tasks))
		parts := []string{}
		if done > 0 {
			parts = append(parts, fmt.Sprintf("%d done", done))
		}
		if inProg > 0 {
			parts = append(parts, fmt.Sprintf("%d in progress", inProg))
		}
		if pending > 0 {
			parts = append(parts, fmt.Sprintf("%d pending", pending))
		}
		fmt.Printf("  %s\n", strings.Join(parts, ", "))
	}

	return nil
}

func formatModTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return t.Format("15:04")
	}
	if t.Year() == now.Year() {
		return t.Format("Jan 02")
	}
	return t.Format("2006-01-02")
}

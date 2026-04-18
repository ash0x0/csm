package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ash0x0/csm/internal/merge"
	"github.com/ash0x0/csm/internal/session"
	"github.com/spf13/cobra"
)

var mergeCmd = &cobra.Command{
	Use:   "merge [session-id...]",
	Short: "Merge context from multiple sessions into a new one",
	Long:  "With no args, opens an interactive picker to select sessions. With args, merges the specified sessions.",
	RunE:  runMerge,
}

var (
	mergeTitle   string
	mergeProject string
	mergeDryRun  bool
)

func init() {
	mergeCmd.Flags().StringVarP(&mergeTitle, "title", "t", "", "title for merged session")
	mergeCmd.Flags().StringVarP(&mergeProject, "project", "p", "", "project directory for output")
	mergeCmd.Flags().BoolVar(&mergeDryRun, "dry-run", false, "preview merge without writing output file")
	rootCmd.AddCommand(mergeCmd)
}

func runMerge(cmd *cobra.Command, args []string) error {
	var metas []*session.SessionMeta

	if len(args) == 0 {
		// Interactive mode: use fzf for multi-select
		selected, err := interactiveSelect()
		if err != nil {
			return err
		}
		if len(selected) < 2 {
			return fmt.Errorf("select at least 2 sessions to merge (got %d)", len(selected))
		}
		for _, prefix := range selected {
			meta, err := findSession(prefix)
			if err != nil {
				return err
			}
			metas = append(metas, meta)
		}
	} else {
		if len(args) < 2 {
			return fmt.Errorf("need at least 2 session IDs to merge")
		}
		for _, prefix := range args {
			meta, err := findSession(prefix)
			if err != nil {
				return err
			}
			metas = append(metas, meta)
		}
	}

	fmt.Printf("Merging %d sessions:\n", len(metas))
	for i, m := range metas {
		fmt.Printf("  %d. %s (%s, %d msgs)\n", i+1, m.ShortID, m.Title, m.Messages)
	}
	fmt.Println()

	opts := merge.MergeOptions{
		Title:  mergeTitle,
		DryRun: mergeDryRun,
	}

	if mergeProject != "" {
		opts.OutputDir = mergeProject
	}

	newID, report, err := merge.MergeN(metas, opts)
	if err != nil {
		return fmt.Errorf("merge failed: %w", err)
	}

	if mergeDryRun {
		fmt.Printf("Strategy:      %s\n", report.Strategy)
		fmt.Printf("Shared events: %d\n", report.SharedCount)
		fmt.Printf("Session A unique: %d\n", report.BranchAOnly)
		fmt.Printf("Session B unique: %d\n", report.BranchBOnly)
		fmt.Printf("Total output events: %d\n", report.TotalEvents)
		fmt.Println("(dry-run: no file written)")
		return nil
	}

	// Mark the output project dir dirty (defaults to first session's project)
	outputDir := filepath.Dir(metas[0].FilePath)
	if mergeProject != "" {
		outputDir = mergeProject
	}
	MarkDirty(outputDir)

	fmt.Printf("Created merged session: %s\n", newID)
	fmt.Printf("Strategy: %s | events: %d\n", report.Strategy, report.TotalEvents)
	fmt.Printf("Resume with: claude --resume %s\n", newID[:8])
	return nil
}

func interactiveSelect() ([]string, error) {
	// Check fzf is available
	if _, err := exec.LookPath("fzf"); err != nil {
		return nil, fmt.Errorf("fzf is required for interactive mode (install with: nix profile install nixpkgs#fzf)")
	}

	// Get session list
	scanner := session.NewScanner(claudeDir)
	sessions, err := scanner.Scan(session.ScanOptions{})
	if err != nil {
		return nil, err
	}

	if len(sessions) == 0 {
		return nil, fmt.Errorf("no sessions found")
	}

	// Build fzf input lines
	var lines []string
	for _, s := range sessions {
		title := s.Title
		if len(title) > 50 {
			title = title[:47] + "..."
		}
		branch := s.Branch
		if branch == "HEAD" {
			branch = ""
		}
		marker := ""
		if s.IsActive {
			marker = " *"
		}
		line := fmt.Sprintf("%s%s  %-50s  %4d msgs  %-10s  %s  %s",
			s.ShortID, marker, title, s.Messages, fmtDate(s.Modified), s.Project, branch)
		lines = append(lines, line)
	}

	input := strings.Join(lines, "\n")

	self, err := os.Executable()
	if err != nil {
		self = "csm"
	}

	// Run fzf with multi-select
	fzfCmd := exec.Command("fzf",
		"--multi",
		"--header", "Select sessions to merge (TAB to select, ENTER to confirm)",
		"--prompt", "merge> ",
		"--preview", self+" show {1}",
		"--preview-window", "right:50%:wrap",
	)
	fzfCmd.Stdin = strings.NewReader(input)
	fzfCmd.Stderr = os.Stderr

	out, err := fzfCmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 130 {
			return nil, fmt.Errorf("cancelled")
		}
		return nil, fmt.Errorf("fzf: %w", err)
	}

	// Extract session IDs from selected lines
	var ids []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: "shortID  title..." or "shortID *  title..." — field[0] is always the ID
		fields := strings.Fields(line)
		if len(fields) >= 1 {
			ids = append(ids, fields[0])
		}
	}

	return ids, nil
}

func fmtDate(t time.Time) string {
	if time.Since(t) < 24*time.Hour {
		return t.Format("15:04")
	}
	if time.Since(t) < 365*24*time.Hour {
		return t.Format("Jan 02")
	}
	return t.Format("2006-01-02")
}

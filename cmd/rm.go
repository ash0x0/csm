package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ash0x0/csm/internal/format"
	"github.com/ash0x0/csm/internal/session"
	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:   "rm [session-id-prefix...]",
	Short: "Delete sessions and associated artifacts",
	RunE:  runRm,
}

var (
	rmOlderThan string
	rmStale     bool
	rmProject   string
	rmDryRun    bool
	rmForce     bool
	rmOrphaned  bool
)

func init() {
	rmCmd.Flags().StringVar(&rmOlderThan, "older-than", "", "delete sessions older than duration (e.g. 30d)")
	rmCmd.Flags().BoolVar(&rmStale, "stale", false, "delete stale sessions (<3 msgs AND older than 14d)")
	rmCmd.Flags().StringVarP(&rmProject, "project", "p", "", "scope to project")
	rmCmd.Flags().BoolVarP(&rmDryRun, "dry-run", "n", false, "show what would be deleted")
	rmCmd.Flags().BoolVarP(&rmForce, "force", "f", false, "skip confirmation")
	rmCmd.Flags().BoolVar(&rmOrphaned, "orphaned", false, "clean orphaned artifacts (session-env, tasks) with no session")
	rootCmd.AddCommand(rmCmd)
}

func runRm(cmd *cobra.Command, args []string) error {
	if rmOrphaned {
		return cleanOrphaned()
	}

	scanner := session.NewScanner(claudeDir)

	// If specific IDs given, delete those
	if len(args) > 0 {
		for _, prefix := range args {
			meta, err := scanner.FindSessionByPrefix(prefix)
			if err != nil {
				return err
			}
			if meta == nil {
				fmt.Fprintf(os.Stderr, "No session found matching '%s'\n", prefix)
				continue
			}
			if meta.IsActive {
				fmt.Fprintf(os.Stderr, "Session %s is active — skipping\n", meta.ShortID)
				continue
			}
			if !rmForce && !rmDryRun {
				fmt.Printf("Delete %s (%s, %s)? [y/N] ", meta.ShortID, meta.Title, format.FormatSize(meta.FileSize))
				if !confirm() {
					continue
				}
			}
			if rmDryRun {
				fmt.Printf("Would delete: %s (%s)\n", meta.ShortID, meta.Title)
				continue
			}
			deleted, err := session.DeleteSession(claudeDir, meta)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error deleting %s: %v\n", meta.ShortID, err)
				continue
			}
			MarkDirty(filepath.Dir(meta.FilePath))
			fmt.Printf("Deleted %s: %s\n", meta.ShortID, strings.Join(deleted, ", "))
		}
		return nil
	}

	// Batch mode with filters
	isBatch := rmOlderThan != "" || rmStale
	if !isBatch {
		return fmt.Errorf("specify session IDs or use --older-than/--stale filters")
	}

	var since time.Duration
	if rmOlderThan != "" {
		d, err := parseDuration(rmOlderThan)
		if err != nil {
			return fmt.Errorf("invalid --older-than: %w", err)
		}
		since = d
	}

	// For batch deletion, scan with inverted filters to find old/stale sessions
	allSessions, err := scanner.Scan(session.ScanOptions{
		ProjectFilter: rmProject,
	})
	if err != nil {
		return err
	}

	var candidates []session.SessionMeta
	for _, s := range allSessions {
		if rmOlderThan != "" && time.Since(s.Modified) < since {
			continue
		}
		if rmStale && !(s.Messages < 3 && time.Since(s.Modified) > 14*24*time.Hour) {
			continue
		}
		candidates = append(candidates, s)
	}

	if len(candidates) == 0 {
		fmt.Println("No sessions matched the filter criteria.")
		return nil
	}

	format.PrintDeletePreview(candidates)

	if rmDryRun {
		fmt.Println("\n(dry run — no files deleted)")
		return nil
	}

	if !rmForce {
		fmt.Print("\nProceed with deletion? [y/N] ")
		if !confirm() {
			fmt.Println("Aborted.")
			return nil
		}
	}

	deleted := 0
	skipped := 0
	for _, s := range candidates {
		if s.IsActive {
			skipped++
			continue
		}
		if _, err := session.DeleteSession(claudeDir, &s); err != nil {
			fmt.Fprintf(os.Stderr, "Error deleting %s: %v\n", s.ShortID, err)
			continue
		}
		MarkDirty(filepath.Dir(s.FilePath))
		deleted++
	}
	fmt.Printf("Deleted %d sessions.\n", deleted)
	if skipped > 0 {
		fmt.Printf("Skipped %d active sessions.\n", skipped)
	}
	return nil
}

func cleanOrphaned() error {
	orphans, err := session.CleanOrphanedArtifacts(claudeDir, rmDryRun)
	if err != nil {
		return err
	}
	if len(orphans) == 0 {
		fmt.Println("No orphaned artifacts found.")
		return nil
	}
	for _, o := range orphans {
		if rmDryRun {
			fmt.Printf("Would delete: %s\n", o)
		} else {
			fmt.Printf("Deleted: %s\n", o)
		}
	}
	if rmDryRun {
		fmt.Printf("\n%d orphaned artifacts (dry run)\n", len(orphans))
	} else {
		fmt.Printf("\n%d orphaned artifacts cleaned.\n", len(orphans))
	}
	return nil
}

func confirm() bool {
	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(strings.ToLower(text)) == "y"
}

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ash0x0/csm/internal/session"
	"github.com/spf13/cobra"
)

var claudeDir string

// dirtyProjects tracks project directories modified during this run.
// Commands that create, delete, move, or merge sessions add
// their project dirs here. On exit, these dirs get reindexed so that
// Claude Code's /resume picker stays in sync.
var dirtyProjects = make(map[string]bool)

// MarkDirty records a project directory as needing reindex on exit.
func MarkDirty(projDir string) {
	dirtyProjects[projDir] = true
}

var rootCmd = &cobra.Command{
	Use:   "csm",
	Short: "Claude Session Manager — fast listing, cleanup, and merging for Claude Code sessions",
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if len(dirtyProjects) == 0 {
			return
		}
		for dir := range dirtyProjects {
			count, err := session.RebuildIndex(dir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "reindex %s: %v\n", dir, err)
				continue
			}
			fmt.Fprintf(os.Stderr, "Reindexed %s (%d sessions)\n", filepath.Base(dir), count)
		}
	},
}

func clearCache() {
	if err := os.Remove(filepath.Join(claudeDir, "csm-cache.json")); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "warning: failed to clear cache: %v\n", err)
	}
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	home, _ := os.UserHomeDir()
	defaultDir := filepath.Join(home, ".claude")
	rootCmd.PersistentFlags().StringVar(&claudeDir, "claude-dir", defaultDir, "path to Claude Code data directory")
}

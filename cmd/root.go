package cmd

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var claudeDir string

var rootCmd = &cobra.Command{
	Use:   "csm",
	Short: "Claude Session Manager — fast listing, cleanup, and merging for Claude Code sessions",
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

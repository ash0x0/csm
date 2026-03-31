package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ash0x0/csm/internal/session"
	"github.com/spf13/cobra"
)

var reindexCmd = &cobra.Command{
	Use:   "reindex",
	Short: "Rebuild sessions-index.json for all projects (fixes /resume)",
	RunE:  runReindex,
}

var reindexProject string

func init() {
	reindexCmd.Flags().StringVarP(&reindexProject, "project", "p", "", "reindex only this project")
	rootCmd.AddCommand(reindexCmd)
}

func runReindex(cmd *cobra.Command, args []string) error {
	projectsDir := filepath.Join(claudeDir, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return err
	}

	total := 0
	projects := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if strings.Contains(e.Name(), "claude-mem-observer") {
			continue
		}
		if reindexProject != "" && !strings.Contains(e.Name(), reindexProject) {
			continue
		}

		projDir := filepath.Join(projectsDir, e.Name())

		// Check if there are any JSONL files
		jsonlFiles, _ := filepath.Glob(filepath.Join(projDir, "*.jsonl"))
		if len(jsonlFiles) == 0 {
			continue
		}

		count, err := session.RebuildIndex(projDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reindexing %s: %v\n", e.Name(), err)
			continue
		}

		fmt.Printf("  %-50s  %3d sessions\n", e.Name(), count)
		total += count
		projects++
	}

	fmt.Printf("\nReindexed %d sessions across %d projects.\n", total, projects)
	return nil
}

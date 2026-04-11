package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ash0x0/csm/internal/session"
	"github.com/spf13/cobra"
)

var tasksJSON bool

var tasksCmd = &cobra.Command{
	Use:   "tasks <session-id-prefix>",
	Short: "Show tasks for a session",
	Args:  cobra.ExactArgs(1),
	RunE:  runTasks,
}

func init() {
	tasksCmd.Flags().BoolVar(&tasksJSON, "json", false, "JSON output")
	rootCmd.AddCommand(tasksCmd)
}

func runTasks(cmd *cobra.Command, args []string) error {
	meta, err := findSession(args[0])
	if err != nil {
		return err
	}

	tasks, err := session.ReadTasks(claudeDir, meta.ID)
	if err != nil {
		return err
	}

	if len(tasks) == 0 {
		fmt.Printf("No tasks found for session %s (%s)\n", meta.ShortID, meta.Title)
		return nil
	}

	if tasksJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(tasks)
	}

	fmt.Printf("Tasks for %s (%s)\n\n", meta.ShortID, meta.Title)
	for _, t := range tasks {
		icon := statusIcon(t.Status)
		fmt.Printf("  %s %2s. %s\n", icon, t.ID, t.Subject)
		if t.Description != "" && t.Description != t.Subject {
			desc := t.Description
			if descRunes := []rune(desc); len(descRunes) > 80 {
				desc = string(descRunes[:77]) + "..."
			}
			fmt.Printf("       %s\n", desc)
		}
	}
	return nil
}

func statusIcon(status string) string {
	switch status {
	case "completed":
		return "[x]"
	case "in_progress":
		return "[~]"
	default:
		return "[ ]"
	}
}

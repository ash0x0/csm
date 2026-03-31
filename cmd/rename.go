package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/ash0x0/csm/internal/session"
	"github.com/spf13/cobra"
)

var renameCmd = &cobra.Command{
	Use:   "rename <session-id> [new-title]",
	Short: "Rename a session",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runRename,
}

func init() {
	rootCmd.AddCommand(renameCmd)
}

func runRename(cmd *cobra.Command, args []string) error {
	meta, err := findSession(args[0])
	if err != nil {
		return err
	}

	var newTitle string
	if len(args) >= 2 {
		newTitle = args[1]
	} else {
		fmt.Printf("Current title: %s\n", meta.Title)
		fmt.Print("New title: ")
		reader := bufio.NewReader(os.Stdin)
		newTitle, _ = reader.ReadString('\n')
		newTitle = strings.TrimSpace(newTitle)
	}

	if newTitle == "" {
		return fmt.Errorf("title cannot be empty")
	}

	if err := session.RenameSession(meta, newTitle); err != nil {
		return fmt.Errorf("rename failed: %w", err)
	}

	// Invalidate cache for this file
	os.Remove(claudeDir + "/csm-cache.json")

	fmt.Printf("Renamed %s → %s\n", meta.ShortID, newTitle)
	return nil
}

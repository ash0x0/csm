package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ash0x0/csm/internal/session"
	"github.com/spf13/cobra"
)

var renameTtyCmd = &cobra.Command{
	Use:    "_rename-tty <session-id>",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE:   runRenameTty,
}

func init() {
	rootCmd.AddCommand(renameTtyCmd)
}

func runRenameTty(cmd *cobra.Command, args []string) error {
	meta, err := findSession(args[0])
	if err != nil {
		return err
	}

	// Read from /dev/tty since fzf's execute connects to the terminal
	tty, err := os.Open("/dev/tty")
	if err != nil {
		return fmt.Errorf("cannot open /dev/tty: %w", err)
	}
	defer tty.Close()

	fmt.Printf("Current: %s\n", meta.Title)
	fmt.Print("New title: ")
	reader := bufio.NewReader(tty)
	newTitle, _ := reader.ReadString('\n')
	newTitle = strings.TrimSpace(newTitle)

	if newTitle == "" {
		fmt.Println("(cancelled)")
		return nil
	}

	if err := session.RenameSession(meta, newTitle); err != nil {
		return fmt.Errorf("rename failed: %w", err)
	}

	os.Remove(claudeDir + "/csm-cache.json")
	MarkDirty(filepath.Dir(meta.FilePath))
	fmt.Printf("Renamed → %s\n", newTitle)
	return nil
}

package cmd

import (
	"fmt"

	"github.com/ash0x0/csm/internal/format"
	"github.com/ash0x0/csm/internal/session"
	"github.com/spf13/cobra"
)

var showMaxPrompts int

var showCmd = &cobra.Command{
	Use:   "show <session-id-prefix>",
	Short: "Show detailed info about a session",
	Args:  cobra.ExactArgs(1),
	RunE:  runShow,
}

func init() {
	showCmd.Flags().IntVar(&showMaxPrompts, "prompts", 20, "max number of prompts to display")
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
	return nil
}

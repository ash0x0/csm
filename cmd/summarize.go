package cmd

import (
	"fmt"
	"time"

	"github.com/ash0x0/csm/internal/summarize"
	"github.com/spf13/cobra"
)

var summarizeCmd = &cobra.Command{
	Use:   "summarize <session-id>",
	Short: "Compress a long session into a lean summary session via Claude",
	Long:  "Reads a session's conversation, generates a summary using the Claude CLI, and writes a new compact session you can resume.",
	Args:  cobra.ExactArgs(1),
	RunE:  runSummarize,
}

var (
	summarizeModel   string
	summarizePrint   bool
	summarizeTimeout time.Duration
)

func init() {
	summarizeCmd.Flags().StringVar(&summarizeModel, "model", "haiku", "Claude model alias to use for summarization")
	summarizeCmd.Flags().BoolVar(&summarizePrint, "print", false, "print summary to stdout without creating a new session")
	summarizeCmd.Flags().DurationVar(&summarizeTimeout, "timeout", 0, "max time to wait for claude (default 5m)")
	rootCmd.AddCommand(summarizeCmd)
}

func runSummarize(cmd *cobra.Command, args []string) error {
	meta, err := findSession(args[0])
	if err != nil {
		return err
	}

	fmt.Printf("Summarizing session: %s (%s, %d msgs)\n", meta.ShortID, meta.Title, meta.Messages)

	opts := summarize.Options{
		Model:   summarizeModel,
		Print:   summarizePrint,
		Timeout: summarizeTimeout,
	}

	newID, err := summarize.Summarize(meta, opts)
	if err != nil {
		return fmt.Errorf("summarize failed: %w", err)
	}

	if summarizePrint {
		return nil
	}

	MarkDirty(claudeDir)
	fmt.Printf("Created summary session: %s\n", newID[:8])
	fmt.Printf("Resume with: claude --resume %s\n", newID[:8])
	return nil
}

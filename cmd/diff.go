package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ash0x0/csm/internal/merge"
	"github.com/ash0x0/csm/internal/session"
	"github.com/spf13/cobra"
)

var diffJSON bool

var diffCmd = &cobra.Command{
	Use:   "diff <session-a> <session-b>",
	Short: "Compare two sessions and report their relationship",
	Args:  cobra.ExactArgs(2),
	RunE:  runDiff,
}

func init() {
	diffCmd.Flags().BoolVar(&diffJSON, "json", false, "JSON output")
	rootCmd.AddCommand(diffCmd)
}

func runDiff(cmd *cobra.Command, args []string) error {
	metaA, err := findSession(args[0])
	if err != nil {
		return err
	}
	metaB, err := findSession(args[1])
	if err != nil {
		return err
	}

	eventsA, err := session.ReadRawEvents(metaA.FilePath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", metaA.ShortID, err)
	}
	eventsB, err := session.ReadRawEvents(metaB.FilePath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", metaB.ShortID, err)
	}

	result := merge.Diff2(eventsA, eventsB)

	if diffJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	switch result.Relationship {
	case "identical":
		fmt.Printf("  Both: %d events\n", result.CommonCount)
		fmt.Printf("  Relationship: identical\n")
	case "a-contains-b":
		fmt.Printf("  %s: %d events (%s)\n", metaA.ShortID, result.CommonCount+result.OnlyACount, metaA.Title)
		fmt.Printf("  %s: %d events (%s)\n", metaB.ShortID, result.CommonCount, metaB.Title)
		fmt.Printf("  Relationship: A contains B (superset)\n")
	case "b-contains-a":
		fmt.Printf("  %s: %d events (%s)\n", metaA.ShortID, result.CommonCount, metaA.Title)
		fmt.Printf("  %s: %d events (%s)\n", metaB.ShortID, result.CommonCount+result.OnlyBCount, metaB.Title)
		fmt.Printf("  Relationship: B contains A (superset)\n")
	case "diverged":
		fmt.Printf("Sessions share %d events, then diverge.\n", result.CommonCount)
		fmt.Printf("  %s: %d shared + %d unique (%s)\n", metaA.ShortID, result.CommonCount, result.OnlyACount, metaA.Title)
		fmt.Printf("  %s: %d shared + %d unique (%s)\n", metaB.ShortID, result.CommonCount, result.OnlyBCount, metaB.Title)
		fmt.Printf("  Relationship: diverged\n")
	case "unrelated":
		fmt.Printf("  %s: %d events (%s)\n", metaA.ShortID, result.CommonCount+result.OnlyACount, metaA.Title)
		fmt.Printf("  %s: %d events (%s)\n", metaB.ShortID, result.CommonCount+result.OnlyBCount, metaB.Title)
		fmt.Printf("  Relationship: unrelated (no shared history)\n")
	}

	return nil
}

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ash0x0/csm/internal/session"
	"github.com/spf13/cobra"
)

var timelineJSON bool

var timelineCmd = &cobra.Command{
	Use:   "timeline <session-id-prefix>",
	Short: "Show chronological timeline of a session",
	Args:  cobra.ExactArgs(1),
	RunE:  runTimeline,
}

func init() {
	timelineCmd.Flags().BoolVar(&timelineJSON, "json", false, "JSON output")
	rootCmd.AddCommand(timelineCmd)
}

func runTimeline(cmd *cobra.Command, args []string) error {
	meta, err := findSession(args[0])
	if err != nil {
		return err
	}

	events, err := session.ReadTimeline(meta.FilePath)
	if err != nil {
		return err
	}

	if len(events) == 0 {
		fmt.Printf("No timeline events for session %s\n", meta.ShortID)
		return nil
	}

	if timelineJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(events)
	}

	fmt.Printf("Timeline for %s (%s)\n\n", meta.ShortID, meta.Title)
	for _, ev := range events {
		ts := ev.Time.Format("15:04:05")
		switch ev.Type {
		case "user":
			fmt.Printf("  %s  > %s\n", ts, ev.Summary)
		case "assistant":
			if ev.TokensOut >= 1000 {
				fmt.Printf("  %s  < assistant (%dk tokens)\n", ts, ev.TokensOut/1000)
			} else if ev.TokensOut > 0 {
				fmt.Printf("  %s  < assistant (%d tokens)\n", ts, ev.TokensOut)
			} else {
				fmt.Printf("  %s  < assistant\n", ts)
			}
		case "turn-duration":
			fmt.Printf("  %s    turn: %s\n", ts, formatDuration(ev.DurationMs))
		case "compact":
			pre := ""
			if ev.PreTokens > 0 {
				pre = fmt.Sprintf(", %dk tokens before", ev.PreTokens/1000)
			}
			fmt.Printf("  %s  ~ compacted (%s%s)\n", ts, ev.Trigger, pre)
		case "queue":
			fmt.Printf("  %s  ! queued: %s\n", ts, ev.Summary)
		}
	}
	return nil
}

func formatDuration(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

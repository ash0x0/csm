package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	activityWeeks int
	activityJSON  bool
)

var activityCmd = &cobra.Command{
	Use:   "activity",
	Short: "Show activity heatmap from Claude Code usage stats",
	Args:  cobra.NoArgs,
	RunE:  runActivity,
}

func init() {
	activityCmd.Flags().IntVar(&activityWeeks, "weeks", 12, "number of weeks to show")
	activityCmd.Flags().BoolVar(&activityJSON, "json", false, "JSON output")
	rootCmd.AddCommand(activityCmd)
}

type statsCache struct {
	TotalSessions  int             `json:"totalSessions"`
	TotalMessages  int             `json:"totalMessages"`
	FirstSession   string          `json:"firstSessionDate"`
	DailyActivity  []dailyActivity `json:"dailyActivity"`
	HourCounts     map[string]int  `json:"hourCounts"`
}

type dailyActivity struct {
	Date         string `json:"date"`
	MessageCount int    `json:"messageCount"`
	SessionCount int    `json:"sessionCount"`
	ToolCalls    int    `json:"toolCallCount"`
}

func runActivity(cmd *cobra.Command, args []string) error {
	statsPath := filepath.Join(claudeDir, "stats-cache.json")
	data, err := os.ReadFile(statsPath)
	if err != nil {
		return fmt.Errorf("no stats-cache.json found (run Claude Code first)")
	}

	var stats statsCache
	if err := json.Unmarshal(data, &stats); err != nil {
		return fmt.Errorf("parsing stats: %w", err)
	}

	if activityJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(stats)
	}

	printHeatmap(stats, activityWeeks)
	fmt.Println()
	printHourDistribution(stats)
	fmt.Println()
	firstDate := stats.FirstSession
	if len(firstDate) > 10 {
		firstDate = firstDate[:10]
	}
	fmt.Printf("Total: %d sessions, %d messages since %s\n",
		stats.TotalSessions, stats.TotalMessages, firstDate)

	return nil
}

func printHeatmap(stats statsCache, weeks int) {
	// Build date → message count map
	dateMap := make(map[string]int)
	for _, d := range stats.DailyActivity {
		dateMap[d.Date] = d.MessageCount
	}

	// Find the date range — end at today, never show future dates
	now := time.Now()
	endDate := now
	startDate := endDate.AddDate(0, 0, -7*weeks+1)

	fmt.Printf("Activity (last %d weeks)\n", weeks)

	days := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	for dow := 0; dow < 7; dow++ {
		fmt.Printf("  %s  ", days[dow])
		d := startDate
		// Advance to the right day of week (Mon=0 in our array, time.Monday=1)
		targetWeekday := time.Weekday((dow + 1) % 7) // convert our 0=Mon to Go's weekday
		for d.Weekday() != targetWeekday {
			d = d.AddDate(0, 0, 1)
		}
		for d.Before(endDate) || d.Equal(endDate) {
			dateStr := d.Format("2006-01-02")
			count := dateMap[dateStr]
			fmt.Print(heatBlock(count))
			d = d.AddDate(0, 0, 7) // next week, same day
		}
		fmt.Println()
	}
	fmt.Println("        " + strings.Repeat(" ", weeks/2-2) + "less " + heatBlock(0) + heatBlock(3) + heatBlock(10) + heatBlock(100) + " more")
}

func heatBlock(count int) string {
	switch {
	case count == 0:
		return "\033[2m.\033[0m"
	case count <= 5:
		return "\033[32m+\033[0m"
	case count <= 50:
		return "\033[33m#\033[0m"
	default:
		return "\033[31m@\033[0m"
	}
}

func printHourDistribution(stats statsCache) {
	if len(stats.HourCounts) == 0 {
		return
	}

	// Find max for scaling
	maxCount := 0
	for _, c := range stats.HourCounts {
		if c > maxCount {
			maxCount = c
		}
	}
	if maxCount == 0 {
		return
	}

	fmt.Println("Hour distribution")

	// Sort hours
	hours := make([]int, 0, 24)
	for h := 0; h < 24; h++ {
		hours = append(hours, h)
	}
	sort.Ints(hours)

	// Bar chart (height 8)
	maxHeight := 8
	for row := maxHeight; row > 0; row-- {
		fmt.Print("  ")
		for _, h := range hours {
			key := fmt.Sprintf("%d", h)
			count := stats.HourCounts[key]
			barHeight := (count * maxHeight) / maxCount
			if barHeight >= row {
				fmt.Print("\033[36m|\033[0m ")
			} else {
				fmt.Print("  ")
			}
		}
		fmt.Println()
	}
	fmt.Print("  ")
	for _, h := range hours {
		if h%3 == 0 {
			fmt.Printf("%-2d", h)
		} else {
			fmt.Print("  ")
		}
	}
	fmt.Println()
}

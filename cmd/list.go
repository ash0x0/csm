package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ash0x0/csm/internal/format"
	"github.com/ash0x0/csm/internal/session"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List Claude Code sessions",
	Aliases: []string{"ls"},
	RunE:  runList,
}

var (
	listProject    string
	listBranch     string
	listSince      string
	listMinMsgs    int
	listStale      bool
	listAll        bool
	listFzf        bool
	listJSON       bool
	listSort       string
	listOrphaned   bool
)

func init() {
	listCmd.Flags().StringVarP(&listProject, "project", "p", "", "filter by project path substring")
	listCmd.Flags().StringVarP(&listBranch, "branch", "b", "", "filter by git branch")
	listCmd.Flags().StringVarP(&listSince, "since", "s", "", "show sessions modified within duration (e.g. 7d, 30d)")
	listCmd.Flags().IntVarP(&listMinMsgs, "min-messages", "m", 0, "minimum message count")
	listCmd.Flags().BoolVar(&listStale, "stale", false, "show stale sessions (<3 msgs AND older than 14d)")
	listCmd.Flags().BoolVar(&listAll, "all", false, "include observer sessions")
	listCmd.Flags().BoolVar(&listFzf, "fzf", false, "compact output for piping to fzf")
	listCmd.Flags().BoolVar(&listJSON, "json", false, "JSON output")
	listCmd.Flags().StringVar(&listSort, "sort", "modified", "sort by: modified, created, messages, size")
	listCmd.Flags().BoolVarP(&listOrphaned, "orphaned", "O", false, "list projects whose paths no longer exist")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	if listOrphaned {
		return runListOrphaned()
	}

	scanner := session.NewScanner(claudeDir)

	var since time.Duration
	if listSince != "" {
		d, err := parseDuration(listSince)
		if err != nil {
			return fmt.Errorf("invalid --since: %w", err)
		}
		since = d
	}

	opts := session.ScanOptions{
		IncludeObservers: listAll,
		ProjectFilter:    listProject,
		BranchFilter:     listBranch,
		Since:            since,
		MinMessages:      listMinMsgs,
		Stale:            listStale,
	}

	sessions, err := scanner.Scan(opts)
	if err != nil {
		return err
	}

	sortSessions(sessions, listSort)

	switch {
	case listJSON:
		format.PrintJSON(sessions)
	case listFzf:
		format.PrintFzf(sessions)
	default:
		format.PrintTable(sessions)
	}
	return nil
}

func runListOrphaned() error {
	orphans, err := session.ListOrphanedProjects(claudeDir)
	if err != nil {
		return err
	}

	if len(orphans) == 0 {
		fmt.Println("No orphaned projects found.")
		return nil
	}

	if listJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(orphans)
	}

	fmt.Printf("%-50s  %8s  %10s\n", "PROJECT PATH", "SESSIONS", "SIZE")
	fmt.Println(strings.Repeat("─", 74))
	for _, o := range orphans {
		fmt.Printf("%-50s  %8d  %10s\n", o.OriginalPath, o.SessionCount, format.FormatSize(o.TotalSize))
	}
	fmt.Printf("\n%d orphaned projects\n", len(orphans))
	return nil
}

func sortSessions(sessions []session.SessionMeta, by string) {
	switch strings.ToLower(by) {
	case "created":
		sortBy(sessions, func(a, b session.SessionMeta) bool { return a.Created.After(b.Created) })
	case "messages", "msgs":
		sortBy(sessions, func(a, b session.SessionMeta) bool { return a.Messages > b.Messages })
	case "size":
		sortBy(sessions, func(a, b session.SessionMeta) bool { return a.FileSize > b.FileSize })
	default: // modified
		sortBy(sessions, func(a, b session.SessionMeta) bool { return a.Modified.After(b.Modified) })
	}
}

func sortBy(sessions []session.SessionMeta, less func(a, b session.SessionMeta) bool) {
	for i := 1; i < len(sessions); i++ {
		for j := i; j > 0 && less(sessions[j], sessions[j-1]); j-- {
			sessions[j], sessions[j-1] = sessions[j-1], sessions[j]
		}
	}
}

func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if strings.HasSuffix(s, "d") {
		s = strings.TrimSuffix(s, "d")
		var days int
		if _, err := fmt.Sscanf(s, "%d", &days); err != nil {
			return 0, fmt.Errorf("invalid duration: %s", s)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	if strings.HasSuffix(s, "w") {
		s = strings.TrimSuffix(s, "w")
		var weeks int
		if _, err := fmt.Sscanf(s, "%d", &weeks); err != nil {
			return 0, fmt.Errorf("invalid duration: %s", s)
		}
		return time.Duration(weeks) * 7 * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func findSession(prefix string) (*session.SessionMeta, error) {
	scanner := session.NewScanner(claudeDir)
	meta, err := scanner.FindSessionByPrefix(prefix)
	if err != nil {
		return nil, err
	}
	if meta == nil {
		return nil, fmt.Errorf("no session found matching '%s'", prefix)
	}
	return meta, nil
}

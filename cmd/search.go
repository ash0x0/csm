package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ash0x0/csm/internal/session"
	"github.com/spf13/cobra"
)

var (
	searchDeep    bool
	searchJSON    bool
	searchProject string
	searchSince   string
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search across all sessions by title or prompt content",
	Args:  cobra.ExactArgs(1),
	RunE:  runSearch,
}

func init() {
	searchCmd.Flags().BoolVar(&searchDeep, "deep", false, "also search all user prompts (slower)")
	searchCmd.Flags().BoolVar(&searchJSON, "json", false, "JSON output")
	searchCmd.Flags().StringVarP(&searchProject, "project", "p", "", "filter by project path substring")
	searchCmd.Flags().StringVar(&searchSince, "since", "", "only show sessions modified after this date (YYYY-MM-DD)")
	rootCmd.AddCommand(searchCmd)
}

type searchResult struct {
	Session session.SessionMeta `json:"session"`
	Hits    []session.SearchHit `json:"hits"`
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := args[0]
	lowerQuery := strings.ToLower(query)

	var sinceTime time.Time
	if searchSince != "" {
		var err error
		sinceTime, err = time.Parse("2006-01-02", searchSince)
		if err != nil {
			return fmt.Errorf("invalid --since date %q: use YYYY-MM-DD format", searchSince)
		}
	}

	scanner := session.NewScanner(claudeDir)
	sessions, err := scanner.Scan(session.ScanOptions{ProjectFilter: searchProject})
	if err != nil {
		return err
	}

	var results []searchResult
	for _, s := range sessions {
		if !sinceTime.IsZero() && s.Modified.Before(sinceTime) {
			continue
		}

		var hits []session.SearchHit

		// Check title first (fast)
		if strings.Contains(strings.ToLower(s.Title), lowerQuery) {
			hits = append(hits, session.SearchHit{Context: s.Title, Type: "title"})
		}

		// Search JSONL for last-prompt and optionally user prompts
		if len(hits) == 0 || searchDeep {
			fileHits, _ := session.SearchSession(s.FilePath, query, searchDeep)
			hits = append(hits, fileHits...)
		}

		if len(hits) > 0 {
			results = append(results, searchResult{Session: s, Hits: hits})
		}
	}

	if searchJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}

	if len(results) == 0 {
		fmt.Printf("No sessions matching %q\n", query)
		return nil
	}

	fmt.Printf("%d sessions matching %q\n\n", len(results), query)
	for _, r := range results {
		date := r.Session.Modified.Format("2006-01-02")
		fmt.Printf("  %s  %-40s  %s  %s\n", r.Session.ShortID, truncStr(r.Session.Title, 40), date, r.Session.Project)
		for _, h := range r.Hits {
			if h.Type != "title" {
				fmt.Printf("         [%s] %s\n", h.Type, truncStr(h.Context, 70))
			}
		}
	}
	return nil
}

func truncStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ash0x0/csm/internal/session"
	"github.com/spf13/cobra"
)

var plansJSON bool

var plansCmd = &cobra.Command{
	Use:   "plans [slug-or-prefix]",
	Short: "List or show plan files correlated with sessions",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPlans,
}

func init() {
	plansCmd.Flags().BoolVar(&plansJSON, "json", false, "JSON output")
	rootCmd.AddCommand(plansCmd)
}

type planEntry struct {
	Slug      string `json:"slug"`
	SessionID string `json:"session_id,omitempty"`
	Project   string `json:"project,omitempty"`
	Modified  string `json:"modified"`
	Path      string `json:"path"`
}

func runPlans(cmd *cobra.Command, args []string) error {
	plansDir := filepath.Join(claudeDir, "plans")

	// If a slug is given, show that plan's content
	if len(args) == 1 {
		return showPlan(plansDir, args[0])
	}

	return listPlans(plansDir)
}

func listPlans(plansDir string) error {
	entries, err := os.ReadDir(plansDir)
	if err != nil {
		return fmt.Errorf("no plans directory found")
	}

	// Build slug → session index
	slugIndex := buildSlugIndex()

	var plans []planEntry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		// Skip agent plans and active markers
		if strings.Contains(e.Name(), "-agent-") || strings.HasPrefix(e.Name(), "active") {
			continue
		}

		slug := strings.TrimSuffix(e.Name(), ".md")
		info, _ := e.Info()
		modified := ""
		if info != nil {
			modified = info.ModTime().Format(time.RFC3339)
		}

		pe := planEntry{
			Slug:     slug,
			Modified: modified,
			Path:     filepath.Join(plansDir, e.Name()),
		}

		if meta, ok := slugIndex[slug]; ok {
			pe.SessionID = meta.ShortID
			pe.Project = meta.Project
		}

		plans = append(plans, pe)
	}

	sort.Slice(plans, func(i, j int) bool {
		return plans[i].Modified > plans[j].Modified
	})

	if plansJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(plans)
	}

	if len(plans) == 0 {
		fmt.Println("No plans found.")
		return nil
	}

	fmt.Printf("%-35s  %-10s  %-40s  %s\n", "SLUG", "SESSION", "PROJECT", "MODIFIED")
	fmt.Println(strings.Repeat("-", 100))
	for _, p := range plans {
		sid := p.SessionID
		if sid == "" {
			sid = "-"
		}
		project := p.Project
		if len(project) > 40 {
			project = "..." + project[len(project)-37:]
		}
		mod := ""
		if t, err := time.Parse(time.RFC3339, p.Modified); err == nil {
			mod = formatPlanDate(t)
		}
		fmt.Printf("%-35s  %-10s  %-40s  %s\n", truncStr(p.Slug, 35), sid, project, mod)
	}
	fmt.Printf("\n%d plans\n", len(plans))
	return nil
}

func showPlan(plansDir, query string) error {
	// Try exact match first
	path := filepath.Join(plansDir, query+".md")
	if _, err := os.Stat(path); err != nil {
		// Try prefix match
		entries, _ := os.ReadDir(plansDir)
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), query) && strings.HasSuffix(e.Name(), ".md") {
				path = filepath.Join(plansDir, e.Name())
				break
			}
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("plan not found: %s", query)
	}

	fmt.Print(string(data))
	return nil
}

func buildSlugIndex() map[string]*session.SessionMeta {
	scanner := session.NewScanner(claudeDir)
	sessions, err := scanner.Scan(session.ScanOptions{})
	if err != nil {
		return nil
	}

	index := make(map[string]*session.SessionMeta)
	for i := range sessions {
		s := &sessions[i]
		if s.Slug != "" {
			index[s.Slug] = s
		}
	}
	return index
}

func formatPlanDate(t time.Time) string {
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return t.Format("15:04")
	}
	if t.Year() == now.Year() {
		return t.Format("Jan 02")
	}
	return t.Format("2006-01-02")
}

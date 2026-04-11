package format

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ash0x0/csm/internal/session"
)

func PrintTable(sessions []session.SessionMeta) {
	if len(sessions) == 0 {
		fmt.Println("No sessions found.")
		return
	}

	groups := groupByProject(sessions)
	total := 0

	for _, g := range groups {
		fmt.Printf("\n▼ %s (%d)\n", g.project, len(g.sessions))
		for _, s := range g.sessions {
			title := s.Title
			if titleRunes := []rune(title); len(titleRunes) > 45 {
				title = string(titleRunes[:42]) + "..."
			}
			branch := normalizeBranch(s.Branch)
			if branchRunes := []rune(branch); len(branchRunes) > 18 {
				branch = string(branchRunes[:15]) + "..."
			}

			marker := " "
			if s.IsActive {
				marker = "*"
			}

			fmt.Printf("%s %-8s  %4d msgs  %-45s  %-10s  %s\n",
				marker, s.ShortID, s.Messages, title, formatDate(s.Modified), branch)
			total++
		}
	}

	fmt.Printf("\n%d sessions across %d projects", total, len(groups))
	active := 0
	for _, s := range sessions {
		if s.IsActive {
			active++
		}
	}
	if active > 0 {
		fmt.Printf(" (* = active)")
	}
	fmt.Println()
}

type projectGroup struct {
	project  string
	sessions []session.SessionMeta
}

func groupByProject(sessions []session.SessionMeta) []projectGroup {
	byProject := make(map[string][]session.SessionMeta)
	var order []string
	for _, s := range sessions {
		if _, seen := byProject[s.Project]; !seen {
			order = append(order, s.Project)
		}
		byProject[s.Project] = append(byProject[s.Project], s)
	}

	// Most recently modified group first
	sort.Slice(order, func(i, j int) bool {
		return byProject[order[i]][0].Modified.After(byProject[order[j]][0].Modified)
	})

	var groups []projectGroup
	for _, proj := range order {
		groups = append(groups, projectGroup{project: proj, sessions: byProject[proj]})
	}
	return groups
}

func PrintFzf(sessions []session.SessionMeta) {
	for _, s := range sessions {
		title := s.Title
		if titleRunes := []rune(title); len(titleRunes) > 60 {
			title = string(titleRunes[:57]) + "..."
		}
		marker := ""
		if s.IsActive {
			marker = "* "
		}
		fmt.Printf("%s\t%s%s\t%s\t%d msgs\t%s\n",
			s.ShortID, marker, title, s.Project, s.Messages, formatDate(s.Modified))
	}
}

func PrintJSON(sessions []session.SessionMeta) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(sessions)
}

func PrintDetail(meta *session.SessionMeta, prompts []string) {
	fmt.Printf("Session: %s\n", meta.ID)
	fmt.Printf("Title:   %s\n", meta.Title)
	fmt.Printf("Project: %s\n", meta.Project)
	if b := normalizeBranch(meta.Branch); b != "" {
		fmt.Printf("Branch:  %s\n", b)
	}
	fmt.Printf("Created: %s\n", meta.Created.Format(time.RFC3339))
	fmt.Printf("Updated: %s\n", meta.Modified.Format(time.RFC3339))
	fmt.Printf("Msgs:    %d\n", meta.Messages)
	fmt.Printf("Size:    %s\n", formatSize(meta.FileSize))
	fmt.Printf("Path:    %s\n", meta.FilePath)
	if meta.IsActive {
		fmt.Printf("Status:  ACTIVE\n")
	}

	if len(prompts) > 0 {
		fmt.Printf("\n── Prompts ──────────────────────────────────────\n")
		for i, p := range prompts {
			fmt.Printf("  %2d. %s\n", i+1, p)
		}
	}
}

func PrintDeletePreview(sessions []session.SessionMeta) {
	if len(sessions) == 0 {
		fmt.Println("No sessions matched for deletion.")
		return
	}
	var totalSize int64
	for _, s := range sessions {
		marker := ""
		if s.IsActive {
			marker = " [ACTIVE - SKIPPED]"
		}
		fmt.Printf("  %s  %-40s  %s  %s%s\n",
			s.ShortID,
			truncateStr(s.Title, 40),
			formatSize(s.FileSize),
			formatDate(s.Modified),
			marker,
		)
		if !s.IsActive {
			totalSize += s.FileSize
		}
	}
	deletable := 0
	for _, s := range sessions {
		if !s.IsActive {
			deletable++
		}
	}
	fmt.Printf("\n%d sessions to delete (%s)\n", deletable, formatSize(totalSize))
}

func PrintStats(stats map[string]ProjectStats, total TotalStats) {
	fmt.Println("── Claude Code Storage ──────────────────────────")
	fmt.Printf("%-35s  %5s  %10s\n", "PROJECT", "COUNT", "SIZE")
	fmt.Println(strings.Repeat("─", 55))

	keys := make([]string, 0, len(stats))
	for k := range stats {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return stats[keys[i]].Size > stats[keys[j]].Size
	})
	for _, proj := range keys {
		ps := stats[proj]
		fmt.Printf("%-35s  %5d  %10s\n", truncateStr(proj, 35), ps.Count, formatSize(ps.Size))
	}

	fmt.Println(strings.Repeat("─", 55))
	fmt.Printf("Sessions:     %5d  %10s\n", total.Sessions, formatSize(total.SessionSize))
	fmt.Printf("Subagents:    %5d  %10s\n", total.Subagents, formatSize(total.SubagentSize))
	fmt.Printf("Observers:    %5d  %10s\n", total.Observers, formatSize(total.ObserverSize))
	fmt.Printf("Debug logs:   %5d  %10s\n", total.DebugLogs, formatSize(total.DebugSize))
	fmt.Printf("Tasks:        %5d  %10s\n", total.Tasks, formatSize(total.TaskSize))
	fmt.Printf("File history: %5s  %10s\n", "", formatSize(total.FileHistorySize))
	fmt.Println(strings.Repeat("─", 55))
	fmt.Printf("Total:                %10s\n", formatSize(total.TotalSize))
}

type ProjectStats struct {
	Count int
	Size  int64
}

type TotalStats struct {
	Sessions        int
	SessionSize     int64
	Subagents       int
	SubagentSize    int64
	Observers       int
	ObserverSize    int64
	DebugLogs       int
	DebugSize       int64
	Tasks           int
	TaskSize        int64
	FileHistorySize int64
	TotalSize       int64
}

func normalizeBranch(branch string) string {
	if branch == "HEAD" {
		return ""
	}
	return branch
}

func formatDate(t time.Time) string {
	if time.Since(t) < 24*time.Hour {
		return t.Format("15:04")
	}
	if time.Since(t) < 365*24*time.Hour {
		return t.Format("Jan 02")
	}
	return t.Format("2006-01-02")
}

// FormatSize formats bytes into human-readable size.
func FormatSize(bytes int64) string {
	return formatSize(bytes)
}

func formatSize(bytes int64) string {
	switch {
	case bytes >= 1024*1024*1024:
		return fmt.Sprintf("%.1f GB", float64(bytes)/1024/1024/1024)
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(bytes)/1024/1024)
	case bytes >= 1024:
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func truncateStr(s string, max int) string {
	runes := []rune(s)
	if len(runes) > max {
		return string(runes[:max-3]) + "..."
	}
	return s
}

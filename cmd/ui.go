package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/ash0x0/csm/internal/merge"
	"github.com/ash0x0/csm/internal/session"
	"github.com/spf13/cobra"
)

const (
	dimColor   = "\033[2m"
	resetColor = "\033[0m"
	cyanColor  = "\033[36m"
	expanded   = "▶"  // right arrow = can expand (collapsed)
	collapsed  = "▼"  // down arrow = can collapse (expanded)
)

var fzfLinesCmd = &cobra.Command{
	Use:    "_fzf-lines",
	Hidden: true,
	RunE:   runFzfLines,
}


var toggleGroupCmd = &cobra.Command{
	Use:    "_toggle-group",
	Hidden: true,
	Args:   cobra.ExactArgs(1), // line-index
	RunE:   runToggleGroup,
}

func init() {
	rootCmd.AddCommand(fzfLinesCmd)
	rootCmd.AddCommand(toggleGroupCmd)
	rootCmd.RunE = runUI
}

func stateFilePath() string {
	return os.TempDir() + "/csm-collapse-state"
}

func readStateFile() map[string]bool {
	data, err := os.ReadFile(stateFilePath())
	if err != nil {
		return make(map[string]bool)
	}
	return decodeCollapseState(strings.TrimSpace(string(data)))
}

func writeStateFile(set map[string]bool) {
	os.WriteFile(stateFilePath(), []byte(encodeCollapseState(set)), 0644)
}

func runFzfLines(cmd *cobra.Command, args []string) error {
	collapsedSet := readStateFile()
	lines := buildFzfLines(collapsedSet)
	fmt.Print(strings.Join(lines, "\n"))
	return nil
}

func runToggleGroup(cmd *cobra.Command, args []string) error {
	lineIdx, err := strconv.Atoi(args[0])
	if err != nil {
		collapsedSet := readStateFile()
		lines := buildFzfLines(collapsedSet)
		fmt.Print(strings.Join(lines, "\n"))
		return nil
	}

	collapsedSet := readStateFile()

	// Build current lines to find which project the line index maps to
	lines := buildFzfLines(collapsedSet)
	if lineIdx < 0 || lineIdx >= len(lines) {
		fmt.Print(strings.Join(lines, "\n"))
		return nil
	}

	project := extractProjectFromHeader(lines[lineIdx])
	if project == "" {
		for i := lineIdx - 1; i >= 0; i-- {
			if p := extractProjectFromHeader(lines[i]); p != "" {
				project = p
				break
			}
		}
	}

	if project == "" {
		fmt.Print(strings.Join(lines, "\n"))
		return nil
	}

	if collapsedSet[project] {
		delete(collapsedSet, project)
	} else {
		collapsedSet[project] = true
	}

	writeStateFile(collapsedSet)

	newLines := buildFzfLines(collapsedSet)
	fmt.Print(strings.Join(newLines, "\n"))
	return nil
}

func decodeCollapseState(s string) map[string]bool {
	set := make(map[string]bool)
	if s == "" {
		return set
	}
	var paths []string
	if err := json.Unmarshal([]byte(s), &paths); err != nil {
		return set
	}
	for _, p := range paths {
		set[p] = true
	}
	return set
}

func encodeCollapseState(set map[string]bool) string {
	paths := make([]string, 0, len(set))
	for p := range set {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	data, _ := json.Marshal(paths)
	return string(data)
}

type uiProjectGroup struct {
	project  string
	sessions []session.SessionMeta
}

func buildFzfLines(collapsedSet map[string]bool) []string {
	scanner := session.NewScanner(claudeDir)
	sessions, err := scanner.Scan(session.ScanOptions{})
	if err != nil {
		return nil
	}

	groups := groupByProjectUI(sessions)

	var lines []string
	for _, g := range groups {
		isCollapsed := collapsedSet[g.project]
		arrow := collapsed // ▼ = expanded (can collapse)
		if isCollapsed {
			arrow = expanded // ▶ = collapsed (can expand)
		}

		header := fmt.Sprintf("%s%s%s %s (%d)%s",
			dimColor, cyanColor, arrow, g.project, len(g.sessions), resetColor)
		lines = append(lines, header)

		if !isCollapsed {
			for _, s := range g.sessions {
				lines = append(lines, formatSessionLine(s))
			}
		}
	}
	return lines
}

func formatSessionLine(s session.SessionMeta) string {
	title := s.Title
	if len(title) > 45 {
		title = title[:42] + "..."
	}
	branch := s.Branch
	if branch == "HEAD" {
		branch = ""
	}
	if len(branch) > 18 {
		branch = branch[:15] + "..."
	}
	active := ""
	if s.IsActive {
		active = " *"
	}

	return fmt.Sprintf("  %s%s  %s%4d msgs%s  %-45s  %-10s  %s",
		s.ShortID, active,
		dimColor, s.Messages, resetColor,
		title,
		fmtDate(s.Modified),
		branch,
	)
}

func groupByProjectUI(sessions []session.SessionMeta) []uiProjectGroup {
	byProject := make(map[string][]session.SessionMeta)
	var order []string
	for _, s := range sessions {
		if _, seen := byProject[s.Project]; !seen {
			order = append(order, s.Project)
		}
		byProject[s.Project] = append(byProject[s.Project], s)
	}

	sort.Slice(order, func(i, j int) bool {
		return byProject[order[i]][0].Modified.After(byProject[order[j]][0].Modified)
	})

	var groups []uiProjectGroup
	for _, proj := range order {
		groups = append(groups, uiProjectGroup{project: proj, sessions: byProject[proj]})
	}
	return groups
}

func extractProjectFromHeader(line string) string {
	clean := stripAnsi(line)
	clean = strings.TrimSpace(clean)
	if !strings.HasPrefix(clean, collapsed) && !strings.HasPrefix(clean, expanded) {
		return ""
	}
	clean = strings.TrimPrefix(clean, collapsed)
	clean = strings.TrimPrefix(clean, expanded)
	clean = strings.TrimSpace(clean)
	if idx := strings.LastIndex(clean, " ("); idx > 0 {
		clean = clean[:idx]
	}
	return strings.TrimSpace(clean)
}

func stripAnsi(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\033' {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			i++
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	return result.String()
}

func runUI(cmd *cobra.Command, args []string) error {
	if _, err := exec.LookPath("fzf"); err != nil {
		return fmt.Errorf("fzf is required (install with: nix profile install nixpkgs#fzf)")
	}

	writeStateFile(make(map[string]bool))

	for {
		action, ids, err := launchFzf()
		if err != nil {
			return nil
		}

		switch action {
		case "enter":
			if len(ids) >= 2 {
				return doMerge(ids)
			}
			return nil
		case "ctrl-o":
			if len(ids) == 1 {
				if err := doMove(ids[0]); err != nil {
					fmt.Fprintf(os.Stderr, "move: %v\n", err)
				}
				continue // re-enter fzf
			}
			return nil
		default:
			return nil
		}
	}
}

func launchFzf() (action string, ids []string, err error) {
	collapsedSet := readStateFile()
	lines := buildFzfLines(collapsedSet)
	if len(lines) == 0 {
		return "", nil, fmt.Errorf("no sessions found")
	}

	input := strings.Join(lines, "\n")
	csmBin, _ := os.Executable()

	header := "TAB select  ENTER merge  ctrl-d delete  ctrl-r rename  ctrl-o move  ctrl-g fold/unfold  ESC quit"

	fzfCmd := exec.Command("fzf",
		"--multi",
		"--ansi",
		"--no-sort",
		"--layout=reverse",
		"--expect", "enter,ctrl-o",
		"--header", header,
		"--prompt", "csm> ",
		"--preview", csmBin+" show {1}",
		"--preview-window", "right:50%:wrap",
		"--bind", fmt.Sprintf("ctrl-d:execute-silent(%s rm --force {1})+reload(%s _fzf-lines)", csmBin, csmBin),
		"--bind", fmt.Sprintf("ctrl-r:execute(%s _rename-tty {1})+reload(%s _fzf-lines)", csmBin, csmBin),
		"--bind", fmt.Sprintf("ctrl-g:reload(%s _toggle-group {n})", csmBin),
	)
	fzfCmd.Stdin = strings.NewReader(input)
	fzfCmd.Stderr = os.Stderr

	out, err := fzfCmd.Output()
	if err != nil {
		return "", nil, err
	}

	outputLines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(outputLines) == 0 {
		return "", nil, fmt.Errorf("cancelled")
	}

	action = strings.TrimSpace(outputLines[0])

	for _, line := range outputLines[1:] {
		clean := stripAnsi(strings.TrimSpace(line))
		if clean == "" || strings.HasPrefix(clean, collapsed) || strings.HasPrefix(clean, expanded) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 1 {
			ids = append(ids, fields[0])
		}
	}

	return action, ids, nil
}

func doMove(id string) error {
	meta, err := findSession(id)
	if err != nil {
		return err
	}

	dest, err := pickProject(meta.Project)
	if err != nil {
		return err
	}

	if dest == meta.Project {
		fmt.Println("Session is already in that project.")
		return nil
	}

	_, err = session.MoveSession(claudeDir, meta, dest)
	if err != nil {
		return err
	}

	fmt.Printf("Moved %s (%s) → %s\n", meta.ShortID, meta.Title, dest)
	return nil
}

func doMerge(ids []string) error {
	var metas []*session.SessionMeta
	for _, id := range ids {
		meta, err := findSession(id)
		if err != nil {
			return err
		}
		metas = append(metas, meta)
	}

	fmt.Printf("\nMerging %d sessions:\n", len(metas))
	for i, m := range metas {
		fmt.Printf("  %d. %s (%s, %d msgs)\n", i+1, m.ShortID, m.Title, m.Messages)
	}
	fmt.Println()

	opts := merge.MergeOptions{}

	newID, err := merge.MergeN(metas, opts)
	if err != nil {
		return fmt.Errorf("merge failed: %w", err)
	}

	fmt.Printf("Created merged session: %s\n", newID)
	fmt.Printf("Resume with: claude --resume %s\n", newID[:8])
	return nil
}

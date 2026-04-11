package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

var listDirsCmd = &cobra.Command{
	Use:    "_list-dirs <prefix>",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE:   runListDirs,
}

func init() {
	rootCmd.AddCommand(fzfLinesCmd)
	rootCmd.AddCommand(toggleGroupCmd)
	rootCmd.AddCommand(listDirsCmd)
	rootCmd.RunE = runUI
}

func runListDirs(cmd *cobra.Command, args []string) error {
	prefix := args[0]
	if strings.HasPrefix(prefix, "~") {
		home, _ := os.UserHomeDir()
		prefix = home + prefix[1:]
	}

	// List directories under the parent of the prefix
	dir := prefix
	base := ""
	if info, err := os.Stat(prefix); err != nil || !info.IsDir() {
		dir = filepath.Dir(prefix)
		base = filepath.Base(prefix)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // silent fail for fzf reload
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if base != "" && !strings.HasPrefix(strings.ToLower(e.Name()), strings.ToLower(base)) {
			continue
		}
		fmt.Println(filepath.Join(dir, e.Name()))
	}
	return nil
}

// checkFzfVersion returns an error if fzf is older than 0.47.0,
// which introduced the `transform` action used by the TUI.
func checkFzfVersion() error {
	major, minor := getFzfVersion()
	if major == 0 && minor < 30 {
		return fmt.Errorf("fzf 0.30.0+ required (found %d.%d) — upgrade with: nix profile install nixpkgs#fzf", major, minor)
	}
	return nil
}

// getFzfVersion returns the installed fzf major and minor version numbers.
// Returns (0, 0) if the version cannot be determined.
func getFzfVersion() (major, minor int) {
	out, err := exec.Command("fzf", "--version").Output()
	if err != nil {
		return 0, 0
	}
	// output is like "0.47.0 (revision)"
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return 0, 0
	}
	parts := strings.Split(fields[0], ".")
	if len(parts) < 2 {
		return 0, 0
	}
	major, _ = strconv.Atoi(parts[0])
	minor, _ = strconv.Atoi(parts[1])
	return major, minor
}

func stateFilePath() string {
	return fmt.Sprintf("%s/csm-collapse-%d", os.TempDir(), os.Getuid())
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

	// Append orphaned projects
	orphans, _ := session.ListOrphanedProjects(claudeDir)
	if len(orphans) > 0 {
		lines = append(lines, fmt.Sprintf("%s── Orphaned Projects ──────────────────%s", dimColor, resetColor))

		for _, o := range orphans {
			// Scan sessions in the orphaned project dir
			orphanSessions, _ := scanner.Scan(session.ScanOptions{ProjectFilter: o.OriginalPath})
			if len(orphanSessions) == 0 {
				continue
			}

			isCollapsed := collapsedSet[o.OriginalPath]
			arrow := collapsed
			if isCollapsed {
				arrow = expanded
			}

			header := fmt.Sprintf("%s%s%s %s (%d)  \033[33m⚠ path not found%s",
				dimColor, cyanColor, arrow, o.OriginalPath, len(orphanSessions), resetColor)
			lines = append(lines, header)

			if !isCollapsed {
				for _, s := range orphanSessions {
					lines = append(lines, formatSessionLine(s))
				}
			}
		}
	}

	return lines
}

func formatSessionLine(s session.SessionMeta) string {
	title := s.Title
	if titleRunes := []rune(title); len(titleRunes) > 45 {
		title = string(titleRunes[:42]) + "..."
	}
	branch := s.Branch
	if branch == "HEAD" {
		branch = ""
	}
	if branchRunes := []rune(branch); len(branchRunes) > 18 {
		branch = string(branchRunes[:15]) + "..."
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
	if err := checkFzfVersion(); err != nil {
		return err
	}

	fzfMajor, fzfMinor := getFzfVersion()
	writeStateFile(make(map[string]bool))

	for {
		action, ids, query, err := launchFzf(fzfMajor, fzfMinor)
		if err != nil {
			return nil
		}

		switch action {
		case "enter":
			if len(ids) >= 2 {
				if err := doMerge(ids); err != nil {
					fmt.Fprintf(os.Stderr, "merge: %v\n", err)
				}
				pause()
				continue
			}
			if len(ids) == 1 {
				fmt.Fprintf(os.Stderr, "select at least 2 sessions to merge\n")
				pause()
				continue
			}
			// 0 ids: Enter pressed on a group header or with nothing visible — stay in TUI
			continue
		case "ctrl-o":
			if len(ids) == 1 {
				if err := doMove(ids[0]); err != nil {
					fmt.Fprintf(os.Stderr, "move: %v\n", err)
				}
				continue
			}
			return nil
		case "ctrl-f":
			if len(ids) == 2 {
				if err := doDiff(ids[0], ids[1]); err != nil {
					fmt.Fprintf(os.Stderr, "diff: %v\n", err)
				}
				pause()
				continue
			}
			fmt.Fprintf(os.Stderr, "diff requires exactly 2 selected sessions\n")
			continue
		case "ctrl-t":
			if len(ids) == 1 {
				runTasks(nil, []string{ids[0]})
			}
			pause()
			continue
		case "ctrl-l":
			if len(ids) == 1 {
				runTimeline(nil, []string{ids[0]})
			}
			pause()
			continue
		case "ctrl-p":
			runPlans(nil, nil)
			pause()
			continue
		case "ctrl-a":
			runActivity(nil, nil)
			pause()
			continue
		case "ctrl-s":
			if query != "" {
				runSearch(nil, []string{query})
			} else {
				fmt.Fprintf(os.Stderr, "type a query first, then press ctrl-s to search\n")
			}
			pause()
			continue
		default:
			return nil
		}
	}
}

func launchFzf(fzfMajor, fzfMinor int) (action string, ids []string, query string, err error) {
	collapsedSet := readStateFile()
	lines := buildFzfLines(collapsedSet)
	if len(lines) == 0 {
		return "", nil, "", fmt.Errorf("no sessions found")
	}

	input := strings.Join(lines, "\n")
	csmBin, _ := os.Executable()

	// fzf >= 0.47 supports `transform`, which lets space conditionally reload
	// (for group headers) or toggle-select (for session lines) without a reload
	// on every keypress. Older fzf doesn't support transform, and reload clears
	// multi-select state, so we fall back to plain toggle (no group fold).
	hasTransform := fzfMajor >= 1 || (fzfMajor == 0 && fzfMinor >= 47)

	var spaceBinding, header string
	if hasTransform {
		spaceBinding = fmt.Sprintf("space:transform(echo {} | grep -q '[▼▶]' && echo 'reload(%s _toggle-group {n})' || echo 'toggle')", csmBin)
		header = "SPACE select/fold  ENTER merge  ctrl-d del  ctrl-o move  ctrl-f diff\nctrl-t tasks  ctrl-l timeline  ctrl-s search  ctrl-p plans  ctrl-a activity  ESC quit"
	} else {
		// No group fold on older fzf — space and tab both select
		spaceBinding = "space:toggle"
		header = "SPACE/TAB select  ENTER merge  ctrl-d del  ctrl-o move  ctrl-f diff\nctrl-t tasks  ctrl-l timeline  ctrl-s search  ctrl-p plans  ctrl-a activity  ESC quit"
	}

	fzfCmd := exec.Command("fzf",
		"--multi",
		"--ansi",
		"--no-sort",
		"--layout=reverse",
		"--header-first",
		"--print-query",
		"--expect", "enter,ctrl-o,ctrl-f,ctrl-t,ctrl-l,ctrl-p,ctrl-a,ctrl-s",
		"--header", header,
		"--prompt", "csm> ",
		"--preview", csmBin+" show {1}",
		"--preview-window", "right:50%:wrap",
		"--bind", fmt.Sprintf("ctrl-d:execute(%s rm --force {1})+reload(%s _fzf-lines)", csmBin, csmBin),
		"--bind", spaceBinding,
	)
	fzfCmd.Stdin = strings.NewReader(input)
	fzfCmd.Stderr = os.Stderr

	out, err := fzfCmd.Output()
	if err != nil {
		return "", nil, "", err
	}

	// --print-query output: line 0 = query (may be empty), line 1 = action key, line 2+ = selected items
	// Don't TrimSpace the whole output — empty query line would be stripped
	outputLines := strings.Split(string(out), "\n")
	if len(outputLines) < 2 {
		return "", nil, "", fmt.Errorf("cancelled")
	}

	query = strings.TrimSpace(outputLines[0])
	action = strings.TrimSpace(outputLines[1])

	for _, line := range outputLines[2:] {
		clean := stripAnsi(strings.TrimSpace(line))
		if clean == "" || strings.HasPrefix(clean, collapsed) || strings.HasPrefix(clean, expanded) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 1 {
			ids = append(ids, fields[0])
		}
	}

	return action, ids, query, nil
}

func pause() {
	fmt.Println("\nPress enter to continue...")
	fmt.Scanln()
}

func doMove(id string) error {
	meta, err := findSession(id)
	if err != nil {
		return err
	}

	if meta.IsActive {
		return fmt.Errorf("session %s is active — cannot move a running session", meta.ShortID)
	}

	dest, err := pickProject(meta.Project)
	if err != nil {
		return err
	}

	if dest == meta.Project {
		fmt.Println("Session is already in that project.")
		return nil
	}

	newPath, err := session.MoveSession(claudeDir, meta, dest)
	if err != nil {
		return err
	}

	MarkDirty(filepath.Dir(meta.FilePath))
	MarkDirty(filepath.Dir(newPath))
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

	for _, m := range metas {
		MarkDirty(filepath.Dir(m.FilePath))
	}

	fmt.Printf("Created merged session: %s\n", newID)
	fmt.Printf("Resume with: claude --resume %s\n", newID[:8])
	return nil
}

func doDiff(idA, idB string) error {
	metaA, err := findSession(idA)
	if err != nil {
		return err
	}
	metaB, err := findSession(idB)
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

	fmt.Println()
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


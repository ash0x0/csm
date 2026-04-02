package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ash0x0/csm/internal/session"
	"github.com/spf13/cobra"
)

var moveCmd = &cobra.Command{
	Use:   "move <session-id> [destination-project]",
	Short: "Move a session to a different project",
	Long:  "With one arg, opens a picker to choose destination. With two args, moves directly.",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runMove,
}

func init() {
	rootCmd.AddCommand(moveCmd)
}

func runMove(cmd *cobra.Command, args []string) error {
	meta, err := findSession(args[0])
	if err != nil {
		return err
	}

	var destProject string
	if len(args) >= 2 {
		destProject = args[1]
	} else {
		dest, err := pickProject(meta.Project)
		if err != nil {
			return err
		}
		destProject = dest
	}

	if destProject == meta.Project {
		fmt.Println("Session is already in that project.")
		return nil
	}

	newPath, err := session.MoveSession(claudeDir, meta, destProject)
	if err != nil {
		return fmt.Errorf("move failed: %w", err)
	}

	MarkDirty(filepath.Dir(meta.FilePath))
	MarkDirty(filepath.Dir(newPath))

	fmt.Printf("Moved %s (%s)\n  from: %s\n  to:   %s\n", meta.ShortID, meta.Title, meta.Project, destProject)
	return nil
}

func pickProject(currentProject string) (string, error) {
	if _, err := exec.LookPath("fzf"); err != nil {
		return "", fmt.Errorf("fzf is required for interactive mode")
	}

	projects, err := session.ListProjects(claudeDir)
	if err != nil {
		return "", err
	}

	var lines []string
	for _, p := range projects {
		marker := "  "
		if p == currentProject {
			marker = "* "
		}
		lines = append(lines, marker+p)
	}

	csmBin, _ := os.Executable()
	fzfCmd := exec.Command("fzf",
		"--print-query",
		"--header", "Select project or type a path (ESC to cancel)  (* = current)",
		"--prompt", "move to> ",
		"--layout=reverse",
		"--bind", "enter:accept-or-print-query",
		"--bind", fmt.Sprintf("change:reload(%s _list-dirs {q} 2>/dev/null || true)", csmBin),
	)
	fzfCmd.Stdin = strings.NewReader(strings.Join(lines, "\n"))
	fzfCmd.Stderr = os.Stderr

	out, err := fzfCmd.Output()
	if err != nil {
		return "", fmt.Errorf("cancelled")
	}

	// --print-query outputs: line 1 = query, line 2 = selected item (if any)
	outputLines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var result string
	if len(outputLines) >= 2 && outputLines[1] != "" {
		result = outputLines[1] // user selected from list
	} else {
		result = outputLines[0] // user typed a path
	}

	result = strings.TrimSpace(result)
	result = strings.TrimPrefix(result, "* ")
	result = strings.TrimPrefix(result, "  ")

	// Expand ~
	if strings.HasPrefix(result, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot expand home directory: %w", err)
		}
		result = home + result[1:]
	}

	// Validate path exists
	if _, err := os.Stat(result); err != nil {
		return "", fmt.Errorf("path does not exist: %s", result)
	}

	return result, nil
}

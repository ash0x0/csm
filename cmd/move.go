package cmd

import (
	"fmt"
	"os"
	"os/exec"
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

	fmt.Printf("Moved %s (%s)\n  from: %s\n  to:   %s\n", meta.ShortID, meta.Title, meta.Project, destProject)
	_ = newPath
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

	fzfCmd := exec.Command("fzf",
		"--header", "Select destination project (* = current)",
		"--prompt", "move to> ",
		"--layout=reverse",
	)
	fzfCmd.Stdin = strings.NewReader(strings.Join(lines, "\n"))
	fzfCmd.Stderr = os.Stderr

	out, err := fzfCmd.Output()
	if err != nil {
		return "", fmt.Errorf("cancelled")
	}

	line := strings.TrimSpace(string(out))
	// Strip marker prefix
	line = strings.TrimPrefix(line, "* ")
	line = strings.TrimPrefix(line, "  ")
	return line, nil
}

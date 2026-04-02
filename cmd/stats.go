package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ash0x0/csm/internal/format"
	"github.com/ash0x0/csm/internal/session"
	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show disk usage breakdown of Claude Code storage",
	RunE:  runStats,
}

func init() {
	rootCmd.AddCommand(statsCmd)
}

func runStats(cmd *cobra.Command, args []string) error {
	projectsDir := filepath.Join(claudeDir, "projects")
	projEntries, err := os.ReadDir(projectsDir)
	if err != nil {
		return err
	}

	projStats := make(map[string]format.ProjectStats)
	var total format.TotalStats

	for _, pe := range projEntries {
		if !pe.IsDir() {
			continue
		}
		projPath := filepath.Join(projectsDir, pe.Name())
		projName := session.DecodeProjectPath(pe.Name())

		isObserver := strings.Contains(pe.Name(), session.ObserverToken)

		// Count top-level JSONL files
		jsonlFiles, _ := filepath.Glob(filepath.Join(projPath, "*.jsonl"))
		var projSize int64
		for _, f := range jsonlFiles {
			if info, err := os.Stat(f); err == nil {
				projSize += info.Size()
			}
		}

		if isObserver {
			total.Observers += len(jsonlFiles)
			total.ObserverSize += projSize
		} else {
			total.Sessions += len(jsonlFiles)
			total.SessionSize += projSize
			if len(jsonlFiles) > 0 {
				projStats[projName] = format.ProjectStats{Count: len(jsonlFiles), Size: projSize}
			}
		}

		// Count subagent dirs
		subDirs, _ := filepath.Glob(filepath.Join(projPath, "*/subagents"))
		for _, sd := range subDirs {
			subFiles, _ := filepath.Glob(filepath.Join(sd, "*"))
			total.Subagents += len(subFiles)
			for _, sf := range subFiles {
				if info, err := os.Stat(sf); err == nil {
					total.SubagentSize += info.Size()
				}
			}
		}
	}

	// Debug logs
	debugDir := filepath.Join(claudeDir, "debug")
	if entries, err := os.ReadDir(debugDir); err == nil {
		for _, e := range entries {
			total.DebugLogs++
			if info, err := e.Info(); err == nil {
				total.DebugSize += info.Size()
			}
		}
	}

	// Tasks
	taskDir := filepath.Join(claudeDir, "tasks")
	if entries, err := os.ReadDir(taskDir); err == nil {
		for _, e := range entries {
			total.Tasks++
			dirPath := filepath.Join(taskDir, e.Name())
			total.TaskSize += dirSize(dirPath)
		}
	}

	// File history
	fhDir := filepath.Join(claudeDir, "file-history")
	total.FileHistorySize = dirSize(fhDir)

	total.TotalSize = total.SessionSize + total.SubagentSize + total.ObserverSize +
		total.DebugSize + total.TaskSize + total.FileHistorySize

	format.PrintStats(projStats, total)
	return nil
}

func dirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, _ error) error {
		if info != nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

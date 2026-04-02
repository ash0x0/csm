package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ReadTasks reads all task JSON files for a session from ~/.claude/tasks/<sessionID>/.
func ReadTasks(claudeDir, sessionID string) ([]Task, error) {
	taskDir := filepath.Join(claudeDir, "tasks", sessionID)
	entries, err := os.ReadDir(taskDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var tasks []Task
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		// Skip non-numeric filenames (.highwatermark, .lock)
		base := strings.TrimSuffix(e.Name(), ".json")
		if _, err := strconv.Atoi(base); err != nil {
			continue
		}

		data, err := os.ReadFile(filepath.Join(taskDir, e.Name()))
		if err != nil {
			continue
		}
		var t Task
		if json.Unmarshal(data, &t) == nil {
			tasks = append(tasks, t)
		}
	}

	sort.Slice(tasks, func(i, j int) bool {
		a, _ := strconv.Atoi(tasks[i].ID)
		b, _ := strconv.Atoi(tasks[j].ID)
		return a < b
	})

	return tasks, nil
}

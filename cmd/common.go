package cmd

import (
	"fmt"
	"strings"
	"time"
)

// parseSinceFlag parses a --since value as either a duration (7d, 2w, 30m)
// or an absolute date (YYYY-MM-DD) and returns the cutoff time.
func parseSinceFlag(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	// Try absolute date first
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	// Try duration (7d, 2w, or Go duration like 30m)
	d, err := parseDuration(s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid --since %q: use a duration (7d, 2w) or date (YYYY-MM-DD)", s)
	}
	return time.Now().Add(-d), nil
}

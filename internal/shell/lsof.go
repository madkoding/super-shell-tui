package shell

import "strings"

// parseLsofCwd extracts the path from `lsof -Fn` output, where each field is
// a line starting with its tag ("p1234", "fcwd", "n/path").
func parseLsofCwd(out string) string {
	for _, line := range strings.Split(out, "\n") {
		if p, ok := strings.CutPrefix(line, "n"); ok && p != "" {
			return p
		}
	}
	return ""
}

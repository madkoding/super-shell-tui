package shell

import (
	"os"
	"strconv"
	"strings"
)

// processCwd reads the working directory of pid from /proc.
func processCwd(pid int) string {
	dir, err := os.Readlink("/proc/" + strconv.Itoa(pid) + "/cwd")
	if err != nil {
		return ""
	}
	return dir
}

// processCommand reads the command line of pid from /proc.
func processCommand(pid int) string {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cmdline")
	if err != nil || len(data) == 0 {
		return ""
	}
	return shellJoin(strings.Split(strings.TrimRight(string(data), "\x00"), "\x00"))
}

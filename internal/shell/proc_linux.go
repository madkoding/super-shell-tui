package shell

import (
	"os"
	"strconv"
)

// processCwd reads the working directory of pid from /proc.
func processCwd(pid int) string {
	dir, err := os.Readlink("/proc/" + strconv.Itoa(pid) + "/cwd")
	if err != nil {
		return ""
	}
	return dir
}

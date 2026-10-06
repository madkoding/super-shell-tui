package shell

import (
	"os/exec"
	"strconv"
)

// processCwd asks lsof for the working directory of pid; macOS has no /proc
// and the libproc call would need cgo.
func processCwd(pid int) string {
	out, err := exec.Command("lsof", "-a", "-p", strconv.Itoa(pid), "-d", "cwd", "-Fn").Output()
	if err != nil {
		return ""
	}
	return parseLsofCwd(string(out))
}

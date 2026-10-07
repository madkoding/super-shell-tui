package shell

import (
	"os/exec"
	"strconv"
	"strings"
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

// processCommand asks ps for the command line of pid. ps joins the
// arguments with spaces, so quoting inside them is lost.
func processCommand(pid int) string {
	out, err := exec.Command("ps", "-o", "command=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

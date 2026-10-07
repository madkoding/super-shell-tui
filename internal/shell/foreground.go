package shell

import (
	"regexp"
	"strings"

	"golang.org/x/sys/unix"
)

// ForegroundCommand returns the command line of the program running in the
// foreground of the shell (e.g. "npm run dev"), or "" when the shell is at
// its prompt or the command cannot be read.
func (s *Session) ForegroundCommand() string {
	conn, err := s.pty.SyscallConn() // Fd() would switch the PTY to blocking mode
	if err != nil {
		return ""
	}
	pgid := 0
	_ = conn.Control(func(fd uintptr) {
		pgid, err = unix.IoctlGetInt(int(fd), unix.TIOCGPGRP)
	})
	if err != nil || pgid <= 0 || pgid == s.Pid() {
		return ""
	}
	return processCommand(pgid)
}

var safeArg = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

// shellJoin quotes args so the shell reads them back as the same words.
func shellJoin(args []string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		if safeArg.MatchString(a) {
			quoted[i] = a
		} else {
			quoted[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		}
	}
	return strings.Join(quoted, " ")
}

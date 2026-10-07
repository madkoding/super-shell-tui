package shell

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// ForegroundCommand returns the command line of the program running in the
// foreground of the shell (e.g. "npm run dev"), or "" when the shell is at
// its prompt or the command cannot be read.
func (s *Session) ForegroundCommand() string {
	pgid, err := s.foregroundGroup()
	if err != nil || pgid <= 0 || pgid == s.Pid() {
		return ""
	}
	return processCommand(pgid)
}

// Busy reports whether a program other than the shell is in the
// foreground. When that can't be told, it says yes, so callers err on the
// side of asking before killing anything.
func (s *Session) Busy() bool {
	pgid, err := s.foregroundGroup()
	return err != nil || pgid != s.Pid()
}

// foregroundGroup returns the PTY's foreground process group.
func (s *Session) foregroundGroup() (int, error) {
	conn, err := s.pty.SyscallConn() // Fd() would switch the PTY to blocking mode
	if err != nil {
		return 0, err
	}
	pgid := 0
	if cerr := conn.Control(func(fd uintptr) {
		pgid, err = unix.IoctlGetInt(int(fd), unix.TIOCGPGRP)
	}); cerr != nil {
		return 0, cerr
	}
	return pgid, err
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

// Label names what the shell is doing, for pane borders: the program in
// the foreground, or the base name of the working directory when idle.
// Like Cwd it is cached briefly because it is read on every redraw.
func (s *Session) Label() string {
	s.cwdMu.Lock()
	if time.Since(s.labelAt) < cwdTTL {
		defer s.cwdMu.Unlock()
		return s.label
	}
	s.cwdMu.Unlock()
	label := ""
	if words := strings.Fields(s.ForegroundCommand()); len(words) > 0 {
		label = filepath.Base(words[0])
	} else if dir := s.Cwd(); dir != "" {
		label = filepath.Base(dir)
	}
	s.cwdMu.Lock()
	s.label, s.labelAt = label, time.Now()
	s.cwdMu.Unlock()
	return label
}

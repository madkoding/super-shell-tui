// Package shell runs an interactive shell inside a PTY and keeps a virtual
// terminal (vt10x) in sync with its output so the UI can render it as a pane.
package shell

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"strconv"
	"sync"

	"github.com/creack/pty"
	"github.com/hinshun/vt10x"
)

// Session is a shell process attached to a PTY plus its emulated screen.
type Session struct {
	cmd  *exec.Cmd
	pty  *os.File
	term vt10x.Terminal

	mu         sync.Mutex
	cols, rows int

	updates chan struct{}
	done    chan struct{}
	err     error
}

// Start launches shellPath (e.g. /bin/bash) in a new PTY of cols x rows.
func Start(shellPath string, cols, rows int) (*Session, error) {
	if cols < 1 {
		cols = 80
	}
	if rows < 1 {
		rows = 24
	}

	cmd := exec.Command(shellPath, "-i")
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "SUPER_SHELL=1")

	f, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		return nil, err
	}

	s := &Session{
		cmd:     cmd,
		pty:     f,
		cols:    cols,
		rows:    rows,
		updates: make(chan struct{}, 1),
		done:    make(chan struct{}),
	}
	// Terminal replies (cursor position reports, device attributes) go back
	// to the shell through the PTY, as a real terminal would do.
	s.term = vt10x.New(vt10x.WithWriter(f), vt10x.WithSize(cols, rows))

	go s.readLoop()
	return s, nil
}

func (s *Session) readLoop() {
	defer close(s.done)
	buf := make([]byte, 32*1024)
	pending := 0 // bytes of an incomplete UTF-8 rune kept from the last read
	for {
		n, err := s.pty.Read(buf[pending:])
		if n > 0 {
			total := pending + n
			// vt10x stops before a rune split across reads; carry it over.
			w, _ := s.term.Write(buf[:total])
			pending = copy(buf, buf[w:total])
			s.notify()
		}
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrClosed) {
				s.err = err
			}
			_ = s.cmd.Wait()
			return
		}
	}
}

// notify signals a pending redraw without blocking; bursts are coalesced.
func (s *Session) notify() {
	select {
	case s.updates <- struct{}{}:
	default:
	}
}

// Write sends raw bytes (keystrokes) straight to the shell's PTY.
func (s *Session) Write(p []byte) (int, error) { return s.pty.Write(p) }

// Updates fires whenever the screen changed.
func (s *Session) Updates() <-chan struct{} { return s.updates }

// Done is closed when the shell exits.
func (s *Session) Done() <-chan struct{} { return s.done }

// Err returns the read error that ended the session, if any.
func (s *Session) Err() error { return s.err }

// Pid returns the shell process id.
func (s *Session) Pid() int { return s.cmd.Process.Pid }

// Size returns the current pane size in cells.
func (s *Session) Size() (cols, rows int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cols, s.rows
}

// Resize updates both the PTY (sends SIGWINCH to the shell) and the emulator.
func (s *Session) Resize(cols, rows int) error {
	if cols < 1 || rows < 1 {
		return nil
	}
	s.mu.Lock()
	if cols == s.cols && rows == s.rows {
		s.mu.Unlock()
		return nil
	}
	s.cols, s.rows = cols, rows
	s.mu.Unlock()

	s.term.Resize(cols, rows)
	err := pty.Setsize(s.pty, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	s.notify()
	return err
}

// AppCursorMode reports whether the shell enabled application cursor keys
// (DECCKM). Arrow keys must then be sent as SS3 (ESC O A) instead of CSI.
func (s *Session) AppCursorMode() bool {
	s.term.Lock()
	defer s.term.Unlock()
	return s.term.Mode()&vt10x.ModeAppCursor != 0
}

// Title returns the window title set by the shell (OSC 0/2).
func (s *Session) Title() string {
	s.term.Lock()
	defer s.term.Unlock()
	return s.term.Title()
}

// Cwd returns the shell's working directory (Linux only, best effort).
func (s *Session) Cwd() string {
	dir, err := os.Readlink("/proc/" + strconv.Itoa(s.Pid()) + "/cwd")
	if err != nil {
		return ""
	}
	return dir
}

// Close terminates the shell and releases the PTY.
func (s *Session) Close() error {
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Signal(os.Kill)
	}
	return s.pty.Close()
}

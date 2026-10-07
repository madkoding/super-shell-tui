// Package shell runs an interactive shell inside a PTY and keeps a virtual
// terminal (charmbracelet/x/vt) in sync with its output so the UI can render
// it as a pane, including scrollback and wide characters.
package shell

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

// Session is a shell process attached to a PTY plus its emulated screen.
type Session struct {
	cmd *exec.Cmd
	pty *os.File

	mu         sync.Mutex // guards emu, cols, rows, scroll, title
	emu        *vt.Emulator
	cols, rows int
	scroll     int // lines scrolled back into history; 0 = live view
	title      string
	sel        selection

	appCursor     atomic.Bool
	bracketed     atomic.Bool
	cursorVisible atomic.Bool
	altScreen     atomic.Bool
	mouseMode     atomic.Int32 // inner mouse tracking: 0, 9, 1000, 1002 or 1003
	mouseSGR      atomic.Bool
	outputs       atomic.Uint64 // chunks read from the shell, for activity marks
	resizedAt     atomic.Int64  // unix nanos of the last effective resize

	updates chan struct{}
	done    chan struct{}
	err     error
}

// Start launches shellPath (e.g. /bin/bash) in a new PTY of cols x rows,
// keeping up to scrollback lines of history.
func Start(shellPath string, cols, rows, scrollback int) (*Session, error) {
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
	s.cursorVisible.Store(true)

	s.emu = vt.NewEmulator(cols, rows)
	s.emu.SetScrollbackSize(scrollback)
	s.emu.SetCallbacks(vt.Callbacks{
		Title:            func(t string) { s.title = t },
		AltScreen:        func(on bool) { s.altScreen.Store(on) },
		CursorVisibility: func(v bool) { s.cursorVisible.Store(v) },
		EnableMode:       func(m ansi.Mode) { s.setMode(m, true) },
		DisableMode:      func(m ansi.Mode) { s.setMode(m, false) },
	})

	// Terminal replies (cursor position reports, device attributes) go back
	// to the shell through the PTY, as a real terminal would do. The
	// emulator's reply pipe is synchronous, so it must always be drained.
	go func() { _, _ = io.Copy(f, s.emu) }()
	go s.readLoop()
	return s, nil
}

func (s *Session) setMode(m ansi.Mode, on bool) {
	switch m {
	case ansi.ModeCursorKeys:
		s.appCursor.Store(on)
	case ansi.ModeBracketedPaste:
		s.bracketed.Store(on)
	case ansi.ModeMouseExtSgr:
		s.mouseSGR.Store(on)
	case ansi.ModeMouseX10, ansi.ModeMouseNormal, ansi.ModeMouseButtonEvent, ansi.ModeMouseAnyEvent:
		code := int32(m.Mode())
		if on {
			s.mouseMode.Store(code)
		} else {
			s.mouseMode.CompareAndSwap(code, 0)
		}
	}
}

func (s *Session) readLoop() {
	defer close(s.done)
	buf := make([]byte, 32*1024)
	for {
		n, err := s.pty.Read(buf)
		if n > 0 {
			s.mu.Lock()
			_, _ = s.emu.Write(buf[:n])
			s.mu.Unlock()
			s.outputs.Add(1)
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

// Outputs counts chunks of shell output; it changes only on real output,
// not on redraws caused by scrolling, selection or resizing.
func (s *Session) Outputs() uint64 { return s.outputs.Load() }

// SinceResize returns the time elapsed since the last resize. Shells redraw
// their prompt on SIGWINCH, which should not count as new activity.
func (s *Session) SinceResize() time.Duration {
	return time.Duration(time.Now().UnixNano() - s.resizedAt.Load())
}

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
	s.resizedAt.Store(time.Now().UnixNano())
	s.emu.Resize(cols, rows)
	s.scroll = min(s.scroll, s.emu.ScrollbackLen())
	s.mu.Unlock()

	err := pty.Setsize(s.pty, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	s.notify()
	return err
}

// AppCursorMode reports whether the shell enabled application cursor keys
// (DECCKM). Arrow keys must then be sent as SS3 (ESC O A) instead of CSI.
func (s *Session) AppCursorMode() bool { return s.appCursor.Load() }

// BracketedPaste reports whether the shell enabled bracketed paste (?2004).
func (s *Session) BracketedPaste() bool { return s.bracketed.Load() }

// AltScreen reports whether a full-screen app (vim, less…) is running.
func (s *Session) AltScreen() bool { return s.altScreen.Load() }

// Title returns the window title set by the shell (OSC 0/2).
func (s *Session) Title() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.title
}

// Cwd returns the shell's working directory (Linux only, best effort).
func (s *Session) Cwd() string {
	dir, err := os.Readlink("/proc/" + strconv.Itoa(s.Pid()) + "/cwd")
	if err != nil {
		return ""
	}
	return dir
}

// ScrollBy moves the view n lines back into history (negative goes forward).
// The alternate screen has no history, so it always stays live.
func (s *Session) ScrollBy(n int) {
	s.mu.Lock()
	if s.altScreen.Load() {
		s.scroll = 0
	} else {
		s.scroll = max(0, min(s.scroll+n, s.emu.ScrollbackLen()))
	}
	s.mu.Unlock()
	s.notify()
}

// ScrollPage scrolls by whole pages (positive goes back in history).
func (s *Session) ScrollPage(pages int) {
	_, rows := s.Size()
	s.ScrollBy(pages * max(rows-1, 1))
}

// ResetScroll returns to the live view.
func (s *Session) ResetScroll() {
	s.mu.Lock()
	changed := s.scroll != 0
	s.scroll = 0
	s.mu.Unlock()
	if changed {
		s.notify()
	}
}

// ScrollOffset returns how many lines the view is scrolled back, and the
// size of the history.
func (s *Session) ScrollOffset() (offset, history int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.scroll, s.emu.ScrollbackLen()
}

// Close terminates the shell and releases the PTY.
func (s *Session) Close() error {
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Signal(os.Kill)
	}
	// Stop the reply-draining goroutine. Closing the pipe directly instead of
	// calling emu.Close avoids racing on the emulator's unsynchronized flag.
	if pw, ok := s.emu.InputPipe().(*io.PipeWriter); ok {
		_ = pw.Close()
	}
	return s.pty.Close()
}

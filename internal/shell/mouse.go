package shell

import (
	"fmt"
	"strings"
)

// WantsMouse reports whether the program in the shell enabled mouse
// tracking (vim with `set mouse=a`, htop, mc…).
func (s *Session) WantsMouse() bool { return s.mouseMode.Load() != 0 }

// ForwardMouse re-encodes a mouse event at pane cell (x, y) for the program
// in the shell, honoring the tracking mode it asked for.
func (s *Session) ForwardMouse(code, x, y int, release bool) {
	mode := s.mouseMode.Load()
	motion := code&32 != 0
	switch {
	case mode == 0:
		return
	case motion && mode != 1002 && mode != 1003:
		return // plain 1000/X10 tracking never reports drags
	case release && mode == 9:
		return // X10 reports presses only
	}

	if s.mouseSGR.Load() {
		end := 'M'
		if release {
			end = 'm'
		}
		_, _ = fmt.Fprintf(s.pty, "\x1b[<%d;%d;%d%c", code, x+1, y+1, end)
		return
	}
	// Legacy X10 encoding: release is button 3, coordinates capped at 223.
	if release {
		code = code&^3 | 3
	}
	clamp := func(v int) byte { return byte(min(v+1, 223) + 32) }
	_, _ = s.pty.Write([]byte{0x1b, '[', 'M', byte(code + 32), clamp(x), clamp(y)})
}

// WheelKeys sends n arrow presses, used for the wheel in full-screen apps
// that don't track the mouse (less, man): the "alternate scroll" behavior.
func (s *Session) WheelKeys(n int) {
	key := "A"
	if n > 0 {
		key = "B"
	} else {
		n = -n
	}
	prefix := "\x1b["
	if s.AppCursorMode() {
		prefix = "\x1bO"
	}
	_, _ = s.pty.Write([]byte(strings.Repeat(prefix+key, n)))
}

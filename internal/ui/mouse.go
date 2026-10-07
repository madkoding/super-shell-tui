package ui

import (
	"encoding/base64"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/madkoding/super-shell-tui/internal/input"
)

// wheelLines is how far one wheel notch scrolls.
const wheelLines = 3

// paneOrigin returns the screen cell of the shell pane's top-left cell.
func (m *Model) paneOrigin() (x, y int) {
	x = 1 // pane border
	if m.showSidebar {
		x += m.opts.SidebarWidth + 2
	}
	return x, 2 // header + pane border
}

// handleMouse routes a mouse report: to the program in the shell when it
// tracks the mouse, otherwise to scrollback and text selection.
func (m *Model) handleMouse(ev input.MouseEvent) tea.Cmd {
	ox, oy := m.paneOrigin()
	cols, rows := m.paneSize()
	x, y := ev.X-ox, ev.Y-oy
	inside := x >= 0 && y >= 0 && x < cols && y < rows

	// A left click on the header switches tabs.
	if ev.Y == 0 && ev.Button() == 0 && !ev.Motion() && !ev.Release && ev.Wheel() == 0 {
		if i := m.tabAt(ev.X); i >= 0 {
			m.switchTab(func() { m.ws.Select(i) })
		}
		return nil
	}

	if m.sess.WantsMouse() && !m.selecting {
		if inside {
			m.sess.ForwardMouse(ev.Code, x, y, ev.Release)
		}
		return nil
	}

	if w := ev.Wheel(); w != 0 {
		if !inside {
			return nil
		}
		if m.sess.AltScreen() {
			m.sess.WheelKeys(w * wheelLines) // less, man: scroll with arrows
		} else {
			m.sess.ScrollBy(-w * wheelLines)
		}
		return nil
	}

	if ev.Button() != 0 && !(ev.Release && m.selecting) {
		return nil // only the left button selects
	}
	switch {
	case ev.Release:
		m.selecting = false
		text := m.sess.SelectedText()
		if text == "" {
			return nil
		}
		if m.Clipboard != nil {
			m.Clipboard(text)
		}
		return m.setFlash(fmt.Sprintf("Copiado al portapapeles: %d caracteres", len([]rune(text))))
	case ev.Motion():
		if m.selecting {
			m.sess.SelectExtend(x, y)
		}
	case inside:
		m.selecting = true
		m.sess.SelectStart(x, y)
	default:
		m.sess.ClearSelection()
	}
	return nil
}

func (m *Model) setFlash(text string) tea.Cmd {
	m.flashID++
	m.flash = text
	id := m.flashID
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return clearFlashMsg{id: id} })
}

// OSC52Clipboard copies text through the terminal (OSC 52), which works
// locally and over SSH in most modern terminals.
func OSC52Clipboard(text string) {
	seq := "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte(text)) + "\a"
	if os.Getenv("TMUX") != "" {
		// tmux needs passthrough wrapping unless set-clipboard is on.
		_, _ = os.Stdout.WriteString(seq)
		seq = "\x1bPtmux;\x1b" + seq + "\x1b\\"
	}
	_, _ = os.Stdout.WriteString(seq)
}

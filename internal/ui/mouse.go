package ui

import (
	"encoding/base64"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/madkoding/super-shell-tui/internal/input"
	"github.com/madkoding/super-shell-tui/internal/workspace"
)

// wheelLines is how far one wheel notch scrolls.
const wheelLines = 3

// paneAt returns the pane under screen cell (sx, sy) and the cell's
// position inside it; inside is false on borders and outside every pane.
func (m *Model) paneAt(sx, sy int) (p workspace.Pane, x, y int, inside bool) {
	ox, oy := m.areaOrigin()
	for _, p := range m.ws.Panes() {
		x, y := sx-ox-p.X-1, sy-oy-p.Y-1 // 1 = pane border
		if x >= -1 && y >= -1 && x <= p.W-2 && y <= p.H-2 {
			cols, rows := p.Inner()
			return p, x, y, x >= 0 && y >= 0 && x < cols && y < rows
		}
	}
	return workspace.Pane{}, -1, -1, false
}

// handleMouse routes a mouse report: a click focuses the pane under it, then
// the report goes to the program in the shell when it tracks the mouse,
// otherwise to scrollback and text selection.
func (m *Model) handleMouse(ev input.MouseEvent) tea.Cmd {
	// A left click on the header switches tabs.
	if ev.Y == 0 && ev.Button() == 0 && !ev.Motion() && !ev.Release && ev.Wheel() == 0 {
		if i := m.tabAt(ev.X); i >= 0 {
			m.switchTab(func() { m.ws.Select(i) })
		}
		return nil
	}

	// The wheel over the sidebar scrolls it when it doesn't fit.
	if w := ev.Wheel(); w != 0 && m.showSidebar && ev.Y > 0 && ev.X < m.opts.SidebarWidth+2 && m.dragging == nil {
		m.sideScroll += w * wheelLines // fitSidebar clamps it
		return nil
	}

	// Dragging a border between panes resizes them.
	ox, oy := m.areaOrigin()
	if m.dragging != nil {
		switch {
		case ev.Release:
			m.dragging = nil
		case ev.Motion():
			m.ws.MoveDivider(*m.dragging, ev.X-ox, ev.Y-oy)
		}
		return nil
	}
	press := ev.Wheel() == 0 && !ev.Motion() && !ev.Release
	if press && ev.Button() == 0 && !m.selecting {
		if d, ok := m.ws.DividerAt(ev.X-ox, ev.Y-oy); ok {
			m.dragging = &d
			return nil
		}
	}

	p, x, y, inside := m.paneAt(ev.X, ev.Y)
	if press && p.Sess != nil && !p.Active && !m.selecting {
		m.changeFocus(func() { m.ws.Focus(p.Sess) })
	}
	if w := ev.Wheel(); w != 0 && p.Sess != nil && !p.Active {
		// The wheel scrolls the pane under the pointer, focused or not.
		if inside && !p.Sess.WantsMouse() && !p.Sess.AltScreen() {
			p.Sess.ScrollBy(-w * wheelLines)
		}
		return nil
	}
	if p.Sess != m.sess {
		// Drags leaving the focused pane keep selecting in it, clamped.
		for _, q := range m.ws.Panes() {
			if q.Active {
				x, y = ev.X-ox-q.X-1, ev.Y-oy-q.Y-1
			}
		}
		inside = false
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

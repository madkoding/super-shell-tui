package ui

// Status-bar prompts that capture keyboard input: tab rename and close
// confirmation.

import (
	"unicode"
	"unicode/utf8"
)

// TextMsg carries raw input captured while the user types a tab name.
type TextMsg []byte

// maxTabName limits tab names so the header stays readable.
const maxTabName = 20

func (m *Model) startRename() {
	m.renaming = true
	m.renameBuf = []rune(m.ws.Tabs()[m.ws.ActiveIndex()].Name)
	if m.SetCapture != nil {
		m.SetCapture(true)
	}
}

func (m *Model) endRename(save bool) {
	if save {
		m.ws.RenameActive(string(m.renameBuf))
	}
	m.renaming = false
	m.renameBuf = nil
	if m.SetCapture != nil {
		m.SetCapture(false)
	}
}

// handleRenameInput edits the name: Enter saves, Esc or Ctrl+C cancels,
// Backspace deletes, Ctrl+U clears. Other escape sequences (arrows, mouse
// reports) are ignored.
func (m *Model) handleRenameInput(b []byte) {
	if len(b) > 1 && b[0] == 0x1b {
		return // a whole escape sequence, not a lone Esc
	}
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		b = b[size:]
		switch {
		case r == '\r' || r == '\n':
			m.endRename(true)
			return
		case r == 0x1b || r == 0x03:
			m.endRename(false)
			return
		case r == 0x7f || r == 0x08:
			if n := len(m.renameBuf); n > 0 {
				m.renameBuf = m.renameBuf[:n-1]
			}
		case r == 0x15:
			m.renameBuf = m.renameBuf[:0]
		case unicode.IsPrint(r) && len(m.renameBuf) < maxTabName:
			m.renameBuf = append(m.renameBuf, r)
		}
	}
}

// startClose closes an idle shell at once and asks first when a program
// is running in it.
func (m *Model) startClose() {
	if m.sess != nil && !m.sess.Busy() {
		m.ws.CloseActive()
		return
	}
	m.confirmClose = true
	if m.SetCapture != nil {
		m.SetCapture(true)
	}
}

func (m *Model) endClose(close bool) {
	m.confirmClose = false
	if m.SetCapture != nil {
		m.SetCapture(false)
	}
	if close {
		m.ws.CloseActive()
	}
}

// handleCloseInput confirms with y/Y (or s/S, "sí"); anything else cancels.
func (m *Model) handleCloseInput(b []byte) {
	switch string(b) {
	case "y", "Y", "s", "S":
		m.endClose(true)
	default:
		m.endClose(false)
	}
}

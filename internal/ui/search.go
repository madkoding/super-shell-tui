package ui

// Scrollback search prompt (prefix + /).

import (
	"unicode"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/madkoding/super-shell-tui/internal/shell"
)

func (m *Model) startSearch() tea.Cmd {
	if m.sess.AltScreen() {
		return m.setFlash("La búsqueda no está disponible en apps de pantalla completa")
	}
	m.selecting = false
	m.sess.ClearSelection()
	m.searching = true
	m.searchBuf = m.searchBuf[:0]
	m.searchMiss = false
	if m.SetCapture != nil {
		m.SetCapture(true)
	}
	return nil
}

// endSearch closes the prompt. keep leaves the view at the match; otherwise
// it returns to the live screen.
func (m *Model) endSearch(keep bool) {
	m.searching = false
	m.sess.EndSearch(keep)
	if m.SetCapture != nil {
		m.SetCapture(false)
	}
}

func (m *Model) find(dir int) {
	if len(m.searchBuf) == 0 {
		m.searchMiss = false
		m.sess.EndSearch(true) // drop the highlight, stay where we are
		return
	}
	m.searchMiss = !m.sess.Find(string(m.searchBuf), dir)
}

// Arrow keys in both CSI and SS3 (application cursor) form.
var (
	keysOlder = map[string]bool{"\x1b[A": true, "\x1bOA": true, "\x12": true} // ↑, Ctrl+R
	keysNewer = map[string]bool{"\x1b[B": true, "\x1bOB": true, "\x13": true} // ↓, Ctrl+S
)

// handleSearchInput edits the query and searches as it is typed: ↑ or
// Ctrl+R goes to an older match, ↓ or Ctrl+S to a newer one, Enter stays at
// the match, Esc or Ctrl+C cancels back to the live screen.
func (m *Model) handleSearchInput(b []byte) {
	switch k := string(b); {
	case keysOlder[k]:
		m.find(shell.SearchOlder)
		return
	case keysNewer[k]:
		m.find(shell.SearchNewer)
		return
	case len(b) > 1 && b[0] == 0x1b:
		return // other escape sequences (mouse, function keys)
	}
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		b = b[size:]
		switch {
		case r == '\r' || r == '\n':
			m.endSearch(true)
			return
		case r == 0x1b || r == 0x03:
			m.endSearch(false)
			return
		case r == 0x7f || r == 0x08:
			if n := len(m.searchBuf); n > 0 {
				m.searchBuf = m.searchBuf[:n-1]
			}
		case r == 0x15:
			m.searchBuf = m.searchBuf[:0]
		case unicode.IsPrint(r):
			m.searchBuf = append(m.searchBuf, r)
		default:
			continue
		}
	}
	m.find(shell.SearchHere)
}

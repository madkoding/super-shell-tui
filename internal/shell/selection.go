package shell

import (
	"strings"

	uv "github.com/charmbracelet/ultraviolet"
)

// point is a cell in "absolute" coordinates: Line counts from the oldest
// history line, so a selection stays on the same text while scrolling.
type point struct{ X, Line int }

func (p point) before(o point) bool {
	return p.Line < o.Line || (p.Line == o.Line && p.X < o.X)
}

type selection struct {
	active       bool
	anchor, head point
}

// bounds returns the selection ordered from start to end.
func (sl selection) bounds() (point, point) {
	if sl.head.before(sl.anchor) {
		return sl.head, sl.anchor
	}
	return sl.anchor, sl.head
}

func (sl selection) contains(x, line int) bool {
	if !sl.active {
		return false
	}
	a, b := sl.bounds()
	p := point{x, line}
	return !p.before(a) && !b.before(p)
}

// absLine converts a pane row to an absolute line. Caller holds s.mu.
func (s *Session) absLine(y int) int {
	return s.emu.ScrollbackLen() - s.scroll + y
}

// cellAtAbs returns the cell at an absolute position. Caller holds s.mu.
func (s *Session) cellAtAbs(x, line int) *uv.Cell {
	history := s.emu.ScrollbackLen()
	if line < history {
		return s.emu.ScrollbackCellAt(x, line)
	}
	return s.emu.CellAt(x, line-history)
}

func (s *Session) clampPane(x, y int) (int, int) {
	return max(0, min(x, s.cols-1)), max(0, min(y, s.rows-1))
}

// SelectStart begins a selection at pane cell (x, y).
func (s *Session) SelectStart(x, y int) {
	s.mu.Lock()
	x, y = s.clampPane(x, y)
	p := point{x, s.absLine(y)}
	s.sel = selection{active: true, anchor: p, head: p}
	s.mu.Unlock()
	s.notify()
}

// SelectExtend moves the selection end to pane cell (x, y).
func (s *Session) SelectExtend(x, y int) {
	s.mu.Lock()
	if !s.sel.active {
		s.mu.Unlock()
		return
	}
	x, y = s.clampPane(x, y)
	s.sel.head = point{x, s.absLine(y)}
	s.mu.Unlock()
	s.notify()
}

// SelectedText returns the selected text, one line per row with trailing
// blanks trimmed. A single-cell selection (a plain click) yields "" and
// clears the selection.
func (s *Session) SelectedText() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.sel.active {
		return ""
	}
	a, b := s.sel.bounds()
	if a == b {
		s.sel = selection{}
		s.notify()
		return ""
	}
	var lines []string
	for line := a.Line; line <= b.Line; line++ {
		from, to := 0, s.cols-1
		if line == a.Line {
			from = a.X
		}
		if line == b.Line {
			to = b.X
		}
		var sb strings.Builder
		for x := from; x <= to; x++ {
			c := s.cellAtAbs(x, line)
			switch {
			case c == nil:
				sb.WriteByte(' ')
			case c.IsZero(): // second half of a wide char
			case c.Content == "":
				sb.WriteByte(' ')
			default:
				sb.WriteString(c.Content)
			}
		}
		lines = append(lines, strings.TrimRight(sb.String(), " "))
	}
	return strings.Join(lines, "\n")
}

// ClearSelection removes the highlight.
func (s *Session) ClearSelection() {
	s.mu.Lock()
	changed := s.sel.active
	s.sel = selection{}
	s.mu.Unlock()
	if changed {
		s.notify()
	}
}

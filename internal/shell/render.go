package shell

import (
	"strings"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// Render returns the visible part of the terminal (live screen or history,
// depending on the scroll offset) as lines with ANSI SGR sequences.
// When showCursor is true and the view is live, the cursor cell is drawn in
// reverse video, as is the mouse selection.
func (s *Session) Render(showCursor bool) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	cols, rows := s.cols, s.rows
	history := s.emu.ScrollbackLen()
	top := history - s.scroll // index of the first visible line in history+screen

	cur := s.emu.CursorPosition()
	showCursor = showCursor && s.scroll == 0 && s.cursorVisible.Load()

	var b strings.Builder
	b.Grow(cols * rows * 2)
	for y := 0; y < rows; y++ {
		if y > 0 {
			b.WriteByte('\n')
		}
		idx := top + y
		cell := func(x int) *uv.Cell { return s.cellAtAbs(x, idx) }
		cursorX := -1
		if showCursor && idx-history == cur.Y {
			cursorX = cur.X
		}
		selected := func(x int) bool { return s.sel.contains(x, idx) }
		renderLine(&b, cols, cell, cursorX, selected)
	}
	return b.String()
}

// renderLine writes one row of exactly cols cells. Missing cells (short
// history lines) are padded with blanks; the trailing half of a wide
// character is skipped because the wide cell already covers it.
func renderLine(b *strings.Builder, cols int, cell func(x int) *uv.Cell, cursorX int, selected func(x int) bool) {
	var pen uv.Style
	for x := 0; x < cols; {
		c := cell(x)
		var st uv.Style
		content, width := " ", 1
		if c != nil {
			if c.IsZero() { // continuation of a wide char that didn't fit
				x++
				continue
			}
			st = c.Style
			if c.Content != "" {
				content = c.Content
			}
			if c.Width > 1 && x+c.Width <= cols {
				width = c.Width
			} else if c.Width > 1 {
				content = " " // wide char cut by the pane edge
			}
		}
		if (x == cursorX) != selected(x) { // selection over the cursor cancels out
			st.Attrs ^= uv.AttrReverse
		}
		if !st.Equal(&pen) {
			b.WriteString(uv.StyleDiff(&pen, &st))
			pen = st
		}
		b.WriteString(content)
		x += width
	}
	b.WriteString(ansi.ResetStyle)
}

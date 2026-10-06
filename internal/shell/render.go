package shell

import (
	"strconv"
	"strings"

	"github.com/hinshun/vt10x"
)

// Glyph attribute bits, mirrored from vt10x (unexported there).
const (
	attrReverse = 1 << iota
	attrUnderline
	attrBold
	attrGfx
	attrItalic
	attrBlink
)

type style struct {
	fg, bg vt10x.Color
	mode   int16
}

// Render returns the emulated screen as lines with ANSI SGR sequences.
// When showCursor is true, the cursor cell is drawn in reverse video.
func (s *Session) Render(showCursor bool) string {
	s.term.Lock()
	defer s.term.Unlock()

	cols, rows := s.term.Size()
	cur := s.term.Cursor()
	showCursor = showCursor && s.term.CursorVisible()

	var b strings.Builder
	b.Grow(cols * rows * 2)

	for y := 0; y < rows; y++ {
		if y > 0 {
			b.WriteByte('\n')
		}
		prev := style{fg: vt10x.DefaultFG, bg: vt10x.DefaultBG}
		for x := 0; x < cols; x++ {
			g := s.term.Cell(x, y)
			st := style{fg: g.FG, bg: g.BG, mode: g.Mode}
			if showCursor && x == cur.X && y == cur.Y {
				st.mode ^= attrReverse
			}
			if st != prev {
				b.WriteString(sgr(st))
				prev = st
			}
			ch := g.Char
			if ch < ' ' {
				ch = ' '
			}
			b.WriteRune(ch)
		}
		b.WriteString("\x1b[0m")
	}
	return b.String()
}

// sgr builds a full SGR sequence (reset + attributes) for a cell style.
func sgr(st style) string {
	var p []string
	p = append(p, "0")
	if st.mode&attrBold != 0 {
		p = append(p, "1")
	}
	if st.mode&attrItalic != 0 {
		p = append(p, "3")
	}
	if st.mode&attrUnderline != 0 {
		p = append(p, "4")
	}
	if st.mode&attrBlink != 0 {
		p = append(p, "5")
	}
	if st.mode&attrReverse != 0 {
		p = append(p, "7")
	}
	if c := colorParam(st.fg, true); c != "" {
		p = append(p, c)
	}
	if c := colorParam(st.bg, false); c != "" {
		p = append(p, c)
	}
	return "\x1b[" + strings.Join(p, ";") + "m"
}

func colorParam(c vt10x.Color, fg bool) string {
	base := "38"
	if !fg {
		base = "48"
	}
	switch {
	case c == vt10x.DefaultFG || c == vt10x.DefaultBG || c == vt10x.DefaultCursor:
		return ""
	case c < 8:
		if fg {
			return strconv.Itoa(30 + int(c))
		}
		return strconv.Itoa(40 + int(c))
	case c < 16:
		if fg {
			return strconv.Itoa(90 + int(c) - 8)
		}
		return strconv.Itoa(100 + int(c) - 8)
	case c < 256:
		return base + ";5;" + strconv.Itoa(int(c))
	case c < 1<<24:
		r, g, bl := (c>>16)&0xff, (c>>8)&0xff, c&0xff
		return base + ";2;" + strconv.Itoa(int(r)) + ";" + strconv.Itoa(int(g)) + ";" + strconv.Itoa(int(bl))
	}
	return ""
}

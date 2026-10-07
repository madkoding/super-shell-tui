package shell

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
)

// newTestSession builds a Session without a process, fed by hand.
func newTestSession(t *testing.T, cols, rows int, output string) *Session {
	t.Helper()
	s := &Session{cols: cols, rows: rows, updates: make(chan struct{}, 1)}
	s.emu = vt.NewEmulator(cols, rows)
	s.emu.SetScrollbackSize(100)
	_, _ = s.emu.Write([]byte(output))
	return s
}

func plain(s string) []string { return strings.Split(ansi.Strip(s), "\n") }

func TestRenderWideCharsKeepWidth(t *testing.T) {
	s := newTestSession(t, 10, 2, "漢字ab🙂")
	for i, line := range plain(s.Render(false)) {
		if w := ansi.StringWidth(line); w != 10 {
			t.Errorf("line %d %q has width %d, want 10", i, line, w)
		}
	}
}

func TestScrollbackView(t *testing.T) {
	s := newTestSession(t, 5, 2, "1\r\n2\r\n3\r\n4")
	if got := plain(s.Render(false)); strings.TrimSpace(got[0]) != "3" {
		t.Fatalf("live view starts at %q", got[0])
	}
	s.ScrollBy(2)
	if got := plain(s.Render(false)); strings.TrimSpace(got[0]) != "1" {
		t.Fatalf("scrolled view starts at %q", got[0])
	}
	s.ScrollBy(100) // clamped to history
	if off, history := s.ScrollOffset(); off != history || history != 2 {
		t.Fatalf("offset %d history %d", off, history)
	}
}

func TestSelectedText(t *testing.T) {
	s := newTestSession(t, 10, 3, "hello\r\nworld 漢字")
	s.SelectStart(1, 0)
	s.SelectExtend(7, 1)
	if got := s.SelectedText(); got != "ello\nworld 漢" {
		t.Fatalf("got %q", got)
	}
	// Selection survives scrolling because it uses absolute lines.
	s.SelectStart(0, 0)
	s.SelectExtend(4, 0)
	s.ScrollBy(0)
	if got := s.SelectedText(); got != "hello" {
		t.Fatalf("got %q", got)
	}
	// A plain click selects nothing.
	s.SelectStart(2, 1)
	if got := s.SelectedText(); got != "" {
		t.Fatalf("click should not copy, got %q", got)
	}
}

package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestTopBorder(t *testing.T) {
	style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
	cases := []struct {
		w     int
		label string
		want  string
	}{
		{12, "src", "╭─ src ────╮"},
		{12, "a-very-long-name", "╭─ a-very… ╮"},
		{12, "", "╭──────────╮"},
		{6, "src", "╭────╮"}, // too narrow for a label
	}
	for _, c := range cases {
		got := ansi.Strip(topBorder(style, c.w, c.label))
		if got != c.want || ansi.StringWidth(got) != c.w {
			t.Errorf("topBorder(%d, %q) = %q, want %q", c.w, c.label, got, c.want)
		}
	}
}

func TestFitSidebar(t *testing.T) {
	m := &Model{opts: Options{SidebarWidth: 12}}
	text := "1\n2\n3\n4\n5\n6"
	if got := m.fitSidebar(text, 10); got != text {
		t.Errorf("text that fits changed: %q", got)
	}
	cases := []struct {
		scroll, wantScroll int
		want               []string
	}{
		{0, 0, []string{"1", "2", "3", "▼ rueda: ver más"}},
		{1, 1, []string{"▲ rueda: ver más", "3", "4", "▼ rueda: ver más"}},
		{99, 2, []string{"▲ rueda: ver más", "4", "5", "6"}}, // clamped to the end
		{-5, 0, []string{"1", "2", "3", "▼ rueda: ver más"}},
	}
	for _, c := range cases {
		m.sideScroll = c.scroll
		got := strings.Split(ansi.Strip(m.fitSidebar(text, 4)), "\n")
		for i := range got {
			got[i] = strings.TrimRight(got[i], " ")
		}
		if strings.Join(got, "|") != strings.Join(c.want, "|") || m.sideScroll != c.wantScroll {
			t.Errorf("scroll %d: got %q (scroll %d), want %q (scroll %d)", c.scroll, got, m.sideScroll, c.want, c.wantScroll)
		}
	}
}

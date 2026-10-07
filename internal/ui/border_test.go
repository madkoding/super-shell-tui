package ui

import (
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

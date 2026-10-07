package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestThemed(t *testing.T) {
	c := lipgloss.AdaptiveColor{Light: "#111111", Dark: "#EEEEEE"}
	if got := themed(c, "light"); got != lipgloss.Color("#111111") {
		t.Errorf("light = %v", got)
	}
	if got := themed(c, "dark"); got != lipgloss.Color("#EEEEEE") {
		t.Errorf("dark = %v", got)
	}
	if got := themed(c, "auto"); got != c {
		t.Errorf("auto = %v", got)
	}
}

package ui

import (
	"slices"
	"testing"

	"github.com/madkoding/super-shell-tui/internal/workspace"
)

func TestTabLayoutSpans(t *testing.T) {
	tabs := []workspace.Tab{{Title: "home", Active: true}, {Title: "漢字", Activity: true}, {Title: "tmp", Zoomed: true}}
	text, spans := tabLayout(tabs)
	if text != "Super Shell [1:home]  2:漢字•  3:tmp (zoom) " {
		t.Fatalf("text %q", text)
	}
	// Columns count the header padding; 漢字 take two cells each.
	want := [][2]int{{13, 21}, {22, 30}, {31, 45}}
	if !slices.Equal(spans, want) {
		t.Fatalf("spans %v, want %v", spans, want)
	}
}

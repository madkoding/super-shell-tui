package input

import (
	"slices"
	"testing"
)

func TestBindingsWith(t *testing.T) {
	b, err := DefaultBindings().With(map[string]string{"split_right": "v", "zoom": "Z"})
	if err != nil {
		t.Fatal(err)
	}
	if b.Action('v') != ActionSplitRight || b.Action('|') != 0 || b.Action('%') != 0 || b.Key(ActionSplitRight) != "v" {
		t.Fatal("split_right should move to v only")
	}
	if b.Action('Z') != ActionZoomPane || b.Action('z') != 0 || b.Action('c') != ActionNewTab {
		t.Fatal("zoom should move to Z, others stay")
	}
	// A key freed by a moved action can be taken by another.
	if _, err := DefaultBindings().With(map[string]string{"zoom": "x", "close": "w"}); err != nil {
		t.Fatalf("swapping keys around: %v", err)
	}
	for _, bad := range []map[string]string{
		{"split_right": "c"}, // taken by new_tab
		{"zoom": "zz"},       // not one character
		{"zoom": "3"},        // tabs
		{"zoom": "J"},        // resizing
		{"nope": "v"},        // unknown action
		{"zoom": " "},        // not printable
	} {
		if _, err := DefaultBindings().With(bad); err == nil {
			t.Errorf("%v: want an error", bad)
		}
	}
}

func TestFeedReboundKey(t *testing.T) {
	tr := NewTranslator(0, nil)
	tr.Keys, _ = DefaultBindings().With(map[string]string{"split_right": "v"})
	out, acts := tr.Feed([]byte{DefaultPrefix, 'v', DefaultPrefix, '|'})
	if len(out) != 0 || !slices.Equal(commands(acts), []Action{ActionSplitRight}) {
		t.Fatalf("out %q actions %v", out, commands(acts))
	}
}

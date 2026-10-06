package input

import (
	"bytes"
	"slices"
	"testing"
)

type fakeState struct{ appCursor, alt, bracketed bool }

func (f fakeState) AppCursorMode() bool  { return f.appCursor }
func (f fakeState) AltScreen() bool      { return f.alt }
func (f fakeState) BracketedPaste() bool { return f.bracketed }

// commands drops the bookkeeping actions (scroll reset, prefix state).
func commands(acts []Action) []Action {
	return slices.DeleteFunc(slices.Clone(acts), func(a Action) bool {
		return a == ActionScrollReset || a == ActionPrefixArmed || a == ActionPrefixDone
	})
}

func TestFeedPassesReadlineKeysRaw(t *testing.T) {
	tr := NewTranslator(0, nil)
	cases := map[string][]byte{
		"tab":       {'\t'},
		"up":        []byte("\x1b[A"),
		"down":      []byte("\x1b[B"),
		"ctrl+r":    {0x12},
		"ctrl+c":    {0x03},
		"alt+b":     []byte("\x1bb"),
		"esc alone": {0x1b},
		"pgup":      []byte("\x1b[5~"),
		"utf8":      []byte("ñandú"),
	}
	for name, in := range cases {
		out, acts := tr.Feed(append([]byte(nil), in...))
		if !bytes.Equal(out, in) || len(commands(acts)) != 0 {
			t.Errorf("%s: got %q %v, want %q", name, out, acts, in)
		}
		if !slices.Contains(acts, ActionScrollReset) {
			t.Errorf("%s: typing should return to the live view", name)
		}
	}
}

func TestFeedPrefixCommands(t *testing.T) {
	tr := NewTranslator(0, nil)
	out, acts := tr.Feed([]byte{'l', 's', DefaultPrefix, 'q'})
	if string(out) != "ls" || !slices.Equal(commands(acts), []Action{ActionQuit}) {
		t.Fatalf("got %q %v", out, acts)
	}
	// Prefix split across reads.
	if out, acts := tr.Feed([]byte{DefaultPrefix}); len(out) != 0 || !tr.Armed() ||
		!slices.Equal(acts, []Action{ActionPrefixArmed}) {
		t.Fatalf("prefix should arm, got %q %v", out, acts)
	}
	if _, acts := tr.Feed([]byte{'s'}); !slices.Equal(acts, []Action{ActionPrefixDone, ActionToggleSidebar}) {
		t.Fatalf("got %v", acts)
	}
	// Double prefix sends a literal one.
	if out, _ := tr.Feed([]byte{DefaultPrefix, DefaultPrefix}); !bytes.Equal(out, []byte{DefaultPrefix}) {
		t.Fatalf("got %q", out)
	}
}

func TestFeedAppCursorMode(t *testing.T) {
	tr := NewTranslator(0, fakeState{appCursor: true})
	out, _ := tr.Feed([]byte("\x1b[A\x1b[D\x1b[3~"))
	if string(out) != "\x1bOA\x1bOD\x1b[3~" {
		t.Fatalf("got %q", out)
	}
	// Pasted text is never rewritten.
	tr = NewTranslator(0, fakeState{appCursor: true, bracketed: true})
	if out, _ := tr.Feed([]byte("\x1b[200~\x1b[A\x1b[201~")); string(out) != "\x1b[200~\x1b[A\x1b[201~" {
		t.Fatalf("got %q", out)
	}
}

func TestFeedScrollKeys(t *testing.T) {
	tr := NewTranslator(0, fakeState{})
	out, acts := tr.Feed([]byte("\x1b[5;2~\x1b[6;2~"))
	if len(out) != 0 || !slices.Equal(acts, []Action{ActionScrollPageUp, ActionScrollPageDown}) {
		t.Fatalf("got %q %v", out, acts)
	}

	// Full-screen apps receive Shift+PgUp themselves.
	tr = NewTranslator(0, fakeState{alt: true})
	out, acts = tr.Feed([]byte("\x1b[5;2~"))
	if string(out) != "\x1b[5;2~" || len(commands(acts)) != 0 {
		t.Fatalf("got %q %v", out, acts)
	}
}

func TestFeedBracketedPaste(t *testing.T) {
	paste := "\x1b[200~echo a\necho \x1d q \x1b[5;2~\x1b[201~"

	// Shell supports bracketed paste: markers and content pass untouched,
	// and the prefix or scroll keys inside the paste are not interpreted.
	tr := NewTranslator(0, fakeState{bracketed: true})
	out, acts := tr.Feed([]byte(paste))
	if string(out) != paste || len(commands(acts)) != 0 || tr.InPaste() {
		t.Fatalf("got %q %v", out, acts)
	}

	// Shell without support: markers are stripped.
	tr = NewTranslator(0, fakeState{})
	out, _ = tr.Feed([]byte(paste))
	if string(out) != "echo a\necho \x1d q \x1b[5;2~" {
		t.Fatalf("got %q", out)
	}

	// Paste split across reads keeps its state.
	tr = NewTranslator(0, fakeState{})
	tr.Feed([]byte("\x1b[200~ab"))
	out, acts = tr.Feed([]byte("\x1dq\x1b[201~"))
	if string(out) != "\x1dq" || len(commands(acts)) != 0 {
		t.Fatalf("got %q %v", out, acts)
	}
}

func TestFeedMouse(t *testing.T) {
	tr := NewTranslator(0, fakeState{})
	var got []MouseEvent
	tr.OnMouse = func(e MouseEvent) { got = append(got, e) }

	out, acts := tr.Feed([]byte("a\x1b[<0;10;5M\x1b[<32;12;5M\x1b[<0;12;5m\x1b[<65;1;1Mb"))
	if string(out) != "ab" || len(commands(acts)) != 0 {
		t.Fatalf("got %q %v", out, acts)
	}
	want := []MouseEvent{
		{Code: 0, X: 9, Y: 4},
		{Code: 32, X: 11, Y: 4},
		{Code: 0, X: 11, Y: 4, Release: true},
		{Code: 65, X: 0, Y: 0},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v", got)
	}
	if !got[1].Motion() || got[3].Wheel() != 1 || got[0].Button() != 0 {
		t.Fatalf("bad helpers: %+v", got)
	}
}

func TestFeedTabCommands(t *testing.T) {
	tr := NewTranslator(0, nil)
	_, acts := tr.Feed([]byte{DefaultPrefix, 'c', DefaultPrefix, 'n', DefaultPrefix, 'p', DefaultPrefix, '3', DefaultPrefix, 'x', DefaultPrefix, '/'})
	got := commands(acts)
	want := []Action{ActionNewTab, ActionNextTab, ActionPrevTab, ActionSelectTab + 2, ActionCloseTab, ActionSearch}
	if !slices.Equal(got, want) || got[3].SelectedTab() != 2 || ActionQuit.SelectedTab() != -1 {
		t.Fatalf("got %v", got)
	}
}

func TestFeedCapture(t *testing.T) {
	tr := NewTranslator(0, nil)
	if _, acts := tr.Feed([]byte{DefaultPrefix, 'r'}); !slices.Equal(commands(acts), []Action{ActionRenameTab}) {
		t.Fatalf("got %v", acts)
	}
	var got []byte
	tr.OnCapture = func(b []byte) { got = append(got, b...) }
	tr.Capture.Store(true)
	out, acts := tr.Feed([]byte("api\x1d\r"))
	if len(out) != 0 || len(acts) != 0 || string(got) != "api\x1d\r" {
		t.Fatalf("got %q %v, captured %q", out, acts, got)
	}
	tr.Capture.Store(false)
	if out, _ := tr.Feed([]byte("ls")); string(out) != "ls" {
		t.Fatalf("got %q", out)
	}
}

package input

import (
	"bytes"
	"slices"
	"testing"
)

type fakeState struct{ appCursor, alt bool }

func (f fakeState) AppCursorMode() bool { return f.appCursor }
func (f fakeState) AltScreen() bool     { return f.alt }

// commands drops ActionScrollReset, which accompanies every forwarded key.
func commands(acts []Action) []Action {
	return slices.DeleteFunc(slices.Clone(acts), func(a Action) bool { return a == ActionScrollReset })
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
	if out, _ := tr.Feed([]byte{DefaultPrefix}); len(out) != 0 || !tr.Armed() {
		t.Fatalf("prefix should arm, got %q", out)
	}
	if _, acts := tr.Feed([]byte{'s'}); !slices.Equal(acts, []Action{ActionToggleSidebar}) {
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

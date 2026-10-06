package input

import (
	"bytes"
	"testing"
)

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
		"utf8":      []byte("ñandú"),
	}
	for name, in := range cases {
		out, acts := tr.Feed(append([]byte(nil), in...))
		if !bytes.Equal(out, in) || len(acts) != 0 {
			t.Errorf("%s: got %q %v, want %q", name, out, acts, in)
		}
	}
}

func TestFeedPrefixCommands(t *testing.T) {
	tr := NewTranslator(0, nil)
	out, acts := tr.Feed([]byte{'l', 's', DefaultPrefix, 'q'})
	if string(out) != "ls" || len(acts) != 1 || acts[0] != ActionQuit {
		t.Fatalf("got %q %v", out, acts)
	}
	// Prefix split across reads.
	if out, _ := tr.Feed([]byte{DefaultPrefix}); len(out) != 0 || !tr.Armed() {
		t.Fatalf("prefix should arm, got %q", out)
	}
	if _, acts := tr.Feed([]byte{'s'}); len(acts) != 1 || acts[0] != ActionToggleSidebar {
		t.Fatalf("got %v", acts)
	}
	// Double prefix sends a literal one.
	if out, _ := tr.Feed([]byte{DefaultPrefix, DefaultPrefix}); !bytes.Equal(out, []byte{DefaultPrefix}) {
		t.Fatalf("got %q", out)
	}
}

func TestFeedAppCursorMode(t *testing.T) {
	tr := NewTranslator(0, func() bool { return true })
	out, _ := tr.Feed([]byte("\x1b[A\x1b[D\x1b[3~"))
	if string(out) != "\x1bOA\x1bOD\x1b[3~" {
		t.Fatalf("got %q", out)
	}
}

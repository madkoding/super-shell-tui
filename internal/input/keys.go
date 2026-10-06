// Package input forwards raw keyboard bytes to the shell PTY untouched,
// except for a single prefix key (default Ctrl+]) reserved for the TUI.
//
// Bubble Tea's key parser is bypassed on purpose: decoding keys into events
// and re-encoding them loses information (Alt combos, Ctrl+R, function keys,
// escape timing), which breaks readline features like Tab completion,
// history navigation and reverse search.
package input

// Action is a TUI command triggered via the prefix key.
type Action int

const (
	ActionQuit Action = iota + 1
	ActionToggleSidebar
	ActionToggleHelp
)

// DefaultPrefix is Ctrl+] (0x1d), rarely used by shells or editors.
const DefaultPrefix byte = 0x1d

// Translator turns raw stdin chunks into bytes for the PTY plus TUI actions.
type Translator struct {
	Prefix byte
	// AppCursor reports whether the shell enabled application cursor mode
	// (DECCKM); arrows are then rewritten from CSI to SS3 form.
	AppCursor func() bool

	armed bool
}

// NewTranslator returns a Translator using prefix (0 means DefaultPrefix).
func NewTranslator(prefix byte, appCursor func() bool) *Translator {
	if prefix == 0 {
		prefix = DefaultPrefix
	}
	return &Translator{Prefix: prefix, AppCursor: appCursor}
}

// Feed processes one chunk read from the real terminal.
func (t *Translator) Feed(chunk []byte) (out []byte, actions []Action) {
	out = make([]byte, 0, len(chunk))
	for _, c := range chunk {
		if t.armed {
			t.armed = false
			switch c {
			case 'q', 'Q':
				actions = append(actions, ActionQuit)
			case 's', 'S':
				actions = append(actions, ActionToggleSidebar)
			case '?', 'h':
				actions = append(actions, ActionToggleHelp)
			case t.Prefix:
				out = append(out, t.Prefix) // double prefix sends it literally
			}
			continue
		}
		if c == t.Prefix {
			t.armed = true
			continue
		}
		out = append(out, c)
	}
	if t.AppCursor != nil && t.AppCursor() {
		out = toSS3(out)
	}
	return out, actions
}

// Armed reports whether the prefix key is waiting for a command.
func (t *Translator) Armed() bool { return t.armed }

// toSS3 rewrites ESC [ {A,B,C,D,H,F} to ESC O {A,B,C,D,H,F}.
func toSS3(b []byte) []byte {
	for i := 0; i+2 < len(b); i++ {
		if b[i] != 0x1b || b[i+1] != '[' {
			continue
		}
		switch b[i+2] {
		case 'A', 'B', 'C', 'D', 'H', 'F':
			b[i+1] = 'O'
			i += 2
		}
	}
	return b
}

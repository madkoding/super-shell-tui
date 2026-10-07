// Package input forwards raw keyboard bytes to the shell PTY untouched,
// except for a single prefix key (default Ctrl+]) reserved for the TUI.
//
// Bubble Tea's key parser is bypassed on purpose: decoding keys into events
// and re-encoding them loses information (Alt combos, Ctrl+R, function keys,
// escape timing), which breaks readline features like Tab completion,
// history navigation and reverse search.
package input

import "bytes"

// Action is a TUI command triggered via the prefix key or a reserved key.
type Action int

const (
	ActionQuit Action = iota + 1
	ActionToggleSidebar
	ActionToggleHelp
	ActionScrollPageUp   // Shift+PgUp
	ActionScrollPageDown // Shift+PgDn
	ActionScrollReset    // any key sent to the shell returns to the live view
	ActionPrefixArmed    // prefix pressed, waiting for a command key
	ActionPrefixDone     // the key after the prefix was consumed
)

// DefaultPrefix is Ctrl+] (0x1d), rarely used by shells or editors.
const DefaultPrefix byte = 0x1d

// Sequences the TUI keeps for itself (xterm encoding of Shift+PgUp/PgDn).
var (
	seqShiftPgUp = []byte("\x1b[5;2~")
	seqShiftPgDn = []byte("\x1b[6;2~")
)

// State is what the translator needs to know about the inner terminal.
type State interface {
	// AppCursorMode reports DECCKM; arrows are then rewritten to SS3 form.
	AppCursorMode() bool
	// AltScreen reports a full-screen app, which gets Shift+PgUp/PgDn itself.
	AltScreen() bool
}

// Translator turns raw stdin chunks into bytes for the PTY plus TUI actions.
type Translator struct {
	Prefix byte
	State  State // may be nil

	armed bool
}

// NewTranslator returns a Translator using prefix (0 means DefaultPrefix).
func NewTranslator(prefix byte, state State) *Translator {
	if prefix == 0 {
		prefix = DefaultPrefix
	}
	return &Translator{Prefix: prefix, State: state}
}

// Feed processes one chunk read from the real terminal.
func (t *Translator) Feed(chunk []byte) (out []byte, actions []Action) {
	out = make([]byte, 0, len(chunk))
	alt := t.State != nil && t.State.AltScreen()
	for i := 0; i < len(chunk); i++ {
		c := chunk[i]
		if t.armed {
			t.armed = false
			actions = append(actions, ActionPrefixDone)
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
			actions = append(actions, ActionPrefixArmed)
			continue
		}
		if c == 0x1b && !alt {
			rest := chunk[i:]
			switch {
			case bytes.HasPrefix(rest, seqShiftPgUp):
				actions = append(actions, ActionScrollPageUp)
				i += len(seqShiftPgUp) - 1
				continue
			case bytes.HasPrefix(rest, seqShiftPgDn):
				actions = append(actions, ActionScrollPageDown)
				i += len(seqShiftPgDn) - 1
				continue
			}
		}
		out = append(out, c)
	}
	if len(out) > 0 {
		actions = append([]Action{ActionScrollReset}, actions...)
		if t.State != nil && t.State.AppCursorMode() {
			out = toSS3(out)
		}
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

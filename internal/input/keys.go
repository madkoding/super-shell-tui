// Package input forwards raw keyboard bytes to the shell PTY untouched,
// except for a single prefix key (default Ctrl+]) reserved for the TUI.
//
// Bubble Tea's key parser is bypassed on purpose: decoding keys into events
// and re-encoding them loses information (Alt combos, Ctrl+R, function keys,
// escape timing), which breaks readline features like Tab completion,
// history navigation and reverse search.
package input

import (
	"bytes"
	"sync/atomic"
)

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
	ActionNewTab
	ActionNextTab
	ActionPrevTab
	ActionRenameTab
	ActionCloseTab
)

// ActionSelectTab is the first of nine actions selecting tabs 1..9
// (ActionSelectTab+0 is tab 1).
const ActionSelectTab Action = 100

// SelectedTab returns the 0-based tab an action selects, or -1.
func (a Action) SelectedTab() int {
	if a >= ActionSelectTab && a < ActionSelectTab+9 {
		return int(a - ActionSelectTab)
	}
	return -1
}

// DefaultPrefix is Ctrl+] (0x1d), rarely used by shells or editors.
const DefaultPrefix byte = 0x1d

// Sequences the TUI keeps for itself (xterm encoding of Shift+PgUp/PgDn),
// and the markers the outer terminal wraps pasted text with.
var (
	seqShiftPgUp  = []byte("\x1b[5;2~")
	seqShiftPgDn  = []byte("\x1b[6;2~")
	seqPasteStart = []byte("\x1b[200~")
	seqPasteEnd   = []byte("\x1b[201~")
)

// State is what the translator needs to know about the inner terminal.
type State interface {
	// AppCursorMode reports DECCKM; arrows are then rewritten to SS3 form.
	AppCursorMode() bool
	// AltScreen reports a full-screen app, which gets Shift+PgUp/PgDn itself.
	AltScreen() bool
	// BracketedPaste reports whether the shell understands paste markers;
	// when it doesn't, they are stripped so they never show up as garbage.
	BracketedPaste() bool
}

// Translator turns raw stdin chunks into bytes for the PTY plus TUI actions.
type Translator struct {
	Prefix byte
	State  State // may be nil
	// OnMouse receives mouse reports; they are never forwarded as keys.
	OnMouse func(MouseEvent)
	// Capture, while set, diverts every chunk to OnCapture instead of the
	// shell (used to type a tab name).
	Capture   atomic.Bool
	OnCapture func([]byte)

	armed   bool
	inPaste bool // between paste markers: content is never interpreted
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
	if t.Capture.Load() && t.OnCapture != nil {
		t.OnCapture(bytes.Clone(chunk))
		return nil, nil
	}
	out = make([]byte, 0, len(chunk))
	alt := t.State != nil && t.State.AltScreen()
	bracketed := t.State != nil && t.State.BracketedPaste()
	appCursor := t.State != nil && t.State.AppCursorMode()
	for i := 0; i < len(chunk); i++ {
		c := chunk[i]
		if c == 0x1b {
			rest := chunk[i:]
			marker := seqPasteStart
			if t.inPaste {
				marker = seqPasteEnd
			}
			if bytes.HasPrefix(rest, marker) {
				t.inPaste = !t.inPaste
				t.armed = false
				if bracketed {
					out = append(out, marker...)
				}
				i += len(marker) - 1
				continue
			}
		}
		if t.inPaste {
			out = append(out, c)
			continue
		}
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
			case 'c':
				actions = append(actions, ActionNewTab)
			case 'n':
				actions = append(actions, ActionNextTab)
			case 'p':
				actions = append(actions, ActionPrevTab)
			case 'r':
				actions = append(actions, ActionRenameTab)
			case 'x':
				actions = append(actions, ActionCloseTab)
			case '1', '2', '3', '4', '5', '6', '7', '8', '9':
				actions = append(actions, ActionSelectTab+Action(c-'1'))
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
		if ev, n, ok := parseSGRMouse(chunk[i:]); ok {
			if t.OnMouse != nil {
				t.OnMouse(ev)
			}
			i += n - 1
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
		if c == 0x1b && appCursor && isCSIArrow(chunk[i:]) {
			out = append(out, 0x1b, 'O', chunk[i+2])
			i += 2
			continue
		}
		out = append(out, c)
	}
	if len(out) > 0 {
		actions = append([]Action{ActionScrollReset}, actions...)
	}
	return out, actions
}

// InPaste reports whether a bracketed paste is in progress.
func (t *Translator) InPaste() bool { return t.inPaste }

// Armed reports whether the prefix key is waiting for a command.
func (t *Translator) Armed() bool { return t.armed }

// isCSIArrow matches ESC [ {A,B,C,D,H,F}, which DECCKM turns into SS3.
func isCSIArrow(b []byte) bool {
	if len(b) < 3 || b[0] != 0x1b || b[1] != '[' {
		return false
	}
	switch b[2] {
	case 'A', 'B', 'C', 'D', 'H', 'F':
		return true
	}
	return false
}

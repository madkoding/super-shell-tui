package input

import (
	"fmt"
	"sort"
)

// Bindings maps the key typed after the prefix to a TUI action. Digits
// (tabs 1-9), arrows and H/J/K/L (resizing) are fixed and not part of it.
type Bindings struct {
	keys  map[byte]Action
	label map[Action]byte // the key shown in help for each action
}

// ActionNames are the rebindable actions by their config-file name.
var ActionNames = map[string]Action{
	"quit":           ActionQuit,
	"toggle_sidebar": ActionToggleSidebar,
	"help":           ActionToggleHelp,
	"new_tab":        ActionNewTab,
	"next_tab":       ActionNextTab,
	"prev_tab":       ActionPrevTab,
	"rename_tab":     ActionRenameTab,
	"close":          ActionCloseTab,
	"search":         ActionSearch,
	"split_right":    ActionSplitRight,
	"split_down":     ActionSplitDown,
	"next_pane":      ActionNextPane,
	"zoom":           ActionZoomPane,
	"break_pane":     ActionBreakPane,
	"equalize":       ActionEqualizePanes,
	"swap_next":      ActionSwapPaneNext,
	"swap_prev":      ActionSwapPanePrev,
	"focus_left":     ActionFocusLeft,
	"focus_down":     ActionFocusDown,
	"focus_up":       ActionFocusUp,
	"focus_right":    ActionFocusRight,
}

// defaultKeys lists each action's keys, the one shown in help first.
var defaultKeys = []struct {
	a    Action
	keys string
}{
	{ActionQuit, "qQ"},
	{ActionToggleSidebar, "sS"},
	{ActionToggleHelp, "?"},
	{ActionNewTab, "c"},
	{ActionNextTab, "n"},
	{ActionPrevTab, "p"},
	{ActionRenameTab, "r"},
	{ActionCloseTab, "x"},
	{ActionSearch, "/"},
	{ActionSplitRight, "|%"},
	{ActionSplitDown, "-\""},
	{ActionNextPane, "o"},
	{ActionZoomPane, "z"},
	{ActionBreakPane, "!"},
	{ActionEqualizePanes, "="},
	{ActionSwapPaneNext, "}"},
	{ActionSwapPanePrev, "{"},
	{ActionFocusLeft, "h"},
	{ActionFocusDown, "j"},
	{ActionFocusUp, "k"},
	{ActionFocusRight, "l"},
}

// DefaultBindings returns the built-in keys.
func DefaultBindings() Bindings {
	b := Bindings{keys: map[byte]Action{}, label: map[Action]byte{}}
	for _, d := range defaultKeys {
		for i := 0; i < len(d.keys); i++ {
			b.keys[d.keys[i]] = d.a
		}
		b.label[d.a] = d.keys[0]
	}
	return b
}

// With returns b with each named action moved to the given key, which
// replaces all of the action's previous keys. Keys must be one printable
// ASCII character, not a digit or H/J/K/L, and not taken by another action.
func (b Bindings) With(overrides map[string]string) (Bindings, error) {
	out := Bindings{keys: map[byte]Action{}, label: map[Action]byte{}}
	moved := map[Action]byte{}
	names := make([]string, 0, len(overrides))
	for name := range overrides {
		names = append(names, name)
	}
	sort.Strings(names) // stable error messages
	for _, name := range names {
		a, ok := ActionNames[name]
		if !ok {
			return b, fmt.Errorf("keys: unknown action %q", name)
		}
		key := overrides[name]
		if len(key) != 1 || key[0] < '!' || key[0] > '~' {
			return b, fmt.Errorf("keys.%s must be one printable character, got %q", name, key)
		}
		if c := key[0]; c >= '0' && c <= '9' || c == 'H' || c == 'J' || c == 'K' || c == 'L' {
			return b, fmt.Errorf("keys.%s: %q is reserved (tabs 1-9, resizing H/J/K/L)", name, key)
		}
		moved[a] = key[0]
	}
	for k, a := range b.keys {
		if _, ok := moved[a]; !ok {
			out.keys[k] = a
		}
	}
	for a, l := range b.label {
		if _, ok := moved[a]; !ok {
			out.label[a] = l
		}
	}
	for _, name := range names {
		a, k := ActionNames[name], moved[ActionNames[name]]
		if other, taken := out.keys[k]; taken && other != a {
			return b, fmt.Errorf("keys.%s: %q is already used by %s", name, string(k), nameOf(other))
		}
		out.keys[k] = a
		out.label[a] = k
	}
	return out, nil
}

// Empty reports a zero Bindings, which binds nothing.
func (b Bindings) Empty() bool { return b.keys == nil }

// Action returns the action bound to key c, or 0.
func (b Bindings) Action(c byte) Action { return b.keys[c] }

// Key returns the key shown in help for a ("" when unbound).
func (b Bindings) Key(a Action) string {
	if k, ok := b.label[a]; ok {
		return string(k)
	}
	return ""
}

func nameOf(a Action) string {
	for n, x := range ActionNames {
		if x == a {
			return n
		}
	}
	return "another action"
}

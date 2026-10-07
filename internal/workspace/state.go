package workspace

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/madkoding/super-shell-tui/internal/shell"
)

// State is what is saved between runs: each tab's name and panes (layout,
// directory and running program of each), and the active tab. History and
// zoom are not restored.
type State struct {
	Tabs   []SavedTab `json:"tabs"`
	Active int        `json:"active"`
}

// SavedTab is one tab in State.
type SavedTab struct {
	Dir  string `json:"dir"`
	Name string `json:"name,omitempty"`
	Cmd  string `json:"cmd,omitempty"` // foreground program at exit, if any
	// Panes is the split layout; nil for a tab with a single pane. Dir and
	// Cmd then describe the focused pane, for files without Panes.
	Panes *SavedPane `json:"panes,omitempty"`
}

// SavedPane is a pane (Dir, Cmd, Focus) or a split of A and B.
type SavedPane struct {
	Name     string     `json:"name,omitempty"`
	Dir      string     `json:"dir,omitempty"`
	Cmd      string     `json:"cmd,omitempty"`
	Focus    bool       `json:"focus,omitempty"`
	Vertical bool       `json:"vertical,omitempty"`
	Ratio    float64    `json:"ratio,omitempty"`
	A        *SavedPane `json:"a,omitempty"`
	B        *SavedPane `json:"b,omitempty"`
}

// maxSavedPanes bounds the panes restored per tab, against a damaged file.
const maxSavedPanes = 16

func (p *SavedPane) isSplit() bool { return p.A != nil && p.B != nil }

func (p *SavedPane) count() int {
	if !p.isSplit() {
		return 1
	}
	return p.A.count() + p.B.count()
}

// save captures n and the panes below it. Caller holds w.mu.
func save(n *node, focus *shell.Session) *SavedPane {
	if n.sess != nil {
		return &SavedPane{Name: n.name, Dir: n.sess.Cwd(), Cmd: n.sess.ForegroundCommand(), Focus: n.sess == focus}
	}
	return &SavedPane{Vertical: n.vertical, Ratio: n.ratio, A: save(n.a, focus), B: save(n.b, focus)}
}

// Snapshot captures the open tabs.
func (w *Workspace) Snapshot() State {
	w.mu.Lock()
	defer w.mu.Unlock()
	st := State{Active: w.active}
	for _, t := range w.tabs {
		saved := SavedTab{
			Dir:  t.focus.Cwd(),
			Name: t.name,
			Cmd:  t.focus.ForegroundCommand(),
		}
		if t.root.sess == nil {
			saved.Panes = save(t.root, t.focus)
		}
		st.Tabs = append(st.Tabs, saved)
	}
	return st
}

// StatePath is $XDG_STATE_HOME/super-shell/tabs.json, falling back to
// ~/.local/state/super-shell/tabs.json.
func StatePath() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "super-shell", "tabs.json")
}

// LoadState reads path; a missing or unreadable file yields an empty State
// so a bad file never blocks startup.
func LoadState(path string) State {
	var st State
	data, err := os.ReadFile(path)
	if err != nil {
		return State{}
	}
	if json.Unmarshal(data, &st) != nil {
		return State{}
	}
	return st
}

// SaveState writes st to path, or removes the file when there are no tabs.
func SaveState(path string, st State) error {
	if path == "" {
		return nil
	}
	if len(st.Tabs) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

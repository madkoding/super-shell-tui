package workspace

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// State is what is saved between runs: each tab's directory, name and the
// program running in it (from its focused pane), and the active tab. Splits
// and history are not restored.
type State struct {
	Tabs   []SavedTab `json:"tabs"`
	Active int        `json:"active"`
}

// SavedTab is one tab in State.
type SavedTab struct {
	Dir  string `json:"dir"`
	Name string `json:"name,omitempty"`
	Cmd  string `json:"cmd,omitempty"` // foreground program at exit, if any
}

// Snapshot captures the open tabs.
func (w *Workspace) Snapshot() State {
	w.mu.Lock()
	defer w.mu.Unlock()
	st := State{Active: w.active}
	for _, t := range w.tabs {
		st.Tabs = append(st.Tabs, SavedTab{
			Dir:  t.focus.Cwd(),
			Name: t.name,
			Cmd:  t.focus.ForegroundCommand(),
		})
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

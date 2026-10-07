// Package workspace manages several shell sessions shown as tabs. Only the
// active tab receives keystrokes and is rendered; the others keep running.
package workspace

import (
	"errors"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/madkoding/super-shell-tui/internal/shell"
)

// MaxTabs caps open tabs (they are selected with prefix + 1..9).
const MaxTabs = 9

// resizeQuiet is how long after a resize output is treated as a redraw.
const resizeQuiet = 500 * time.Millisecond

// Tab describes a tab for the tab bar.
type Tab struct {
	Title    string // Name, or the working directory when unnamed
	Name     string // user-given name, empty when automatic
	Active   bool
	Activity bool // output arrived while in the background
}

type tab struct {
	sess     *shell.Session
	name     string // set by the user; empty means automatic
	activity bool
}

// Workspace is safe for concurrent use: the input pump writes to it while
// the UI renders.
type Workspace struct {
	shellPath  string
	scrollback int

	mu         sync.Mutex
	tabs       []*tab
	active     int
	cols, rows int

	changed chan struct{}
}

// New starts a workspace with one shell of cols x rows.
func New(shellPath string, scrollback, cols, rows int) (*Workspace, error) {
	w := &Workspace{
		shellPath:  shellPath,
		scrollback: scrollback,
		cols:       cols,
		rows:       rows,
		changed:    make(chan struct{}, 1),
	}
	if err := w.NewTab(); err != nil {
		return nil, err
	}
	return w, nil
}

// Restore starts one tab per saved entry (directory and name) and activates
// the saved tab. With no usable entries it behaves like New.
func Restore(st State, shellPath string, scrollback, cols, rows int) (*Workspace, error) {
	if len(st.Tabs) == 0 {
		return New(shellPath, scrollback, cols, rows)
	}
	w := &Workspace{
		shellPath:  shellPath,
		scrollback: scrollback,
		cols:       cols,
		rows:       rows,
		changed:    make(chan struct{}, 1),
	}
	for _, t := range st.Tabs[:min(len(st.Tabs), MaxTabs)] {
		if err := w.NewTabAt(t.Dir, t.Name); err != nil {
			w.Close()
			return nil, err
		}
	}
	w.Select(st.Active)
	return w, nil
}

// ErrTooManyTabs is returned by NewTab when MaxTabs are open.
var ErrTooManyTabs = errors.New("too many tabs")

// NewTab starts a shell in a new tab and activates it. The shell starts in
// the active tab's directory, like most terminals do.
func (w *Workspace) NewTab() error {
	dir := ""
	if s := w.Active(); s != nil {
		dir = s.Cwd()
	}
	return w.NewTabAt(dir, "")
}

// NewTabAt starts a shell in dir with the given tab name ("" = automatic).
func (w *Workspace) NewTabAt(dir, name string) error {
	w.mu.Lock()
	if len(w.tabs) >= MaxTabs {
		w.mu.Unlock()
		return ErrTooManyTabs
	}
	cols, rows := w.cols, w.rows
	w.mu.Unlock()

	sess, err := shell.Start(w.shellPath, dir, cols, rows, w.scrollback)
	if err != nil {
		return err
	}
	t := &tab{sess: sess, name: name}
	w.mu.Lock()
	w.tabs = append(w.tabs, t)
	w.active = len(w.tabs) - 1
	w.mu.Unlock()

	go w.watch(t)
	w.notify()
	return nil
}

// watch forwards a tab's screen updates and removes it when its shell exits.
func (w *Workspace) watch(t *tab) {
	seen := t.sess.Outputs()
	for {
		select {
		case <-t.sess.Updates():
			out := t.sess.Outputs()
			w.mu.Lock()
			redraw := t.sess.SinceResize() < resizeQuiet
			if out != seen && !redraw && w.indexOf(t) != w.active {
				t.activity = true
			}
			seen = out
			w.mu.Unlock()
			w.notify()
		case <-t.sess.Done():
			w.mu.Lock()
			if i := w.indexOf(t); i >= 0 {
				w.tabs = append(w.tabs[:i], w.tabs[i+1:]...)
				if w.active > i || w.active >= len(w.tabs) {
					w.active = max(w.active-1, 0)
				}
			}
			w.mu.Unlock()
			_ = t.sess.Close()
			w.notify()
			return
		}
	}
}

func (w *Workspace) indexOf(t *tab) int {
	for i, x := range w.tabs {
		if x == t {
			return i
		}
	}
	return -1
}

func (w *Workspace) notify() {
	select {
	case w.changed <- struct{}{}:
	default:
	}
}

// Changed fires when the active screen or the tab list changed.
func (w *Workspace) Changed() <-chan struct{} { return w.changed }

// Len returns the number of open tabs; 0 means every shell exited.
func (w *Workspace) Len() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.tabs)
}

// Active returns the active session, or nil when no tab is left.
func (w *Workspace) Active() *shell.Session {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.tabs) == 0 {
		return nil
	}
	return w.tabs[w.active].sess
}

// ActiveIndex returns the 0-based index of the active tab.
func (w *Workspace) ActiveIndex() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.active
}

// Select activates tab i (0-based); out of range is ignored.
func (w *Workspace) Select(i int) {
	w.mu.Lock()
	if i < 0 || i >= len(w.tabs) || i == w.active {
		w.mu.Unlock()
		return
	}
	w.active = i
	w.tabs[i].activity = false
	w.mu.Unlock()
	w.notify()
}

// RenameActive sets the active tab's name; "" restores the automatic title.
func (w *Workspace) RenameActive(name string) {
	w.mu.Lock()
	if len(w.tabs) > 0 {
		w.tabs[w.active].name = name
	}
	w.mu.Unlock()
	w.notify()
}

// CloseActive terminates the active tab's shell; its tab disappears once
// the shell exits, like a regular exit.
func (w *Workspace) CloseActive() {
	if s := w.Active(); s != nil {
		_ = s.Close()
	}
}

// Next and Prev cycle through tabs.
func (w *Workspace) Next() { w.cycle(1) }
func (w *Workspace) Prev() { w.cycle(-1) }

func (w *Workspace) cycle(d int) {
	w.mu.Lock()
	n := len(w.tabs)
	i := w.active
	w.mu.Unlock()
	if n > 1 {
		w.Select((i + d + n) % n)
	}
}

// Tabs lists the tabs for display.
func (w *Workspace) Tabs() []Tab {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]Tab, len(w.tabs))
	for i, t := range w.tabs {
		name := t.name
		if name == "" {
			name = title(t.sess, i)
		}
		out[i] = Tab{Title: name, Name: t.name, Active: i == w.active, Activity: t.activity}
	}
	return out
}

func title(s *shell.Session, i int) string {
	if dir := s.Cwd(); dir != "" {
		return filepath.Base(dir)
	}
	return "shell " + strconv.Itoa(i+1)
}

// Resize applies a new pane size to every tab.
func (w *Workspace) Resize(cols, rows int) {
	w.mu.Lock()
	w.cols, w.rows = cols, rows
	tabs := append([]*tab(nil), w.tabs...)
	w.mu.Unlock()
	for _, t := range tabs {
		_ = t.sess.Resize(cols, rows)
	}
}

// Write sends keystrokes to the active shell.
func (w *Workspace) Write(p []byte) (int, error) {
	if s := w.Active(); s != nil {
		return s.Write(p)
	}
	return len(p), nil
}

// AppCursorMode, AltScreen and BracketedPaste report the active shell's
// modes, so the workspace can drive input.Translator directly.
func (w *Workspace) AppCursorMode() bool { return w.query((*shell.Session).AppCursorMode) }
func (w *Workspace) AltScreen() bool     { return w.query((*shell.Session).AltScreen) }
func (w *Workspace) BracketedPaste() bool {
	return w.query((*shell.Session).BracketedPaste)
}

func (w *Workspace) query(f func(*shell.Session) bool) bool {
	if s := w.Active(); s != nil {
		return f(s)
	}
	return false
}

// Close terminates every shell.
func (w *Workspace) Close() {
	w.mu.Lock()
	tabs := append([]*tab(nil), w.tabs...)
	w.mu.Unlock()
	for _, t := range tabs {
		_ = t.sess.Close()
	}
}

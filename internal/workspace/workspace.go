// Package workspace manages shell sessions shown as tabs, each split into one
// or more panes. Only the focused pane of the active tab receives keystrokes;
// every other shell keeps running.
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
	Zoomed   bool // showing one pane of several
}

type tab struct {
	root     *node
	focus    *shell.Session // the pane receiving keystrokes
	name     string         // set by the user; empty means automatic
	activity bool
	zoomed   bool // only the focused pane is shown
}

// Workspace is safe for concurrent use: the input pump writes to it while
// the UI renders.
type Workspace struct {
	shellPath  string
	scrollback int

	mu            sync.Mutex
	tabs          []*tab
	active        int
	width, height int // pane area, pane borders included

	changed chan struct{}
}

// New starts a workspace with one shell. width x height is the pane area,
// borders included.
func New(shellPath string, scrollback, width, height int) (*Workspace, error) {
	w := &Workspace{
		shellPath:  shellPath,
		scrollback: scrollback,
		width:      width,
		height:     height,
		changed:    make(chan struct{}, 1),
	}
	if err := w.NewTab(); err != nil {
		return nil, err
	}
	return w, nil
}

// Replay says what Restore does with the program a tab was running.
type Replay int

const (
	ReplayOff  Replay = iota // ignore it
	ReplayType               // type it at the prompt; Enter runs it
	ReplayRun                // type it and run it
)

// Restore starts one tab per saved entry (directory and name), replays the
// program each one was running per replay, and activates the saved tab.
// With no usable entries it behaves like New.
func Restore(st State, replay Replay, shellPath string, scrollback, width, height int) (*Workspace, error) {
	if len(st.Tabs) == 0 {
		return New(shellPath, scrollback, width, height)
	}
	w := &Workspace{
		shellPath:  shellPath,
		scrollback: scrollback,
		width:      width,
		height:     height,
		changed:    make(chan struct{}, 1),
	}
	for _, t := range st.Tabs[:min(len(st.Tabs), MaxTabs)] {
		if err := w.NewTabAt(t.Dir, t.Name); err != nil {
			w.Close()
			return nil, err
		}
		if t.Cmd != "" && replay != ReplayOff {
			line := t.Cmd
			if replay == ReplayRun {
				line += "\r"
			}
			go typeWhenReady(w.Active(), line)
		}
	}
	w.Select(st.Active)
	return w, nil
}

// typeWhenReady writes line to the shell once its startup output (rc file
// messages, the first prompt) has gone quiet, so the line lands at the prompt
// instead of being echoed before it.
func typeWhenReady(s *shell.Session, line string) {
	const quiet, poll, limit = 150 * time.Millisecond, 20 * time.Millisecond, 3 * time.Second
	var last uint64
	stable := time.Duration(0)
	for waited := time.Duration(0); waited < limit; waited += poll {
		time.Sleep(poll)
		if n := s.Outputs(); n == 0 || n != last {
			last, stable = n, 0
			continue
		}
		if stable += poll; stable >= quiet {
			break
		}
	}
	_, _ = s.Write([]byte(line))
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
	cols, rows := Pane{W: w.width, H: w.height}.Inner()
	w.mu.Unlock()

	sess, err := shell.Start(w.shellPath, dir, cols, rows, w.scrollback)
	if err != nil {
		return err
	}
	t := &tab{root: &node{sess: sess}, focus: sess, name: name}
	w.mu.Lock()
	w.tabs = append(w.tabs, t)
	w.active = len(w.tabs) - 1
	w.mu.Unlock()

	go w.watch(t, sess)
	w.notify()
	return nil
}

// watch forwards a pane's screen updates and removes the pane when its
// shell exits; the tab goes away with its last pane.
func (w *Workspace) watch(t *tab, sess *shell.Session) {
	seen := sess.Outputs()
	for {
		select {
		case <-sess.Updates():
			out := sess.Outputs()
			w.mu.Lock()
			redraw := sess.SinceResize() < resizeQuiet
			if out != seen && !redraw && w.indexOf(t) != w.active {
				t.activity = true
			}
			seen = out
			w.mu.Unlock()
			w.notify()
		case <-sess.Done():
			w.mu.Lock()
			if leaf := t.root.find(sess); leaf != nil {
				t.root = remove(t.root, leaf)
			}
			remaining := t.root != nil
			unzoom(t)
			if remaining && t.focus == sess {
				t.focus = t.root.first().sess
			}
			if i := w.indexOf(t); i >= 0 && !remaining {
				w.tabs = append(w.tabs[:i], w.tabs[i+1:]...)
				if w.active > i || w.active >= len(w.tabs) {
					w.active = max(w.active-1, 0)
				}
			}
			w.mu.Unlock()
			_ = sess.Close()
			if remaining {
				w.relayout(t)
			}
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
	return w.tabs[w.active].focus
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

// CloseActive terminates the focused pane's shell; the pane (and the tab,
// with its last pane) disappears once the shell exits, like a regular exit.
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
			name = title(t.focus, i)
		}
		out[i] = Tab{Title: name, Name: t.name, Active: i == w.active, Activity: t.activity, Zoomed: t.zoomed}
	}
	return out
}

func title(s *shell.Session, i int) string {
	if dir := s.Cwd(); dir != "" {
		return filepath.Base(dir)
	}
	return "shell " + strconv.Itoa(i+1)
}

// Resize sets the pane area (borders included) and lays out every tab.
func (w *Workspace) Resize(width, height int) {
	w.mu.Lock()
	w.width, w.height = width, height
	tabs := append([]*tab(nil), w.tabs...)
	w.mu.Unlock()
	for _, t := range tabs {
		w.relayout(t)
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

// Close terminates every shell, hidden panes included.
func (w *Workspace) Close() {
	w.mu.Lock()
	var all []*shell.Session
	for _, t := range w.tabs {
		all = t.root.leaves(all)
	}
	w.mu.Unlock()
	for _, s := range all {
		_ = s.Close()
	}
}

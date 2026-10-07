package workspace

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("timed out")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestTabsLifecycle(t *testing.T) {
	w, err := New("/bin/sh", 100, 40, 10)
	if err != nil {
		t.Skip("no /bin/sh:", err)
	}
	defer w.Close()

	if err := w.NewTab(); err != nil {
		t.Fatal(err)
	}
	if w.Len() != 2 || w.ActiveIndex() != 1 {
		t.Fatalf("len %d active %d", w.Len(), w.ActiveIndex())
	}
	w.Next()
	if w.ActiveIndex() != 0 {
		t.Fatalf("Next should wrap to 0, got %d", w.ActiveIndex())
	}
	w.Prev()
	w.Select(7) // out of range: ignored
	if w.ActiveIndex() != 1 {
		t.Fatalf("got %d", w.ActiveIndex())
	}

	// Exiting the active shell closes its tab and activates the previous one.
	first := w.tabs[0].focus
	_, _ = w.Write([]byte("exit\n"))
	waitFor(t, func() bool { return w.Len() == 1 })
	if w.Active() != first {
		t.Fatal("first tab should be active")
	}
	_, _ = w.Write([]byte("exit\n"))
	waitFor(t, func() bool { return w.Len() == 0 })
	if w.Active() != nil {
		t.Fatal("no tab should be left")
	}
}

func TestStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state", "tabs.json")

	if st := LoadState(path); len(st.Tabs) != 0 {
		t.Fatalf("missing file should be empty, got %+v", st)
	}
	want := State{Tabs: []SavedTab{{Dir: dir, Name: "api", Cmd: "sleep 30"}, {Dir: "/"}}, Active: 1}
	if err := SaveState(path, want); err != nil {
		t.Fatal(err)
	}
	if got := LoadState(path); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}

	w, err := Restore(want, ReplayRun, "/bin/sh", 100, 40, 10)
	if err != nil {
		t.Skip("no /bin/sh:", err)
	}
	defer w.Close()
	if w.Len() != 2 || w.ActiveIndex() != 1 || w.Tabs()[0].Title != "api" {
		t.Fatalf("restored %+v active %d", w.Tabs(), w.ActiveIndex())
	}
	// The saved command is run again and shows up in the next snapshot.
	waitFor(t, func() bool {
		st := w.Snapshot()
		return st.Tabs[0].Dir == dir && st.Tabs[0].Cmd == "sleep 30" && st.Tabs[1].Cmd == ""
	})

	// No tabs left: the file is removed.
	if err := SaveState(path, State{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("state file should be gone, err=%v", err)
	}
	// A corrupt file never blocks startup.
	_ = os.WriteFile(path, []byte("{nope"), 0o600)
	if st := LoadState(path); len(st.Tabs) != 0 {
		t.Fatalf("corrupt file should be empty, got %+v", st)
	}
}

func TestSplitPanes(t *testing.T) {
	w, err := New("/bin/sh", 100, 40, 10)
	if err != nil {
		t.Skip("no /bin/sh:", err)
	}
	defer w.Close()
	first := w.Active()

	if err := w.Split(true); err != nil {
		t.Fatal(err)
	}
	if err := w.Split(false); err != nil {
		t.Fatal(err)
	}
	if err := w.Split(false); err != ErrNoRoom {
		t.Fatalf("a 5-row pane should not split, got %v", err)
	}
	ps := w.Panes()
	want := []Pane{{X: 0, Y: 0, W: 20, H: 10}, {X: 20, Y: 0, W: 20, H: 5}, {X: 20, Y: 5, W: 20, H: 5, Active: true}}
	if len(ps) != len(want) {
		t.Fatalf("got %d panes", len(ps))
	}
	for i := range ps {
		got := ps[i]
		got.Sess = nil
		if got != want[i] {
			t.Errorf("pane %d = %+v, want %+v", i, got, want[i])
		}
	}
	if c, r := ps[2].Sess.Size(); c != 18 || r != 3 {
		t.Errorf("shell size %dx%d, want 18x3", c, r)
	}

	// Zoom shows the focused pane alone, at full size, until focus moves.
	w.ToggleZoom()
	if ps := w.Panes(); len(ps) != 1 || ps[0].W != 40 || !w.Tabs()[0].Zoomed || w.PaneCount() != 3 {
		t.Fatalf("zoomed panes %+v", ps)
	}
	waitFor(t, func() bool { c, r := w.Active().Size(); return c == 38 && r == 8 })

	w.FocusNext()
	if w.Tabs()[0].Zoomed || len(w.Panes()) != 3 {
		t.Fatal("moving the focus should end the zoom")
	}
	if w.Active() != first {
		t.Fatal("FocusNext should wrap to the first pane")
	}
	// Exiting a pane gives its room to the sibling; the tab stays.
	_, _ = w.Write([]byte("exit\n"))
	waitFor(t, func() bool { return w.PaneCount() == 2 })
	if w.Len() != 1 || w.Active() == first {
		t.Fatalf("len %d, focus should move off the exited pane", w.Len())
	}
	waitFor(t, func() bool { c, _ := w.Active().Size(); return c == 38 })
}

func TestFocusPaneDirection(t *testing.T) {
	w, err := New("/bin/sh", 100, 40, 10)
	if err != nil {
		t.Skip("no /bin/sh:", err)
	}
	defer w.Close()
	left := w.Active()
	if err := w.Split(true); err != nil {
		t.Fatal(err)
	}
	top := w.Active()
	if err := w.Split(false); err != nil {
		t.Fatal(err)
	}
	bottom := w.Active()

	steps := []struct {
		dir  Direction
		want any
	}{
		{Up, top}, {Up, top}, {Left, left}, {Left, left}, {Right, top}, {Down, bottom}, {Right, bottom},
	}
	for i, s := range steps {
		w.FocusPane(s.dir)
		if w.Active() != s.want {
			t.Fatalf("step %d: focus did not move as expected", i)
		}
	}
	w.ToggleZoom()
	w.FocusPane(Left)
	if w.Tabs()[0].Zoomed || w.Active() != left {
		t.Fatal("FocusPane should end the zoom and move")
	}
}

func TestEqualizePanes(t *testing.T) {
	w, err := New("/bin/sh", 100, 40, 10)
	if err != nil {
		t.Skip("no /bin/sh:", err)
	}
	defer w.Close()
	w.Resize(160, 40) // room for three columns
	// Splitting the right half again leaves panes of 1/2, 1/4 and 1/4.
	if err := w.Split(true); err != nil {
		t.Fatal(err)
	}
	if err := w.Split(true); err != nil {
		t.Fatal(err)
	}
	widths := func() []int {
		var out []int
		for _, p := range w.Panes() {
			out = append(out, p.W)
		}
		return out
	}
	before := widths()
	w.EqualizePanes()
	got := widths()
	if len(got) != 3 || before[0] == got[0] {
		t.Fatalf("widths %v -> %v", before, got)
	}
	for _, x := range got {
		if x < got[0]-1 || x > got[0]+1 {
			t.Fatalf("uneven widths %v", got)
		}
	}
}

func TestSwapPane(t *testing.T) {
	w, err := New("/bin/sh", 100, 40, 10)
	if err != nil {
		t.Skip("no /bin/sh:", err)
	}
	defer w.Close()
	left := w.Active()
	if err := w.Split(true); err != nil {
		t.Fatal(err)
	}
	right := w.Active()
	w.ToggleZoom()

	w.SwapPane(false) // wraps: the right pane moves to the left
	ps := w.Panes()
	if len(ps) != 2 || ps[0].Sess != right || ps[1].Sess != left || !ps[0].Active {
		t.Fatalf("after swap: %+v", ps)
	}
	if w.Tabs()[0].Zoomed {
		t.Fatal("swapping should end the zoom")
	}
	w.SwapPane(true)
	if ps := w.Panes(); ps[0].Sess != left || ps[1].Sess != right || !ps[1].Active {
		t.Fatalf("after swap back: %+v", ps)
	}
	waitFor(t, func() bool { c, _ := left.Size(); return c == 18 })
}

func TestResizePanes(t *testing.T) {
	w, err := New("/bin/sh", 100, 40, 10)
	if err != nil {
		t.Skip("no /bin/sh:", err)
	}
	defer w.Close()
	if err := w.Split(true); err != nil {
		t.Fatal(err)
	}
	widths := func() [2]int { ps := w.Panes(); return [2]int{ps[0].W, ps[1].W} }

	w.ResizePane(Right) // one step is a tenth of the split
	if got := widths(); got != [2]int{24, 16} {
		t.Fatalf("after Right: %v", got)
	}
	w.ResizePane(Up) // no horizontal divider: nothing moves
	if got := widths(); got != [2]int{24, 16} {
		t.Fatalf("after Up: %v", got)
	}
	if _, ok := w.DividerAt(5, 5); ok {
		t.Fatal("no divider inside a pane")
	}
	d, ok := w.DividerAt(23, 3)
	if !ok {
		t.Fatal("divider not found on the border")
	}
	w.MoveDivider(d, 35, 3) // clamped so the right pane keeps 10 columns
	if got := widths(); got != [2]int{28, 12} {
		t.Fatalf("after drag: %v", got)
	}
	waitFor(t, func() bool { c, _ := w.Active().Size(); return c == 10 })
}

func TestRestorePanes(t *testing.T) {
	w, err := New("/bin/sh", 100, 40, 10)
	if err != nil {
		t.Skip("no /bin/sh:", err)
	}
	defer w.Close()
	if err := w.Split(true); err != nil {
		t.Fatal(err)
	}
	w.ResizePane(Right)
	if err := w.Split(false); err != nil {
		t.Fatal(err)
	}
	w.FocusNext() // focus the left pane
	rects := func(w *Workspace) (out [][5]int) {
		for _, p := range w.Panes() {
			active := 0
			if p.Active {
				active = 1
			}
			out = append(out, [5]int{p.X, p.Y, p.W, p.H, active})
		}
		return out
	}
	want := rects(w)

	// Through JSON, as on disk.
	path := filepath.Join(t.TempDir(), "tabs.json")
	if err := SaveState(path, w.Snapshot()); err != nil {
		t.Fatal(err)
	}
	r, err := Restore(LoadState(path), ReplayOff, "/bin/sh", 100, 40, 10)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if got := rects(r); !reflect.DeepEqual(got, want) {
		t.Fatalf("restored %v, want %v", got, want)
	}

	// A damaged layout falls back to one pane.
	st := State{Tabs: []SavedTab{{Dir: "/", Panes: &SavedPane{A: &SavedPane{}}}}}
	r2, err := Restore(st, ReplayOff, "/bin/sh", 100, 40, 10)
	if err != nil {
		t.Fatal(err)
	}
	defer r2.Close()
	if r2.PaneCount() != 1 {
		t.Fatalf("damaged layout gave %d panes", r2.PaneCount())
	}
}

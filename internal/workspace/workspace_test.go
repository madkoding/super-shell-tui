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
	first := w.tabs[0].sess
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
	want := State{Tabs: []SavedTab{{Dir: dir, Name: "api"}, {Dir: "/"}}, Active: 1}
	if err := SaveState(path, want); err != nil {
		t.Fatal(err)
	}
	if got := LoadState(path); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}

	w, err := Restore(want, "/bin/sh", 100, 40, 10)
	if err != nil {
		t.Skip("no /bin/sh:", err)
	}
	defer w.Close()
	if w.Len() != 2 || w.ActiveIndex() != 1 || w.Tabs()[0].Title != "api" {
		t.Fatalf("restored %+v active %d", w.Tabs(), w.ActiveIndex())
	}
	waitFor(t, func() bool { return w.Snapshot().Tabs[0].Dir == dir })

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

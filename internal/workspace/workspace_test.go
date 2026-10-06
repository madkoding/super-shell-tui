package workspace

import (
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

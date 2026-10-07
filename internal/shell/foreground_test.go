package shell

import (
	"strings"
	"testing"
	"time"
)

func TestShellJoin(t *testing.T) {
	got := shellJoin([]string{"npm", "run", "dev", "--port=3000", "it's here", "a b"})
	want := `npm run dev --port=3000 'it'\''s here' 'a b'`
	if got != want {
		t.Errorf("shellJoin = %s, want %s", got, want)
	}
}

var testShell = "/bin/bash"

func TestForegroundCommand(t *testing.T) {
	s, err := Start(testShell, "", 80, 24, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	waitIdle(t, s)
	if cmd := s.ForegroundCommand(); cmd != "" {
		t.Fatalf("idle shell reports %q", cmd)
	}
	_, _ = s.Write([]byte("sleep 30\n"))
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cmd := s.ForegroundCommand(); strings.HasPrefix(cmd, "sleep 30") {
			if !s.Busy() {
				t.Fatal("Busy is false while sleep runs")
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("foreground command = %q, want sleep 30", s.ForegroundCommand())
}

// waitIdle waits for the shell to take the terminal: right after Start the
// foreground group can still be the one being set up.
func waitIdle(t *testing.T, s *Session) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for s.Busy() {
		if time.Now().After(deadline) {
			t.Fatal("a fresh shell stays busy")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestLabel(t *testing.T) {
	s, err := Start(testShell, "/", 80, 24, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	waitIdle(t, s)
	if got := s.Label(); got != "/" {
		t.Fatalf("idle label = %q, want /", got)
	}
	_, _ = s.Write([]byte("sleep 30\n"))
	deadline := time.Now().Add(3 * time.Second)
	for s.Label() != "sleep" {
		if time.Now().After(deadline) {
			t.Fatalf("label = %q, want sleep", s.Label())
		}
		time.Sleep(50 * time.Millisecond)
	}
}

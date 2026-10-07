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
	if cmd := s.ForegroundCommand(); cmd != "" {
		t.Fatalf("idle shell reports %q", cmd)
	}
	_, _ = s.Write([]byte("sleep 30\n"))
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cmd := s.ForegroundCommand(); strings.HasPrefix(cmd, "sleep 30") {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("foreground command = %q, want sleep 30", s.ForegroundCommand())
}

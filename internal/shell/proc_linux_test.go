package shell

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestProcessCwd(t *testing.T) {
	want, _ := os.Getwd()
	if got := processCwd(os.Getpid()); got != want {
		t.Errorf("processCwd = %q, want %q", got, want)
	}
}

func TestProcessCommand(t *testing.T) {
	got := processCommand(os.Getpid())
	if got == "" || !strings.Contains(got, ".test") {
		t.Errorf("processCommand = %q, want the test binary", got)
	}
}

func TestFreshCwdSkipsCache(t *testing.T) {
	s, err := Start(testShell, "/", 80, 24, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if got := s.Cwd(); got != "/" {
		t.Fatalf("Cwd = %q, want /", got)
	}
	dir := t.TempDir()
	_, _ = s.Write([]byte("cd " + dir + "\n"))
	deadline := time.Now().Add(3 * time.Second)
	for s.FreshCwd() != dir {
		if time.Now().After(deadline) {
			t.Fatalf("FreshCwd = %q, want %q", s.FreshCwd(), dir)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if got := s.Cwd(); got != dir {
		t.Errorf("Cwd after FreshCwd = %q, want %q", got, dir)
	}
}

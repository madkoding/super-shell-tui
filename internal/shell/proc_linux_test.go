package shell

import (
	"os"
	"strings"
	"testing"
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

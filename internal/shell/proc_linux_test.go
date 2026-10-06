package shell

import (
	"os"
	"testing"
)

func TestProcessCwd(t *testing.T) {
	want, _ := os.Getwd()
	if got := processCwd(os.Getpid()); got != want {
		t.Errorf("processCwd = %q, want %q", got, want)
	}
}

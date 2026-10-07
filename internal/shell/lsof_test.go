package shell

import "testing"

func TestParseLsofCwd(t *testing.T) {
	tests := map[string]string{
		"p123\nfcwd\nn/Users/mad/src\n": "/Users/mad/src",
		"p123\nfcwd\nn/tmp/a b\n":       "/tmp/a b",
		"":                              "",
		"p123\nfcwd\n":                  "",
	}
	for in, want := range tests {
		if got := parseLsofCwd(in); got != want {
			t.Errorf("parseLsofCwd(%q) = %q, want %q", in, got, want)
		}
	}
}

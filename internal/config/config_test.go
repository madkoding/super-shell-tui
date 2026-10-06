package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePrefix(t *testing.T) {
	cases := map[string]struct {
		b     byte
		label string
	}{
		"ctrl+]":  {0x1d, "Ctrl+]"},
		"Ctrl+A":  {0x01, "Ctrl+A"},
		"ctrl+b":  {0x02, "Ctrl+B"},
		"ctrl+\\": {0x1c, "Ctrl+\\"},
		"ctrl+_":  {0x1f, "Ctrl+_"},
	}
	for in, want := range cases {
		b, label, err := ParsePrefix(in)
		if err != nil || b != want.b || label != want.label {
			t.Errorf("%q: got %#x %q %v", in, b, label, err)
		}
	}
	for _, bad := range []string{"ctrl+[", "alt+a", "ctrl+ab", "", "a"} {
		if _, _, err := ParsePrefix(bad); err == nil {
			t.Errorf("%q should fail", bad)
		}
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()

	cfg, err := Load(filepath.Join(dir, "missing.toml"))
	if err != nil || cfg != Default() {
		t.Fatalf("missing file: %+v %v", cfg, err)
	}

	path := filepath.Join(dir, "config.toml")
	if err := WriteSample(path); err != nil {
		t.Fatal(err)
	}
	if cfg, err := Load(path); err != nil || cfg != Default() {
		t.Fatalf("sample should equal defaults: %+v %v", cfg, err)
	}
	if err := WriteSample(path); err == nil {
		t.Fatal("WriteSample must not overwrite")
	}

	write := func(s string) error {
		_ = os.WriteFile(path, []byte(s), 0o644)
		_, err := Load(path)
		return err
	}
	if err := write("prefix = \"ctrl+a\"\nsidebar = false\n[colors]\naccent = \"#ff8800\"\n"); err != nil {
		t.Fatal(err)
	}
	cfg, _ = Load(path)
	if cfg.Prefix != "ctrl+a" || cfg.Sidebar || cfg.SidebarWidth != 30 || cfg.Colors.Accent != "#ff8800" {
		t.Fatalf("partial file: %+v", cfg)
	}
	for _, bad := range []string{"sidebar_width = 5", "scrollback = -1", "prefix = \"ctrl+[\"", "[colors]\naccent = \"red\"", "typo = 1"} {
		if err := write(bad); err == nil || !strings.Contains(err.Error(), path) {
			t.Errorf("%q: want error naming the file, got %v", bad, err)
		}
	}
}

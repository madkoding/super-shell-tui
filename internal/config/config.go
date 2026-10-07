// Package config loads the user's settings from a TOML file, by default
// $XDG_CONFIG_HOME/super-shell/config.toml (~/.config/super-shell on Linux,
// ~/Library/Application Support/super-shell on macOS).
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config holds every user-tunable setting. Absent keys keep their default.
type Config struct {
	// Shell to run; empty means $SHELL, then /bin/bash.
	Shell string `toml:"shell"`
	// Prefix is the key that introduces TUI commands, e.g. "ctrl+]" or "ctrl+a".
	Prefix string `toml:"prefix"`
	// Sidebar shows the side panel on start.
	Sidebar bool `toml:"sidebar"`
	// SidebarWidth is the side panel width in cells (16-80).
	SidebarWidth int `toml:"sidebar_width"`
	// Scrollback is how many history lines each shell keeps (1-100000).
	Scrollback int `toml:"scrollback"`
	// RestoreTabs reopens the previous run's tabs (directory and name).
	RestoreTabs bool `toml:"restore_tabs"`
	// RestoreCommand is what to do with the program each restored tab was
	// running: "off", "type" (left at the prompt) or "run".
	RestoreCommand string `toml:"restore_command"`
	// Colors are hex values ("#9D7CFF"); empty keeps the adaptive defaults.
	Colors Colors `toml:"colors"`
}

// Colors customizes the theme.
type Colors struct {
	Accent string `toml:"accent"`
	Muted  string `toml:"muted"`
}

// Default returns the built-in settings.
func Default() Config {
	return Config{
		Prefix:       "ctrl+]",
		Sidebar:      true,
		SidebarWidth: 30,
		Scrollback:   10000,
		RestoreTabs:  true,

		RestoreCommand: "type",
	}
}

// DefaultPath returns where the config file is looked up.
func DefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "super-shell", "config.toml")
}

// Load reads path (DefaultPath when empty). A missing file yields defaults.
func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		path = DefaultPath()
	}
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	meta, err := toml.Decode(string(data), &cfg)
	if err != nil {
		return cfg, fmt.Errorf("%s: %w", path, err)
	}
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		return cfg, fmt.Errorf("%s: unknown key %q", path, undecoded[0].String())
	}
	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

var hexColor = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// Validate checks ranges and formats.
func (c Config) Validate() error {
	if _, _, err := ParsePrefix(c.Prefix); err != nil {
		return err
	}
	if c.SidebarWidth < 16 || c.SidebarWidth > 80 {
		return fmt.Errorf("sidebar_width must be between 16 and 80, got %d", c.SidebarWidth)
	}
	if c.Scrollback < 1 || c.Scrollback > 100000 {
		return fmt.Errorf("scrollback must be between 1 and 100000, got %d", c.Scrollback)
	}
	switch c.RestoreCommand {
	case "off", "type", "run":
	default:
		return fmt.Errorf("restore_command must be \"off\", \"type\" or \"run\", got %q", c.RestoreCommand)
	}
	for name, v := range map[string]string{"colors.accent": c.Colors.Accent, "colors.muted": c.Colors.Muted} {
		if v != "" && !hexColor.MatchString(v) {
			return fmt.Errorf("%s must be a hex color like #9D7CFF, got %q", name, v)
		}
	}
	return nil
}

// ParsePrefix turns "ctrl+a".."ctrl+z", "ctrl+\", "ctrl+]", "ctrl+^" or
// "ctrl+_" into the control byte the terminal sends, plus a display label.
// Ctrl+[ is rejected: it is Escape, which every program needs.
func ParsePrefix(s string) (byte, string, error) {
	k, ok := strings.CutPrefix(strings.ToLower(strings.TrimSpace(s)), "ctrl+")
	if !ok || len(k) != 1 {
		return 0, "", fmt.Errorf("prefix must look like \"ctrl+a\" or \"ctrl+]\", got %q", s)
	}
	c := k[0]
	switch {
	case c >= 'a' && c <= 'z':
		return c - 'a' + 1, "Ctrl+" + strings.ToUpper(k), nil
	case c == '\\' || c == ']' || c == '^' || c == '_':
		return c & 0x1f, "Ctrl+" + k, nil
	}
	return 0, "", fmt.Errorf("unsupported prefix %q (Ctrl+[ is Escape)", s)
}

// Sample is a commented config file with the default values.
const Sample = `# Super Shell TUI configuration.

# Shell to run (empty: $SHELL, then /bin/bash).
shell = ""

# Key that introduces TUI commands: ctrl+a .. ctrl+z, ctrl+\, ctrl+], ctrl+^, ctrl+_
prefix = "ctrl+]"

# Show the side panel on start, and its width in cells (16-80).
sidebar = true
sidebar_width = 30

# History lines kept per shell (1-100000).
scrollback = 10000

# Reopen the previous run's tabs (directory and name) on start.
restore_tabs = true

# Program a restored tab was running when you quit (e.g. a dev server):
# "type" leaves it at the prompt for you to press Enter, "run" runs it,
# "off" ignores it.
restore_command = "type"

[colors]
# Hex colors; leave empty for the defaults that adapt to light/dark terminals.
accent = ""
muted = ""
`

// WriteSample creates path with Sample, refusing to overwrite a file.
func WriteSample(path string) error {
	if path == "" {
		return errors.New("no config directory available")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(Sample); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

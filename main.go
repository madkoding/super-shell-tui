// Command super-shell is a TUI that embeds an interactive shell in a pane.
// Keystrokes are forwarded raw to the shell's PTY so readline features
// (Tab completion, ↑/↓ history, Ctrl+R search) behave exactly as in a
// regular terminal.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/madkoding/super-shell-tui/internal/config"
	"github.com/madkoding/super-shell-tui/internal/input"
	"github.com/madkoding/super-shell-tui/internal/ui"
	"github.com/madkoding/super-shell-tui/internal/workspace"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "super-shell:", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", config.DefaultPath(), "config file (TOML)")
	initConfig := flag.Bool("init-config", false, "write a commented config file with the defaults and exit")
	shellFlag := flag.String("shell", "", "shell to run inside the pane (overrides the config)")
	flag.Parse()

	if *initConfig {
		if err := config.WriteSample(*configPath); err != nil {
			return fmt.Errorf("init config: %w", err)
		}
		fmt.Println("config written to", *configPath)
		return nil
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	prefix, prefixLabel, _ := config.ParsePrefix(cfg.Prefix) // validated by Load
	shellPath := firstNonEmpty(*shellFlag, cfg.Shell, os.Getenv("SHELL"), "/bin/bash")

	stdin := int(os.Stdin.Fd())
	if !term.IsTerminal(stdin) {
		return fmt.Errorf("stdin is not a terminal")
	}

	cols, rows, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		cols, rows = 80, 24
	}

	var saved workspace.State
	statePath := workspace.StatePath()
	if cfg.RestoreTabs {
		saved = workspace.LoadState(statePath)
	}
	ws, err := workspace.Restore(saved, shellPath, cfg.Scrollback, cols, rows)
	if err != nil {
		return fmt.Errorf("start shell: %w", err)
	}
	defer ws.Close()

	// Raw mode is set here, not by Bubble Tea: with WithInput(nil) the
	// program never reads stdin, so every byte reaches the PTY untouched.
	oldState, err := term.MakeRaw(stdin)
	if err != nil {
		return fmt.Errorf("raw mode: %w", err)
	}
	defer term.Restore(stdin, oldState) //nolint:errcheck

	model := ui.New(ws, ui.Options{
		ShellPath:    shellPath,
		PrefixLabel:  prefixLabel,
		ShowSidebar:  cfg.Sidebar,
		SidebarWidth: cfg.SidebarWidth,
		Accent:       cfg.Colors.Accent,
		Muted:        cfg.Colors.Muted,
	})
	model.Clipboard = ui.OSC52Clipboard
	// Bubble Tea enables bracketed paste on the real terminal by default;
	// input.Translator forwards or strips the markers per the shell's mode.
	// Mouse reporting (button events, SGR) is turned on for selection and the
	// wheel; the reports are decoded by input.Translator, not Bubble Tea.
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithInput(nil), tea.WithMouseCellMotion())

	tr := input.NewTranslator(prefix, ws)
	tr.OnMouse = func(ev input.MouseEvent) { p.Send(ui.MouseMsg(ev)) }
	tr.OnCapture = func(b []byte) { p.Send(ui.TextMsg(b)) }
	model.SetCapture = tr.Capture.Store
	go func() {
		_ = input.Pump(os.Stdin, ws, tr, func(a input.Action) {
			sess := ws.Active()
			if sess == nil {
				return
			}
			switch a {
			case input.ActionScrollPageUp:
				sess.ScrollPage(1)
			case input.ActionScrollPageDown:
				sess.ScrollPage(-1)
			case input.ActionScrollReset:
				sess.ResetScroll()
				sess.ClearSelection()
			default:
				p.Send(ui.ActionMsg(a))
			}
		})
	}()

	_, err = p.Run()
	if cfg.RestoreTabs {
		// Save the tabs still open (none if the last shell exited).
		if serr := workspace.SaveState(statePath, ws.Snapshot()); serr != nil && err == nil {
			err = fmt.Errorf("save tabs: %w", serr)
		}
	}
	return err
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

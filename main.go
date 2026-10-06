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

	"github.com/madkoding/super-shell-tui/internal/input"
	"github.com/madkoding/super-shell-tui/internal/shell"
	"github.com/madkoding/super-shell-tui/internal/ui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "super-shell:", err)
		os.Exit(1)
	}
}

func run() error {
	shellPath := flag.String("shell", defaultShell(), "shell to run inside the pane")
	flag.Parse()

	stdin := int(os.Stdin.Fd())
	if !term.IsTerminal(stdin) {
		return fmt.Errorf("stdin is not a terminal")
	}

	cols, rows, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		cols, rows = 80, 24
	}

	sess, err := shell.Start(*shellPath, cols, rows)
	if err != nil {
		return fmt.Errorf("start shell: %w", err)
	}
	defer sess.Close()

	// Raw mode is set here, not by Bubble Tea: with WithInput(nil) the
	// program never reads stdin, so every byte reaches the PTY untouched.
	oldState, err := term.MakeRaw(stdin)
	if err != nil {
		return fmt.Errorf("raw mode: %w", err)
	}
	defer term.Restore(stdin, oldState) //nolint:errcheck

	model := ui.New(sess, *shellPath)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithInput(nil))

	tr := input.NewTranslator(input.DefaultPrefix, sess.AppCursorMode)
	go func() {
		_ = input.Pump(os.Stdin, sess, tr, func(a input.Action) { p.Send(ui.ActionMsg(a)) })
	}()

	if _, err := p.Run(); err != nil {
		return err
	}
	return model.Err
}

func defaultShell() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	return "/bin/bash"
}

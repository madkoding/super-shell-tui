//go:build !linux && !darwin

package shell

// processCwd is unsupported here; tabs fall back to numbered titles.
func processCwd(int) string { return "" }

// processCommand is unsupported here; running commands are not restored.
func processCommand(int) string { return "" }

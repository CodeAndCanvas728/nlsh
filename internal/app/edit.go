package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Editor returns the editor to open a plan in.
func Editor() string {
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}
	if e := os.Getenv("VISUAL"); e != "" {
		return e
	}
	return "vim"
}

// EditCommands opens the commands in the user's editor and returns what
// they saved.
//
// The editor is run directly rather than through a shell, so a value in
// $EDITOR cannot turn into a shell command.
func EditCommands(commands []string) []string {
	dir, err := os.MkdirTemp("", "nlsh-edit-")
	if err != nil {
		return nil
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "plan.sh")
	if err := os.WriteFile(path, []byte(renderCommands(commands)), 0o600); err != nil {
		return nil
	}

	argv := strings.Fields(Editor())
	if len(argv) == 0 {
		return nil
	}
	argv = append(argv, path)

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// renderCommands renders the commands as an editable file.
func renderCommands(commands []string) string {
	return strings.Join(commands, "\n\n") + "\n"
}

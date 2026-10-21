//go:build windows

package run

import (
	"errors"
	"os/exec"
	"syscall"
)

// configureProcess puts the child in its own process group so that a
// Ctrl-C reaches the command rather than only nlsh.
func configureProcess(cmd *exec.Cmd) {
	const createNewProcessGroup = 0x00000200
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewProcessGroup}
}

// forwardSignals is a no-op on Windows: the console delivers Ctrl-C to
// every process in the group already.
func forwardSignals(cmd *exec.Cmd) func() {
	return func() {}
}

// exitStatus turns the error of a finished command into an exit status.
func exitStatus(err error) int {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return 127
	}
	if code := exitErr.ExitCode(); code >= 0 {
		return code
	}
	return 127
}

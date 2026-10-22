//go:build windows

package run

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
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

// processStatus returns the status of a command that has exited. Windows
// has no signal convention, so the exit code is used as it is.
func processStatus(state *os.ProcessState) int {
	if state == nil {
		return 127
	}
	if code := state.ExitCode(); code >= 0 {
		return code
	}
	return 127
}

// stopGroup stops the command and everything it started.
//
// Windows has no process groups to signal the way Unix does, so taskkill
// with /T is used to take the children with it. Stopping only the shell
// would leave the command running, still holding the output pipe, and the
// wait would not return — the same failure the timeout is meant to
// prevent. There is no graceful variant: Windows has no SIGTERM to send.
//
// It reports whether the group was still there to stop.
//
// os.Process.Signal only supports Kill on Windows, so there is no harmless
// liveness probe: any other signal comes back "not implemented". Kill
// reports ErrProcessDone when the process has already been reaped, which is
// the same check Go's own bookkeeping gives, so it is used instead.
func stopGroup(cmd *exec.Cmd, graceful bool) bool {
	if cmd.Process == nil {
		return false
	}
	if err := cmd.Process.Kill(); err != nil {
		// Already gone: Go has reaped it, and the pid may be recycled.
		return false
	}
	// Take the children with it. The shell has just been killed and has
	// not been reaped, so the pid is still ours.
	_ = exec.Command("taskkill", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F").Run()
	return true
}

//go:build !windows

package run

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// configureProcess puts the child in its own process group so that a
// Ctrl-C reaches the command rather than only nlsh.
func configureProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// forwardSignals relays interrupt and terminate signals to the command and
// returns a function that stops relaying.
func forwardSignals(cmd *exec.Cmd) func() {
	if cmd.Process == nil {
		return func() {}
	}

	ch := make(chan os.Signal, 2)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case s := <-ch:
				if pgid, err := syscall.Getpgid(cmd.Process.Pid); err == nil {
					_ = syscall.Kill(-pgid, s.(syscall.Signal))
				} else {
					_ = cmd.Process.Signal(s)
				}
			}
		}
	}()

	return func() {
		signal.Stop(ch)
		close(done)
	}
}

// exitStatus turns the error of a finished command into an exit status.
// A command killed by a signal reports 128 plus the signal number, the
// convention every shell uses.
func exitStatus(err error) int {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return 127
	}
	if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
		if status.Signaled() {
			return 128 + int(status.Signal())
		}
		return status.ExitStatus()
	}
	if code := exitErr.ExitCode(); code >= 0 {
		return code
	}
	return 127
}

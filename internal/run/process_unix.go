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

// processStatus returns the status of a command that has exited.
//
// A command killed by a signal has no exit code of its own, and
// os.ProcessState reports -1 for it. The shells report 128 plus the
// signal number, and so does this: a status of -1 would be meaningless
// to anything reading the exit code.
func processStatus(state *os.ProcessState) int {
	if state == nil {
		return 127
	}
	if status, ok := state.Sys().(syscall.WaitStatus); ok {
		if status.Signaled() {
			return 128 + int(status.Signal())
		}
		return status.ExitStatus()
	}
	if code := state.ExitCode(); code >= 0 {
		return code
	}
	return 127
}

// stopGroup stops the whole process group the command is in.
//
// The shell often forks rather than execs, so stopping only the shell
// leaves the command running, still holding the output pipe. The wait would
// then not return until the command finished on its own, which is exactly
// what a timeout is supposed to prevent.
//
// With graceful set the group is asked to stop first, so that it can clean
// up; otherwise it is killed outright.
//
// It reports whether the group was still there to stop.
//
// The group is only signalled while the child is still ours. A pid is
// released once the child has been reaped, and the group id is the child's
// pid, so a kill after the reap could reach an unrelated process group.
// Go takes the same lock when it reaps a process that it does when it
// signals one, so the check cannot overlap the reap: either the process is
// still running, or Go knows it has been reaped and refuses.
//
// The limitation: a command that exits on its own and leaves a background
// process behind has already been reaped by the time the group could be
// stopped, so the background process survives. The wait is bounded by
// cmd.WaitDelay, so it cannot hang, but it is not stopped.
func stopGroup(cmd *exec.Cmd, graceful bool) bool {
	if cmd.Process == nil {
		return false
	}
	if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
		// Already gone: Go has reaped it, and the pid may be recycled.
		return false
	}
	// The command runs in its own process group, so the group is the
	// process itself.
	if graceful {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	} else {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	return true
}

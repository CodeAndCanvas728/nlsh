// Package run executes the commands of a plan.
//
// Commands run one at a time, in order, through the platform's shell. The
// first failure stops the run and its exit status becomes nlsh's own, so
// that nlsh can be used inside a script.
package run

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/CodeAndCanvas728/nlsh/internal/platform"
)

// ExitTimedOut is the status returned when a command ran out of time. It is
// the same status the usual shells use for a timeout.
const ExitTimedOut = 124

// stopGrace is how long a command gets to stop by itself before it is
// stopped the hard way. A command that cleans up — a build, a tar, rsync —
// needs the chance to finish what it was doing.
const stopGrace = 3 * time.Second

// outputGrace bounds how long the wait for the output pipes lasts once the
// command itself has gone. A command that leaves a background process
// holding the pipe would otherwise keep nlsh waiting for ever.
const outputGrace = 2 * time.Second

// stopReason says why a command was stopped.
type stopReason int

const (
	// stopNone means the command was not stopped by nlsh.
	stopNone stopReason = iota
	// stopDeadline means the command was stopped because it ran out of
	// time.
	stopDeadline
	// stopCancelled means the command was stopped because the user
	// interrupted nlsh.
	stopCancelled
)

// Execute runs the commands in order and returns the exit status of the
// last one that ran.
//
// A non-zero status stops the run: the commands after it were planned on the
// assumption that the ones before it succeeded.
//
// Each command gets at most timeout. A timeout of zero means no limit, which
// is the default: the user asked for the command, and a backup or a build
// can legitimately take a long time.
func Execute(ctx context.Context, p platform.Profile, commands []string, stdout, stderr io.Writer, timeout time.Duration) int {
	for i, command := range commands {
		fmt.Fprintf(stdout, "\n  $ %s\n", command)
		status, timedOut := runOne(ctx, p, command, stdout, stderr, timeout)
		if timedOut {
			fmt.Fprintf(stderr, "nlsh: command %d timed out after %s; stopping.\n", i+1, timeout)
			return ExitTimedOut
		}
		if status != 0 {
			fmt.Fprintf(stderr, "nlsh: command %d exited with status %d; stopping.\n", i+1, status)
			return status
		}
	}
	return 0
}

// runOne runs a single command line through the platform's shell.
//
// It reports whether the command ran out of time, which is not the same as
// the context being done: a command that finished at the very moment the
// deadline expired succeeded, and saying it timed out would be a lie.
func runOne(ctx context.Context, p platform.Profile, command string, stdout, stderr io.Writer, timeout time.Duration) (status int, timedOut bool) {
	argv := append(append([]string{}, p.Shell()...), command)

	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// The command is not run under exec.CommandContext: that kills only
	// the shell, and a shell that forks leaves the command running, still
	// holding the output pipe, so the wait would not return until the
	// command finished on its own. The whole process group is stopped
	// instead.
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.WaitDelay = outputGrace
	configureProcess(cmd)

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(stderr, "nlsh: could not launch %s: %v\n", argv[0], err)
		return 127, false
	}

	// The context is watched here, and the goroutine is joined before
	// this returns, so the answer cannot arrive after the child has been
	// reaped and its pid recycled.
	finished := make(chan struct{})
	watcher := make(chan stopReason, 1)
	go func() {
		select {
		case <-ctx.Done():
		case <-finished:
			watcher <- stopNone
			return
		}

		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			// The timeout. Ask the command to stop, and insist if it
			// does not.
			if stopGroup(cmd, true) {
				select {
				case <-finished:
				case <-time.After(stopGrace):
					stopGroup(cmd, false)
				}
			}
			watcher <- stopDeadline
			return
		}

		// The user interrupted. The signal has already been relayed, so
		// the command gets a chance to clean up; if it has not gone by
		// itself shortly afterwards, it is stopped the hard way.
		select {
		case <-finished:
			watcher <- stopNone
		case <-time.After(stopGrace):
			stopGroup(cmd, false)
			watcher <- stopCancelled
		}
	}()

	stop := forwardSignals(cmd)
	err := cmd.Wait()
	stop()
	close(finished)
	reason := <-watcher

	// The command exited but something is still holding the output pipe:
	// it left a background process behind and did not really finish.
	if errors.Is(err, exec.ErrWaitDelay) {
		if reason == stopDeadline {
			return ExitTimedOut, true
		}
		return processStatus(cmd.ProcessState), false
	}

	switch {
	case err == nil:
		// It finished on its own. If the deadline expired at the same
		// moment, it still finished: that is not a timeout.
		return 0, false
	case reason == stopDeadline:
		return ExitTimedOut, true
	default:
		return exitStatus(err), false
	}
}

package run

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/CodeAndCanvas728/nlsh/internal/platform"
	"github.com/CodeAndCanvas728/nlsh/internal/shell"
)

// requirePOSIXShell skips the test when the platform's shell is not POSIX.
//
// The tests run real commands, so they use the profile of the machine they
// run on: the macOS profile runs commands through /bin/zsh, which a Linux
// runner does not have. The commands themselves are POSIX, and the Windows
// profile runs PowerShell, where they do not exist.
func requirePOSIXShell(t *testing.T) {
	t.Helper()
	sh := platform.Current().Shell()
	if len(sh) == 0 {
		t.Skip("the current platform has no shell: skipping the tests that run commands")
	}
	if !shell.IsPOSIX(sh[0]) {
		t.Skipf("%s is not a POSIX shell: skipping the tests that run commands", sh[0])
	}
}

func TestExecuteRunsEveryCommand(t *testing.T) {
	requirePOSIXShell(t)
	var out, errOut strings.Builder
	status := Execute(context.Background(), platform.Current(),
		[]string{"echo one", "echo two"}, &out, &errOut, 0)

	if status != 0 {
		t.Errorf("status = %d, want 0 (stderr: %s)", status, errOut.String())
	}
	if !strings.Contains(out.String(), "one") || !strings.Contains(out.String(), "two") {
		t.Errorf("output = %q, want both commands' output", out.String())
	}
}

func TestExecuteStopsAtTheFirstFailure(t *testing.T) {
	requirePOSIXShell(t)
	var out, errOut strings.Builder
	status := Execute(context.Background(), platform.Current(),
		[]string{"echo one", "false", "echo three"}, &out, &errOut, 0)

	if status != 1 {
		t.Errorf("status = %d, want 1", status)
	}
	if strings.Contains(out.String(), "three") {
		t.Errorf("output = %q, want the third command not to have run", out.String())
	}
	if !strings.Contains(errOut.String(), "stopping") {
		t.Errorf("stderr = %q, want it to say the run stopped", errOut.String())
	}
}

func TestExecutePassesTheExitStatusOn(t *testing.T) {
	requirePOSIXShell(t)
	var out, errOut strings.Builder
	status := Execute(context.Background(), platform.Current(),
		[]string{"sh -c 'exit 7'"}, &out, &errOut, 0)

	if status != 7 {
		t.Errorf("status = %d, want 7", status)
	}
}

// TestExecuteStopsACommandThatRunsOutOfTime is the test that was missing:
// the --timeout option was parsed and then never used, so a slow command ran
// for as long as it liked.
func TestExecuteStopsACommandThatRunsOutOfTime(t *testing.T) {
	requirePOSIXShell(t)
	var out, errOut strings.Builder

	start := time.Now()
	status := Execute(context.Background(), platform.Current(),
		[]string{"sleep 30"}, &out, &errOut, 300*time.Millisecond)
	elapsed := time.Since(start)

	if status != ExitTimedOut {
		t.Errorf("status = %d, want %d (the timeout status)", status, ExitTimedOut)
	}
	if elapsed > 10*time.Second {
		t.Errorf("the command ran for %s, want it stopped after the timeout", elapsed)
	}
	if !strings.Contains(errOut.String(), "timed out") {
		t.Errorf("stderr = %q, want it to say the command timed out", errOut.String())
	}
}

// TestExecuteStopsACommandThatOutlivesItsShell covers the case the Linux
// runner found: a shell that forks instead of execs. Killing the shell
// alone leaves the command running, still holding the output pipe, so the
// wait does not return until the command finishes on its own — which is
// exactly what a timeout is meant to prevent.
func TestExecuteStopsACommandThatOutlivesItsShell(t *testing.T) {
	requirePOSIXShell(t)
	var out, errOut strings.Builder

	start := time.Now()
	status := Execute(context.Background(), platform.Current(),
		[]string{"sleep 30 & wait"}, &out, &errOut, 300*time.Millisecond)
	elapsed := time.Since(start)

	if status != ExitTimedOut {
		t.Errorf("status = %d, want %d (the timeout status)", status, ExitTimedOut)
	}
	if elapsed > 10*time.Second {
		t.Errorf("the command ran for %s, want it stopped after the timeout", elapsed)
	}
}

// TestExecuteWithoutATimeoutWaits checks that a command is left alone
// when no limit was asked for: a timeout of zero means no limit.
func TestExecuteWithoutATimeoutWaits(t *testing.T) {
	requirePOSIXShell(t)
	var out, errOut strings.Builder

	start := time.Now()
	status := Execute(context.Background(), platform.Current(),
		[]string{"sleep 1"}, &out, &errOut, 0)
	elapsed := time.Since(start)

	if status != 0 {
		t.Errorf("status = %d, want 0", status)
	}
	if elapsed < 900*time.Millisecond {
		t.Errorf("the command took %s, want it to have been left to finish", elapsed)
	}
}

// TestExecuteStopsWhenTheContextIsCancelled covers a cancelled context:
// the command is stopped once the grace period is over.
//
// A cancellation is not a timeout: nothing ran out of time, and the run must
// not say that it did.
func TestExecuteStopsWhenTheContextIsCancelled(t *testing.T) {
	requirePOSIXShell(t)
	var out, errOut strings.Builder

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(300 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	status := Execute(ctx, platform.Current(), []string{"sleep 30"}, &out, &errOut, 0)
	elapsed := time.Since(start)

	// Nothing signalled it — the context was simply cancelled — so it was
	// killed the hard way once the grace period was over.
	if want := 128 + 9; status != want {
		t.Errorf("status = %d, want %d (killed after the grace period)", status, want)
	}
	if strings.Contains(errOut.String(), "timed out") {
		t.Errorf("stderr = %q, want it not to say the command timed out", errOut.String())
	}
	if elapsed > 20*time.Second {
		t.Errorf("the command ran for %s, want it stopped when the context was cancelled", elapsed)
	}
}

// TestExecuteKillsACommandThatIgnoresTheInterrupt covers the grace period:
// a command that ignores the signal is killed once the grace is over,
// rather than being left to run for ever.
func TestExecuteKillsACommandThatIgnoresTheInterrupt(t *testing.T) {
	requirePOSIXShell(t)
	var out, errOut strings.Builder

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(300 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	status := Execute(ctx, platform.Current(),
		[]string{`trap "" INT; sleep 30`}, &out, &errOut, 0)
	elapsed := time.Since(start)

	// It ignored the interrupt, so it was killed the hard way.
	if want := 128 + 9; status != want {
		t.Errorf("status = %d, want %d (killed after the grace period)", status, want)
	}
	if strings.Contains(errOut.String(), "timed out") {
		t.Errorf("stderr = %q, want it not to say the command timed out", errOut.String())
	}
	if elapsed > 20*time.Second {
		t.Errorf("the command ran for %s, want it killed after the grace period", elapsed)
	}
}

// TestExecuteStopsACommandThatLeftABackgroundProcess states a limitation
// rather than leaving it assumed.
//
// The shell exits at once and leaves a background process holding the
// output pipe. The shell has been reaped by then, so the process group
// cannot be stopped any more: the group id belongs to a pid that may have
// been recycled. The run does not hang — the wait for the pipe is bounded —
// and it reports the timeout, but the background process survives.
func TestExecuteStopsACommandThatLeftABackgroundProcess(t *testing.T) {
	requirePOSIXShell(t)
	var out, errOut strings.Builder

	start := time.Now()
	status := Execute(context.Background(), platform.Current(),
		[]string{"sleep 30 &"}, &out, &errOut, 2*time.Second)
	elapsed := time.Since(start)

	if status != ExitTimedOut {
		t.Errorf("status = %d, want %d (the command did not finish)", status, ExitTimedOut)
	}
	if elapsed > 20*time.Second {
		t.Errorf("the command ran for %s, want it stopped at the timeout", elapsed)
	}
	if !strings.Contains(errOut.String(), "timed out") {
		t.Errorf("stderr = %q, want it to say the command timed out", errOut.String())
	}
}

// TestExecuteReportsTheStatusOfASignalledCommandThatLeftABackgroundProcess
// covers the command that was killed by a signal and left a background
// process holding the output pipe.
//
// The wait ends for the background process rather than for the command,
// and a signalled command has no exit code of its own: the status has to
// be the shell's convention for one, not -1.
func TestExecuteReportsTheStatusOfASignalledCommandThatLeftABackgroundProcess(t *testing.T) {
	requirePOSIXShell(t)
	var out, errOut strings.Builder

	status := Execute(context.Background(), platform.Current(),
		[]string{"sleep 30 & kill -KILL $$"}, &out, &errOut, 0)

	if want := 128 + 9; status != want {
		t.Errorf("status = %d, want %d (killed by SIGKILL)", status, want)
	}
	if strings.Contains(errOut.String(), "status -1") {
		t.Errorf("stderr = %q, want it not to report a status of -1", errOut.String())
	}
}

// TestExecuteDoesNotHangWhenTheCommandLeftABackgroundProcessAndTheRunWasCancelled
// covers the same case with the run interrupted: the wait still has to end
// and the status still has to mean something.
func TestExecuteDoesNotHangWhenTheCommandLeftABackgroundProcessAndTheRunWasCancelled(t *testing.T) {
	requirePOSIXShell(t)
	var out, errOut strings.Builder

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(300 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	status := Execute(ctx, platform.Current(), []string{"sleep 30 &"}, &out, &errOut, 0)
	elapsed := time.Since(start)

	if status < 0 {
		t.Errorf("status = %d, want a status that means something", status)
	}
	if strings.Contains(errOut.String(), "status -1") {
		t.Errorf("stderr = %q, want it not to report a status of -1", errOut.String())
	}
	if elapsed > 20*time.Second {
		t.Errorf("the run took %s, want it to end when the command was interrupted", elapsed)
	}
}

// TestProcessStatusOfASignalledCommand guards the conversion of a
// signalled status.
//
// A command killed by a signal has no exit code of its own, and
// os.ProcessState reports -1 for one. The status has to be the shell's
// convention, because -1 means nothing to anything reading it.
func TestProcessStatusOfASignalledCommand(t *testing.T) {
	requirePOSIXShell(t)

	cmd := exec.Command("/bin/sh", "-c", "kill -KILL $$")
	_ = cmd.Run()

	if cmd.ProcessState == nil {
		t.Fatal("the command has no process state")
	}
	if got := cmd.ProcessState.ExitCode(); got != -1 {
		t.Skipf("this platform reports %d for a signalled command, not -1", got)
	}
	want := 128 + 9
	if got := processStatus(cmd.ProcessState); got != want {
		t.Errorf("processStatus = %d, want %d (killed by SIGKILL)", got, want)
	}
}

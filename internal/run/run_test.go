package run

import (
	"context"
	"strings"
	"testing"

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
		[]string{"echo one", "echo two"}, &out, &errOut)

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
		[]string{"echo one", "false", "echo three"}, &out, &errOut)

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
		[]string{"sh -c 'exit 7'"}, &out, &errOut)

	if status != 7 {
		t.Errorf("status = %d, want 7", status)
	}
}

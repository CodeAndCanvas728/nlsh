//go:build !windows

package run

import (
	"context"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/CodeAndCanvas728/nlsh/internal/platform"
)

// TestExecuteRelaysAnInterruptToTheCommand covers the interrupt path with a
// real signal, the way Ctrl+C arrives.
//
// The signal is relayed to the command rather than the group being killed
// outright, so a command that cleans up — a build, a tar, rsync — gets the
// chance to. The command here dies of the interrupt itself.
//
// An interrupt is not a timeout: nothing ran out of time, and the run must
// not say that it did.
func TestExecuteRelaysAnInterruptToTheCommand(t *testing.T) {
	requirePOSIXShell(t)
	var out, errOut strings.Builder

	go func() {
		time.Sleep(300 * time.Millisecond)
		// The run registers a handler for this signal, so the test
		// process is not killed by it.
		_ = syscall.Kill(os.Getpid(), syscall.SIGINT)
	}()

	start := time.Now()
	status := Execute(context.Background(), platform.Current(), []string{"sleep 30"}, &out, &errOut, 0)
	elapsed := time.Since(start)

	// The command was signalled, so it died of the interrupt.
	if want := 128 + 2; status != want {
		t.Errorf("status = %d, want %d (killed by the interrupt)", status, want)
	}
	if strings.Contains(errOut.String(), "timed out") {
		t.Errorf("stderr = %q, want it not to say the command timed out", errOut.String())
	}
	if elapsed > 20*time.Second {
		t.Errorf("the command ran for %s, want it stopped by the interrupt", elapsed)
	}
}

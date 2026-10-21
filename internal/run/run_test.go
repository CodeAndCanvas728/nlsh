package run

import (
	"context"
	"strings"
	"testing"

	"github.com/jonah/nlsh/internal/platform"
)

func TestExecuteRunsEveryCommand(t *testing.T) {
	var out, errOut strings.Builder
	status := Execute(context.Background(), platform.Darwin,
		[]string{"echo one", "echo two"}, &out, &errOut)

	if status != 0 {
		t.Errorf("status = %d, want 0 (stderr: %s)", status, errOut.String())
	}
	if !strings.Contains(out.String(), "one") || !strings.Contains(out.String(), "two") {
		t.Errorf("output = %q, want both commands' output", out.String())
	}
}

func TestExecuteStopsAtTheFirstFailure(t *testing.T) {
	var out, errOut strings.Builder
	status := Execute(context.Background(), platform.Darwin,
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
	var out, errOut strings.Builder
	status := Execute(context.Background(), platform.Darwin,
		[]string{"sh -c 'exit 7'"}, &out, &errOut)

	if status != 7 {
		t.Errorf("status = %d, want 7", status)
	}
}

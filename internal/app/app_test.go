package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/CodeAndCanvas728/nlsh/internal/cli"
	"github.com/CodeAndCanvas728/nlsh/internal/plan"
	"github.com/CodeAndCanvas728/nlsh/internal/platform"
	"github.com/CodeAndCanvas728/nlsh/internal/report"
	"github.com/CodeAndCanvas728/nlsh/internal/run"
	"github.com/CodeAndCanvas728/nlsh/internal/shell"
	"github.com/CodeAndCanvas728/nlsh/internal/validate"
)

// requirePOSIXShell skips the test when the platform's shell is not POSIX.
//
// These tests run real commands and check real tools: `ls --not-a-real-flag`
// is only an invalid flag because BSD and GNU ls both reject it, and
// `false` and `sh` are POSIX programs. The Windows profile runs PowerShell,
// where none of that holds.
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

// fakePlanner returns canned plans and records what it was asked.
type fakePlanner struct {
	plans    []plan.Plan
	regen    []plan.Plan
	calls    int
	feedback []string
}

func (f *fakePlanner) Plan(ctx context.Context, request string) (plan.Plan, error) {
	i := f.calls
	f.calls++
	if i >= len(f.plans) {
		return plan.Plan{}, nil
	}
	return f.plans[i], nil
}

func (f *fakePlanner) Regenerate(ctx context.Context, feedback string) (plan.Plan, error) {
	f.feedback = append(f.feedback, feedback)
	if len(f.regen) > 0 {
		p := f.regen[0]
		f.regen = f.regen[1:]
		return p, nil
	}
	if len(f.plans) > 1 {
		return f.plans[1], nil
	}
	return plan.Plan{}, nil
}

// newTestApp builds an App that writes to buffers instead of the terminal.
func newTestApp(t *testing.T, opts cli.Options, planner *fakePlanner) (*App, *strings.Builder, *strings.Builder) {
	t.Helper()
	var out, errOut strings.Builder
	reporter := report.New(&out, &errOut, opts.PrintMode)
	// The profile of the machine the test runs on, so that the commands
	// the tests execute go through a shell that exists.
	profile := platform.Current()
	return &App{
		Profile:  platform.Current(),
		Planner:  planner,
		Checker:  validate.New(profile),
		Reporter: reporter,
		Options:  opts,
		Out:      &out,
		Err:      &errOut,
	}, &out, &errOut
}

func TestPrintModeWritesOnlyCommandsToStdout(t *testing.T) {
	requirePOSIXShell(t)
	planner := &fakePlanner{plans: []plan.Plan{{
		Commands:    []plan.Command{{Command: "ls -l", Risk: "safe"}},
		Explanation: "List the files.",
	}}}
	a, out, errOut := newTestApp(t, cli.Options{PrintMode: true}, planner)

	status := a.Handle(context.Background(), "list the files")
	if status != ExitOK {
		t.Errorf("status = %d, want 0", status)
	}
	if got := out.String(); got != "ls -l\n" {
		t.Errorf("stdout = %q, want %q", got, "ls -l\n")
	}
	if !strings.Contains(errOut.String(), "thinking") {
		t.Errorf("stderr = %q, want it to carry the progress messages", errOut.String())
	}
	if strings.Contains(out.String(), "thinking") {
		t.Errorf("stdout = %q, want no progress messages", out.String())
	}
}

func TestPrintModeExitsTwoWhenIssuesRemain(t *testing.T) {
	requirePOSIXShell(t)
	planner := &fakePlanner{plans: []plan.Plan{{
		Commands:    []plan.Command{{Command: "ls --not-a-real-flag", Risk: "safe"}},
		Explanation: "List it.",
	}}}
	a, _, errOut := newTestApp(t, cli.Options{PrintMode: true}, planner)

	status := a.Handle(context.Background(), "list it")
	if status != ExitInvalid {
		t.Errorf("status = %d, want %d", status, ExitInvalid)
	}
	if !strings.Contains(errOut.String(), "validation issues remain") {
		t.Errorf("stderr = %q, want it to report the remaining issues", errOut.String())
	}
}

func TestSudoIsRefused(t *testing.T) {
	requirePOSIXShell(t)
	planner := &fakePlanner{plans: []plan.Plan{{
		Commands:    []plan.Command{{Command: "sudo rm -rf /", Risk: "destructive"}},
		Explanation: "Remove everything.",
	}}}
	a, out, _ := newTestApp(t, cli.Options{Force: true}, planner)

	status := a.Handle(context.Background(), "remove everything")
	if status != ExitInvalid {
		t.Errorf("status = %d, want %d", status, ExitInvalid)
	}
	if !strings.Contains(out.String(), "Run it yourself") {
		t.Errorf("stdout = %q, want it to show the refused command", out.String())
	}
}

func TestDryRunRunsNothing(t *testing.T) {
	requirePOSIXShell(t)
	planner := &fakePlanner{plans: []plan.Plan{{
		Commands:    []plan.Command{{Command: "echo hello", Risk: "safe"}},
		Explanation: "Say hello.",
	}}}
	a, out, _ := newTestApp(t, cli.Options{DryRun: true}, planner)

	status := a.Handle(context.Background(), "say hello")
	if status != ExitOK {
		t.Errorf("status = %d, want 0", status)
	}
	if strings.Contains(out.String(), "hello\n") {
		t.Errorf("stdout = %q, want the command not to have been run", out.String())
	}
	if !strings.Contains(out.String(), "dry run") {
		t.Errorf("stdout = %q, want it to mention the dry run", out.String())
	}
}

func TestForceRunsTheCommands(t *testing.T) {
	requirePOSIXShell(t)
	planner := &fakePlanner{plans: []plan.Plan{{
		Commands:    []plan.Command{{Command: "echo hello", Risk: "safe"}},
		Explanation: "Say hello.",
	}}}
	a, out, _ := newTestApp(t, cli.Options{Force: true}, planner)

	status := a.Handle(context.Background(), "say hello")
	if status != ExitOK {
		t.Errorf("status = %d, want 0", status)
	}
	if !strings.Contains(out.String(), "hello") {
		t.Errorf("stdout = %q, want the command's output", out.String())
	}
}

func TestFailedCommandPropagatesItsStatus(t *testing.T) {
	requirePOSIXShell(t)
	planner := &fakePlanner{plans: []plan.Plan{{
		Commands:    []plan.Command{{Command: "sh -c 'exit 3'", Risk: "safe"}},
		Explanation: "Fail.",
	}}}
	a, _, _ := newTestApp(t, cli.Options{Force: true}, planner)

	status := a.Handle(context.Background(), "fail")
	if status != 3 {
		t.Errorf("status = %d, want 3", status)
	}
}

// TestTimeoutReachesTheRunner is the test that was missing at the wiring
// level: the --timeout option was parsed and then never used, so a slow
// command ran for as long as it liked.
func TestTimeoutReachesTheRunner(t *testing.T) {
	requirePOSIXShell(t)
	planner := &fakePlanner{plans: []plan.Plan{{
		Commands:    []plan.Command{{Command: "sleep 30", Risk: "safe"}},
		Explanation: "Wait for a while.",
	}}}
	a, _, errOut := newTestApp(t, cli.Options{Force: true, Timeout: 300 * time.Millisecond}, planner)

	start := time.Now()
	status := a.Handle(context.Background(), "wait for a while")
	elapsed := time.Since(start)

	if status != run.ExitTimedOut {
		t.Errorf("status = %d, want %d (the timeout status)", status, run.ExitTimedOut)
	}
	if elapsed > 10*time.Second {
		t.Errorf("the command ran for %s, want it stopped after the timeout", elapsed)
	}
	if !strings.Contains(errOut.String(), "timed out") {
		t.Errorf("stderr = %q, want it to say the command timed out", errOut.String())
	}
}

func TestValidationFailureTriggersOneRegeneration(t *testing.T) {
	requirePOSIXShell(t)
	planner := &fakePlanner{plans: []plan.Plan{
		{
			Commands:    []plan.Command{{Command: "definitely-not-a-real-binary --wat", Risk: "safe"}},
			Explanation: "Do something.",
		},
		{
			Commands:    []plan.Command{{Command: "ls -l", Risk: "safe"}},
			Explanation: "Do something.",
		},
	}}
	a, _, _ := newTestApp(t, cli.Options{PrintMode: true}, planner)

	a.Handle(context.Background(), "do something")

	if len(planner.feedback) != 1 {
		t.Fatalf("got %d regenerations, want 1", len(planner.feedback))
	}
	if !strings.Contains(planner.feedback[0], "not found in PATH") {
		t.Errorf("feedback = %q, want it to mention the missing binary", planner.feedback[0])
	}
}

func TestValidationFailureRegeneratesUntilClean(t *testing.T) {
	requirePOSIXShell(t)
	broken := plan.Plan{
		Commands:    []plan.Command{{Command: "definitely-not-a-real-binary --wat", Risk: "safe"}},
		Explanation: "Do something.",
	}
	good := plan.Plan{
		Commands:    []plan.Command{{Command: "ls -l", Risk: "safe"}},
		Explanation: "Do something.",
	}
	planner := &fakePlanner{
		plans: []plan.Plan{broken},
		regen: []plan.Plan{broken, good},
	}
	a, out, errOut := newTestApp(t, cli.Options{PrintMode: true}, planner)

	status := a.Handle(context.Background(), "do something")

	if status != ExitOK {
		t.Errorf("status = %d, want 0", status)
	}
	if len(planner.feedback) != 2 {
		t.Fatalf("got %d regenerations, want 2 (the first fix was still broken)", len(planner.feedback))
	}
	if got := out.String(); got != "ls -l\n" {
		t.Errorf("stdout = %q, want the corrected plan's command", got)
	}
	if !strings.Contains(errOut.String(), "regenerated plan passes validation") {
		t.Errorf("stderr = %q, want it to report the clean regeneration", errOut.String())
	}
}

// TestPrintModeReportsCaveatWhenNoCommands: in --print mode an empty plan
// (an ambiguous request answered with a question in caveat) must still
// reach the user — on stderr, so the stdout contract holds — or the zsh
// buffer comes back empty with no explanation.
func TestPrintModeReportsCaveatWhenNoCommands(t *testing.T) {
	requirePOSIXShell(t)
	planner := &fakePlanner{plans: []plan.Plan{{
		Commands:    nil,
		Explanation: "What counts as clutter?",
		Caveat:      "Ask: delete files older than a given age?",
	}}}
	a, out, errOut := newTestApp(t, cli.Options{PrintMode: true}, planner)

	status := a.Handle(context.Background(), "clean up my downloads folder")

	if status != ExitOK {
		t.Errorf("status = %d, want 0", status)
	}
	if got := out.String(); got != "" {
		t.Errorf("stdout = %q, want it empty (no commands)", got)
	}
	if !strings.Contains(errOut.String(), "delete files older than a given age") {
		t.Errorf("stderr = %q, want it to carry the caveat", errOut.String())
	}
}

func TestRegenerationStopsAtTheLimit(t *testing.T) {
	requirePOSIXShell(t)
	broken := plan.Plan{
		Commands:    []plan.Command{{Command: "definitely-not-a-real-binary --wat", Risk: "safe"}},
		Explanation: "Do something.",
	}
	planner := &fakePlanner{
		plans: []plan.Plan{broken},
		regen: []plan.Plan{broken, broken, broken},
	}
	a, _, errOut := newTestApp(t, cli.Options{PrintMode: true}, planner)

	status := a.Handle(context.Background(), "do something")

	if status != ExitInvalid {
		t.Errorf("status = %d, want %d (issues remain)", status, ExitInvalid)
	}
	if len(planner.feedback) != maxRegenerations {
		t.Errorf("got %d regenerations, want the cap of %d", len(planner.feedback), maxRegenerations)
	}
	if !strings.Contains(errOut.String(), "remaining issues") {
		t.Errorf("stderr = %q, want it to report the remaining issues", errOut.String())
	}
}

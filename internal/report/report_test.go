package report

import (
	"strings"
	"testing"

	"github.com/jonah/nlsh/internal/guard"
	"github.com/jonah/nlsh/internal/plan"
)

func TestMachineModeKeepsStdoutClean(t *testing.T) {
	var out, errOut strings.Builder
	r := New(&out, &errOut, true)

	r.Progress("thinking...")
	r.Warn("something is off")
	r.Error("something broke")
	r.Note("an ordinary note")
	r.Command("ls -l")

	if got := out.String(); got != "ls -l\n" {
		t.Errorf("stdout = %q, want only the command", got)
	}
	for _, want := range []string{"thinking", "something is off", "something broke", "an ordinary note"} {
		if !strings.Contains(errOut.String(), want) {
			t.Errorf("stderr = %q, want it to contain %q", errOut.String(), want)
		}
	}
}

func TestHumanModeWritesToStdout(t *testing.T) {
	var out, errOut strings.Builder
	r := New(&out, &errOut, false)

	r.Progress("thinking...")
	r.Note("an ordinary note")
	r.Command("ls -l")

	if !strings.Contains(out.String(), "an ordinary note") {
		t.Errorf("stdout = %q, want the note", out.String())
	}
	if !strings.Contains(out.String(), "ls -l") {
		t.Errorf("stdout = %q, want the command", out.String())
	}
	// Progress always goes to stderr, so that redirecting stdout does
	// not hide it.
	if !strings.Contains(errOut.String(), "thinking") {
		t.Errorf("stderr = %q, want the progress message", errOut.String())
	}
}

func TestPlanIsHiddenInMachineMode(t *testing.T) {
	var out, errOut strings.Builder
	r := New(&out, &errOut, true)

	r.Plan(plan.Plan{
		Commands:    []plan.Command{{Command: "ls -l", Risk: "safe"}},
		Explanation: "List the files.",
	}, []guard.Risk{guard.RiskSafe})

	if out.Len() != 0 {
		t.Errorf("stdout = %q, want nothing", out.String())
	}
}

func TestPlanShowsTheRiskBadge(t *testing.T) {
	var out, errOut strings.Builder
	r := New(&out, &errOut, false)

	r.Plan(plan.Plan{
		Commands:    []plan.Command{{Command: "rm -rf /", Risk: "destructive"}},
		Explanation: "Remove everything.",
		Caveat:      "This is permanent.",
	}, []guard.Risk{guard.RiskDestructive})

	text := out.String()
	for _, want := range []string{"Remove everything.", "This is permanent.", "rm -rf /", "DESTRUCTIVE"} {
		if !strings.Contains(text, want) {
			t.Errorf("output = %q, want it to contain %q", text, want)
		}
	}
}

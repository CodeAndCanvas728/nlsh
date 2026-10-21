package guard

import (
	"testing"

	"github.com/CodeAndCanvas728/nlsh/internal/plan"
	"github.com/CodeAndCanvas728/nlsh/internal/platform"
)

func TestFromTag(t *testing.T) {
	cases := []struct {
		tag  string
		want Risk
	}{
		{"safe", RiskSafe},
		{"SAFE", RiskSafe},
		{" read-only ", RiskSafe},
		{"careful", RiskCareful},
		{"cautious", RiskCareful},
		{"destructive", RiskDestructive},
		{"dangerous", RiskDestructive},
		{"danger", RiskDestructive},
		{"", RiskCareful},
		{"who knows", RiskCareful},
	}

	for _, tc := range cases {
		t.Run(tc.tag, func(t *testing.T) {
			if got := FromTag(tc.tag); got != tc.want {
				t.Errorf("FromTag(%q) = %v, want %v", tc.tag, got, tc.want)
			}
		})
	}
}

func TestClassifyNeverLowersTheRisk(t *testing.T) {
	cases := []struct {
		name    string
		command plan.Command
		want    Risk
	}{
		{
			name:    "the model says safe and it is safe",
			command: plan.Command{Command: "ls -l", Risk: "safe"},
			want:    RiskSafe,
		},
		{
			name:    "the model says safe but it deletes",
			command: plan.Command{Command: "rm -rf /tmp/x", Risk: "safe"},
			want:    RiskDestructive,
		},
		{
			name:    "the model says destructive",
			command: plan.Command{Command: "ls -l", Risk: "destructive"},
			want:    RiskDestructive,
		},
		{
			name:    "sudo with a delete is destructive as well as refused",
			command: plan.Command{Command: "sudo rm -rf /", Risk: "safe"},
			want:    RiskDestructive,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Classify(platform.Darwin, tc.command); got != tc.want {
				t.Errorf("Classify(%q) = %v, want %v", tc.command.Command, got, tc.want)
			}
		})
	}
}

func TestMax(t *testing.T) {
	if got := Max(RiskSafe, RiskDestructive); got != RiskDestructive {
		t.Errorf("got %v, want destructive", got)
	}
	if got := Max(RiskCareful, RiskSafe); got != RiskCareful {
		t.Errorf("got %v, want careful", got)
	}
}

func TestRiskString(t *testing.T) {
	cases := map[Risk]string{
		RiskSafe:        "safe",
		RiskCareful:     "careful",
		RiskDestructive: "destructive",
	}
	for risk, want := range cases {
		if got := risk.String(); got != want {
			t.Errorf("Risk(%d).String() = %q, want %q", risk, got, want)
		}
	}
}

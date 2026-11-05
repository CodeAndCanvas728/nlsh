package app

import (
	"strings"
	"testing"

	"github.com/CodeAndCanvas728/nlsh/internal/plan"
	"github.com/CodeAndCanvas728/nlsh/internal/platform"
)

// TestSystemPromptReadsIntentNotJustLiteralWords guards the change that
// matters most for the on-device model: the prompt must ask for the intent
// behind the words, and must never tell the model to take the most literal
// reading — that instruction is what made plans literal.
func TestSystemPromptReadsIntentNotJustLiteralWords(t *testing.T) {
	p := SystemPrompt(platform.Current())
	if strings.Contains(p, "most literal reading") {
		t.Error("system prompt still tells the model to take the most literal reading")
	}
	for _, want := range []string{
		"what the user wants to accomplish",
		"empty commands array",
		"restating what you understood",
		"Examples",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("system prompt does not contain %q", want)
		}
	}
	// The ambiguous-and-harmful request must be shown as the
	// ask-instead-of-guess case, never as a destructive guess with an
	// invented cutoff: an example wins over a rule for the on-device model.
	if !strings.Contains(p, `{"commands":[]`) {
		t.Error("system prompt examples do not show the empty-commands ask case")
	}
	if strings.Contains(p, "mtime +30") {
		t.Error("system prompt still contains the invented-cutoff delete example")
	}
}

// TestSystemPromptExamplesComeFromTheProfile: the examples must be in the
// shell of the profile they are sent with, so a Windows prompt is never
// shown POSIX commands.
func TestSystemPromptExamplesComeFromTheProfile(t *testing.T) {
	win := SystemPrompt(platform.Windows)
	if !strings.Contains(win, "Get-ChildItem") {
		t.Error("windows system prompt does not contain the PowerShell examples")
	}
	if strings.Contains(win, "du -ah .") {
		t.Error("windows system prompt contains POSIX examples")
	}
	posix := SystemPrompt(platform.Darwin)
	if !strings.Contains(posix, "du -ah .") {
		t.Error("darwin system prompt does not contain the POSIX examples")
	}
}

// TestSystemPromptExamplesAreValidPlans keeps the few-shot examples honest:
// every example line that looks like JSON must decode as a plan with a valid
// risk label, or the model is being shown a shape it cannot produce.
func TestSystemPromptExamplesAreValidPlans(t *testing.T) {
	p := SystemPrompt(platform.Current())
	examples := 0
	for _, line := range strings.Split(p, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") {
			continue
		}
		examples++
		pl, err := plan.Decode(line)
		if err != nil {
			t.Errorf("example %q does not decode as a plan: %v", line, err)
			continue
		}
		for _, c := range pl.Commands {
			if !plan.Risk(c.Risk).Valid() {
				t.Errorf("example command %q has invalid risk %q", c.Command, c.Risk)
			}
		}
	}
	if examples < 2 {
		t.Errorf("found %d examples in the system prompt, want at least 2", examples)
	}
}

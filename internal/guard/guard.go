// Package guard decides how risky a command is.
//
// The planner reports a risk with every command, but that label is never
// trusted on its own: it is combined with nlsh's own detection, which can
// only raise the risk. A command the planner calls "safe" that turns out to
// be `rm -rf` is still treated as destructive.
package guard

import (
	"strings"

	"github.com/CodeAndCanvas728/nlsh/internal/plan"
	"github.com/CodeAndCanvas728/nlsh/internal/platform"
)

// Risk is how much a command should be trusted.
type Risk int

const (
	// RiskSafe means the command only reads state.
	RiskSafe Risk = iota
	// RiskCareful means the command creates or changes something.
	RiskCareful
	// RiskDestructive means the command deletes, overwrites or resets.
	RiskDestructive
)

// String returns the lowercase name of the risk level.
func (r Risk) String() string {
	switch r {
	case RiskCareful:
		return "careful"
	case RiskDestructive:
		return "destructive"
	default:
		return "safe"
	}
}

// Classify returns the effective risk of a command: the higher of the risk
// the planner reported and the risk nlsh detected locally.
func Classify(p platform.Profile, c plan.Command) Risk {
	risk := FromTag(c.Risk)
	if p.IsDestructive(c.Command) {
		return RiskDestructive
	}
	return risk
}

// FromTag maps a planner-supplied risk label to a Risk.
//
// An unrecognised label is treated as careful rather than safe: a backend
// that ignores the schema is improvising, and improvising about risk should
// cost an extra confirmation, not a free pass.
func FromTag(tag string) Risk {
	switch strings.ToLower(strings.TrimSpace(tag)) {
	case "safe", "read-only", "readonly":
		return RiskSafe
	case "careful", "cautious":
		return RiskCareful
	case "destructive", "dangerous", "danger":
		return RiskDestructive
	default:
		return RiskCareful
	}
}

// Max returns the higher of two risks.
func Max(a, b Risk) Risk {
	if a > b {
		return a
	}
	return b
}

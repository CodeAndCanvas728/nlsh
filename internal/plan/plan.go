// Package plan defines the contract between nlsh and a planner backend.
//
// A plan is deliberately tiny: an ordered list of commands, each with a
// self-assessed risk, plus one sentence of explanation and an optional
// caveat. Everything else nlsh knows how to do — validation, risk
// re-classification, approval, execution — operates on this shape.
package plan

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Risk is the self-assessed risk of a single command, as reported by the
// planner. It is advisory: nlsh always combines it with its own local
// detection, which can only raise the risk, never lower it.
type Risk string

const (
	// RiskSafe means the command only reads state.
	RiskSafe Risk = "safe"
	// RiskCareful means the command creates, moves or modifies something.
	RiskCareful Risk = "careful"
	// RiskDestructive means the command deletes, overwrites, resets or
	// force-pushes.
	RiskDestructive Risk = "destructive"
)

// Valid reports whether r is one of the three known risk levels.
func (r Risk) Valid() bool {
	switch r {
	case RiskSafe, RiskCareful, RiskDestructive:
		return true
	}
	return false
}

// Command is one shell command in a plan.
type Command struct {
	Command string `json:"command"`
	Risk    string `json:"risk"`
}

// Plan is a complete, ordered plan.
type Plan struct {
	Commands    []Command `json:"commands"`
	Explanation string    `json:"explanation"`
	Caveat      string    `json:"caveat"`
}

// Decode parses a plan from the raw JSON a backend returned.
func Decode(raw string) (Plan, error) {
	var p Plan
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return Plan{}, fmt.Errorf("plan is not valid JSON: %w", err)
	}
	return p, nil
}

// ExtractJSON returns the JSON object embedded in raw. Backends wrap their
// answer in prose more often than they should, and a stray fence or sentence
// should not cost us the plan.
func ExtractJSON(raw string) (string, bool) {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return "", false
	}
	return raw[start : end+1], true
}

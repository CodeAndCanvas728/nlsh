// Package backend talks to the model that turns a request into a plan.
//
// A backend has to do two things: send the request, and keep enough of the
// conversation that a follow-up makes sense. The second part is easy to get
// wrong: if the assistant's own answers are not recorded, the model never
// sees what it proposed last time and cannot react to it.
package backend

import (
	"context"
	"strconv"

	"github.com/CodeAndCanvas728/nlsh/internal/plan"
)

// Planner turns a request into a plan.
type Planner interface {
	// Plan returns a plan for the request.
	Plan(ctx context.Context, request string) (plan.Plan, error)

	// Regenerate returns a corrected plan after the previous one was
	// rejected. The feedback describes what was wrong with it.
	Regenerate(ctx context.Context, feedback string) (plan.Plan, error)
}

// Error describes a failure to produce a plan.
type Error struct {
	// Op is what was being attempted, for example "chat completion".
	Op string
	// Status is the HTTP status code, or 0 when the failure was not an
	// HTTP one.
	Status int
	// Body is the response body, truncated to a readable length.
	Body string
	// Err is the underlying error, if any.
	Err error
}

func (e *Error) Error() string {
	switch {
	case e.Status != 0:
		return e.Op + ": HTTP " + strconv.Itoa(e.Status) + ": " + e.Body
	case e.Err != nil:
		return e.Op + ": " + e.Err.Error()
	default:
		return e.Op + ": failed"
	}
}

func (e *Error) Unwrap() error { return e.Err }

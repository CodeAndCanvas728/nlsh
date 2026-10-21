// Package app wires the pieces together: plan, validate, approve, run.
//
// The order matters. Validation happens before the plan is shown, and
// approval happens after it. Nothing is executed until the user has seen
// exactly what will run.
package app

import (
	"context"
	"io"
	"strings"

	"github.com/CodeAndCanvas728/nlsh/internal/backend"
	"github.com/CodeAndCanvas728/nlsh/internal/cli"
	"github.com/CodeAndCanvas728/nlsh/internal/guard"
	"github.com/CodeAndCanvas728/nlsh/internal/plan"
	"github.com/CodeAndCanvas728/nlsh/internal/platform"
	"github.com/CodeAndCanvas728/nlsh/internal/report"
	"github.com/CodeAndCanvas728/nlsh/internal/run"
	"github.com/CodeAndCanvas728/nlsh/internal/validate"
)

// Exit codes. Anything else is the exit status of the last command that ran.
const (
	// ExitOK means the request was handled, or was cancelled.
	ExitOK = 0
	// ExitFailure means no plan could be produced at all.
	ExitFailure = 1
	// ExitInvalid means the plan was rejected: validation issues
	// remained, or a command needed elevated privileges.
	ExitInvalid = 2
)

// App is one run of nlsh.
type App struct {
	// Profile describes the machine nlsh is running on.
	Profile platform.Profile
	// Planner turns requests into plans.
	Planner backend.Planner
	// Checker validates plans.
	Checker *validate.Checker
	// Reporter writes to the terminal.
	Reporter *report.Reporter
	// Options are the parsed command line.
	Options cli.Options
	// Out and Err are where commands are executed.
	Out, Err io.Writer
}

// Run handles the request, or starts the interactive loop when there is
// none.
func (a *App) Run(ctx context.Context) int {
	if a.Options.Request == "" {
		a.interactive(ctx)
		return ExitOK
	}
	return a.Handle(ctx, a.Options.Request)
}

// Handle plans, validates, approves and runs a single request.
func (a *App) Handle(ctx context.Context, request string) int {
	a.Reporter.Progress("thinking...")

	pl, err := a.Planner.Plan(ctx, request)
	if err != nil {
		a.Reporter.Error("the model failed to produce a plan: %v", err)
		return ExitFailure
	}

	issues := a.Checker.Issues(pl, request)
	if len(issues) > 0 {
		a.Reporter.Warn("plan validation failed:")
		for _, issue := range issues {
			a.Reporter.Note("  - %s", issue)
		}
		a.Reporter.Progress("regenerating...")
		fixed, err := a.Planner.Regenerate(ctx, strings.Join(issues, "\n- "))
		if err == nil {
			if len(fixed.Commands) == 0 {
				// The model gave up rather than fixed the plan. The
				// original problems are still the reason there is no
				// usable plan.
				a.Reporter.Warn("the model could not produce a corrected plan")
			} else {
				pl = fixed
				issues = a.Checker.Issues(pl, request)
				if len(issues) == 0 {
					a.Reporter.Progress("regenerated plan passes validation")
				} else {
					a.Reporter.Warn("remaining issues — review carefully:")
					for _, issue := range issues {
						a.Reporter.Note("  - %s", issue)
					}
				}
			}
		}
	}

	if a.Options.PrintMode {
		for _, c := range pl.Commands {
			a.Reporter.Command(c.Command)
		}
		if len(issues) > 0 {
			a.Reporter.Error("validation issues remain; commands may be broken")
			return ExitInvalid
		}
		return ExitOK
	}

	risks := a.risks(pl)
	a.Reporter.Plan(pl, risks)
	if len(pl.Commands) == 0 {
		return ExitOK
	}

	commands := make([]string, 0, len(pl.Commands))
	for _, c := range pl.Commands {
		commands = append(commands, c.Command)
	}

	if a.Options.DryRun {
		a.Reporter.Note("dry run — nothing executed.")
		return ExitOK
	}

	// A command that needs elevated privileges is refused rather than
	// approved: it would ask for a password nobody can type.
	for _, command := range commands {
		if a.Profile.IsSudo(command) {
			a.Reporter.Error("refusing to run a command that needs elevated privileges")
			a.Reporter.Note("Run it yourself:")
			for _, c := range commands {
				a.Reporter.Note("  $ %s", c)
			}
			return ExitInvalid
		}
	}

	approved, edited, status := a.approve(commands, risks)
	if status != ExitOK {
		return status
	}
	if !approved {
		a.Reporter.Note("cancelled.")
		return ExitOK
	}
	if edited != nil {
		commands = edited
	}

	return run.Execute(ctx, a.Profile, commands, a.Out, a.Err)
}

// risks returns the effective risk of every command in the plan.
func (a *App) risks(pl plan.Plan) []guard.Risk {
	risks := make([]guard.Risk, 0, len(pl.Commands))
	for _, c := range pl.Commands {
		risks = append(risks, guard.Classify(a.Profile, c))
	}
	return risks
}

// approve asks the user whether to run the plan.
//
// It returns whether the plan was approved, any commands the user edited,
// and an exit status when the approval could not be completed.
func (a *App) approve(commands []string, risks []guard.Risk) (bool, []string, int) {
	for {
		maxRisk := guard.RiskSafe
		for _, r := range risks {
			maxRisk = guard.Max(maxRisk, r)
		}

		if a.Options.Force {
			return true, nil, ExitOK
		}
		if a.Options.AutoYes && maxRisk != guard.RiskDestructive {
			return true, nil, ExitOK
		}

		if maxRisk == guard.RiskDestructive {
			a.Reporter.Note("one or more commands are destructive.")
			answer, err := a.Reporter.Prompt("  type 'yes' to run them, anything else to cancel: ")
			if err != nil {
				return false, nil, ExitOK
			}
			if strings.EqualFold(strings.TrimSpace(answer), "yes") {
				return true, nil, ExitOK
			}
			return false, nil, ExitOK
		}

		answer, err := a.Reporter.Prompt("\n  Run? [Y=run · n=cancel · e=edit] ")
		if err != nil {
			return false, nil, ExitOK
		}
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "", "y", "yes":
			return true, nil, ExitOK
		case "n", "no":
			return false, nil, ExitOK
		case "e", "edit":
			edited := EditCommands(commands)
			if edited == nil {
				a.Reporter.Note("edit produced no commands — plan unchanged")
				continue
			}
			commands = edited
			risks = a.risksFor(commands)
			a.Reporter.Commands(commands, risks)
			continue
		default:
			a.Reporter.Note("answer y, n, or e.")
			continue
		}
	}
}

// risksFor classifies a bare list of commands.
func (a *App) risksFor(commands []string) []guard.Risk {
	risks := make([]guard.Risk, 0, len(commands))
	for _, c := range commands {
		risks = append(risks, guard.Classify(a.Profile, plan.Command{Command: c, Risk: "safe"}))
	}
	return risks
}

// interactive runs the conversational loop.
func (a *App) interactive(ctx context.Context) {
	a.Reporter.Note("nlsh %s — type a request, 'help', or 'exit'.", cli.Version)
	for {
		answer, err := a.Reporter.Prompt("nlsh > ")
		if err != nil {
			return
		}
		request := strings.TrimSpace(answer)
		if request == "" {
			continue
		}
		switch strings.ToLower(request) {
		case "exit", "quit", "bye":
			return
		case "help":
			a.Reporter.Note("%s", cli.Usage)
			continue
		}
		// The planner keeps the conversation, so a follow-up such as
		// "now make it read-only" has the earlier turns in front of it.
		_ = a.Handle(ctx, request)
	}
}

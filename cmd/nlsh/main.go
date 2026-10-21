// Command nlsh turns a request in plain words into shell commands.
//
// It asks a model for a plan, checks the plan locally, shows it, and runs
// it once the user has approved it.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/CodeAndCanvas728/nlsh/internal/app"
	"github.com/CodeAndCanvas728/nlsh/internal/backend"
	"github.com/CodeAndCanvas728/nlsh/internal/cli"
	"github.com/CodeAndCanvas728/nlsh/internal/platform"
	"github.com/CodeAndCanvas728/nlsh/internal/report"
	"github.com/CodeAndCanvas728/nlsh/internal/validate"
)

func main() {
	os.Exit(run())
}

// run does the work and returns the exit status, so that the deferred
// cleanup in main still happens.
func run() int {
	opts, err := cli.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "nlsh:", err)
		fmt.Fprintln(os.Stderr, "run 'nlsh --help' for usage")
		return app.ExitFailure
	}

	switch {
	case opts.ShowHelp:
		fmt.Print(cli.Usage)
		return app.ExitOK
	case opts.ShowVersion:
		fmt.Printf("nlsh %s\n", cli.Version)
		return app.ExitOK
	case opts.ShowShellInit:
		fmt.Print(shellIntegration())
		return app.ExitOK
	}

	profile := platform.Current()
	reporter := report.New(os.Stdout, os.Stderr, opts.PrintMode)

	planner, err := makePlanner(opts, profile, reporter)
	if err != nil {
		reporter.Error("%v", err)
		return app.ExitFailure
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a := &app.App{
		Profile:  profile,
		Planner:  planner,
		Checker:  validate.New(profile),
		Reporter: reporter,
		Options:  opts,
		Out:      os.Stdout,
		Err:      os.Stderr,
	}
	return a.Run(ctx)
}

// makePlanner builds the planner the options ask for.
//
// There is one backend: an OpenAI-compatible server. apfel serves Apple
// Foundation Models through that API on-device, and it is started on demand,
// so a Mac with Apple Intelligence needs no API key and no cloud.
func makePlanner(opts cli.Options, profile platform.Profile, reporter *report.Reporter) (backend.Planner, error) {
	base := opts.BaseURL
	if base == "" {
		base = os.Getenv("NLSH_BASE_URL")
	}
	key := opts.APIKey
	if key == "" {
		key = os.Getenv("NLSH_API_KEY")
	}
	if key == "" {
		key = os.Getenv("OPENAI_API_KEY")
	}
	model := opts.Model
	if model == "" {
		model = os.Getenv("NLSH_MODEL")
	}

	if base == "" {
		return nil, errors.New("no model backend configured; point nlsh at an OpenAI-compatible server:\n" +
			"  export NLSH_BASE_URL=http://localhost:11434/v1\n" +
			"  export NLSH_MODEL=apple-foundationmodel\n" +
			"or install apfel (brew install apfel) and nlsh will start it for you")
	}
	if model == "" {
		return nil, errors.New("no model configured; set NLSH_MODEL or pass --model")
	}

	launcher := &backend.ApfelLauncher{
		BaseURL: base,
		Log:     func(msg string) { reporter.Progress("%s", msg) },
	}
	if err := launcher.Ensure(context.Background()); err != nil {
		return nil, err
	}

	return backend.NewOpenAI(backend.Config{
		BaseURL: base,
		APIKey:  key,
		Model:   model,
		System:  app.SystemPrompt(profile),
	}), nil
}

// Package cli parses the command line.
//
// The rules are deliberately strict about ordering. Options come before the
// request, because a request is free text: once the first word of the
// request has been seen, everything after it belongs to the request. An
// option in that position is almost always a mistake, and silently sending
// it to the model as part of the request is the worst possible outcome, so
// it is reported instead.
package cli

import (
	"fmt"
	"strings"
	"time"
)

// Options are the parsed command line.
type Options struct {
	// AutoYes approves the plan without asking, unless it is destructive.
	AutoYes bool
	// Force approves everything, including destructive commands.
	Force bool
	// DryRun shows the plan without running it.
	DryRun bool
	// PrintMode writes only the commands to stdout.
	PrintMode bool
	// ShowVersion asks for the version.
	ShowVersion bool
	// ShowHelp asks for the usage text.
	ShowHelp bool
	// ShowShellInit asks for the shell integration script.
	ShowShellInit bool
	// BaseURL is the OpenAI-compatible endpoint to use.
	BaseURL string
	// APIKey is sent as a bearer token.
	APIKey string
	// Model is the model to ask for.
	Model string
	// Timeout bounds a single command. Zero means no limit.
	Timeout time.Duration
	// Request is the request in plain words.
	Request string
}

// Options known by name, so that an option that appears after the request
// can be recognised and reported rather than quietly swallowed.
var knownFlags = map[string]bool{
	"-y": true, "--yes": true,
	"-f": true, "--force": true,
	"-n": true, "--dry-run": true,
	"-p": true, "--print": true,
	"-h": true, "--help": true,
	"-v": true, "--version": true,
	"--base-url": true,
	"--api-key":  true,
	"--model":    true,
	"--timeout":  true,
	"--zsh-init": true,
}

// Parse reads the command line.
//
// It returns a usage error for an unknown option, a missing value or an
// option that appears after the request.
func Parse(args []string) (Options, error) {
	var o Options
	var request []string
	inRequest := false
	explicit := false

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if !inRequest && arg == "--" {
			inRequest = true
			explicit = true
			continue
		}

		if !inRequest && strings.HasPrefix(arg, "-") && arg != "-" {
			value, err := takeValue(args, &i)
			if err != nil {
				return o, err
			}
			switch arg {
			case "-y", "--yes":
				o.AutoYes = true
			case "-f", "--force":
				o.Force = true
			case "-n", "--dry-run":
				o.DryRun = true
			case "-p", "--print":
				o.PrintMode = true
			case "-h", "--help":
				o.ShowHelp = true
			case "-v", "--version":
				o.ShowVersion = true
			case "--zsh-init":
				o.ShowShellInit = true
			case "--base-url":
				o.BaseURL = value
			case "--api-key":
				o.APIKey = value
			case "--model":
				o.Model = value
			case "--timeout":
				d, err := time.ParseDuration(value)
				if err != nil {
					return o, fmt.Errorf("--timeout expects a duration such as 30s or 5m, not %q", value)
				}
				if d < 0 {
					return o, fmt.Errorf("--timeout cannot be negative, not %q", value)
				}
				o.Timeout = d
			default:
				return o, fmt.Errorf("unknown option %s", arg)
			}
			continue
		}

		// The first word of the request starts request mode.
		if !inRequest {
			inRequest = true
		} else if knownFlags[arg] && !explicit {
			return o, fmt.Errorf("option %s appears after the request; put all options before the request, "+
				"or use -- to mark the end of the options", arg)
		}
		request = append(request, arg)
	}

	o.Request = strings.Join(request, " ")
	return o, nil
}

// takeValue reads the value of an option that needs one and advances past
// it.
func takeValue(args []string, i *int) (string, error) {
	name := args[*i]
	switch name {
	case "--base-url", "--api-key", "--model", "--timeout":
	default:
		// A flag without a value.
		return "", nil
	}
	if *i+1 >= len(args) {
		return "", fmt.Errorf("option %s expects a value", name)
	}
	*i++
	return args[*i], nil
}

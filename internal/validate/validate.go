// Package validate checks a plan before nlsh shows it to the user.
//
// The checks are heuristics. They gate the obvious breakage — a binary that
// does not exist, a flag the tool does not have, quotes that do not balance,
// a path the user mentioned that the plan ignores — not correctness. A plan
// that passes every check can still be wrong.
package validate

import (
	"context"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/CodeAndCanvas728/nlsh/internal/plan"
	"github.com/CodeAndCanvas728/nlsh/internal/platform"
	"github.com/CodeAndCanvas728/nlsh/internal/shell"
)

// helpTimeout bounds how long nlsh waits for a tool's --help output.
const helpTimeout = 3 * time.Second

// Checker validates plans against a machine profile. It caches help output
// per binary, because checking a plan with five commands should not run
// five --help processes.
type Checker struct {
	profile platform.Profile

	mu        sync.Mutex
	helpCache map[string]string
}

// New returns a Checker for the given platform.
func New(p platform.Profile) *Checker {
	return &Checker{profile: p, helpCache: map[string]string{}}
}

// Issues returns a description of everything that looks wrong with the plan.
// An empty slice means the plan passed every check.
//
// The request is needed as well as the plan: it is what the plan is
// supposed to be about, and paths mentioned there but missing from the plan
// are a common way for a plan to go wrong.
func (c *Checker) Issues(pl plan.Plan, request string) []string {
	if len(pl.Commands) == 0 {
		return nil
	}

	var issues []string
	for i, cmd := range pl.Commands {
		line := strings.TrimSpace(cmd.Command)
		at := "command " + itoa(i+1) + ": "
		switch {
		case line == "":
			issues = append(issues, at+"empty command in plan")
			continue
		case !quotesBalanced(line):
			issues = append(issues, at+"unbalanced quotes")
			continue
		}

		bin, ok := shell.FirstBinary(line)
		if !ok {
			issues = append(issues, at+"could not identify a command to run")
			continue
		}
		if !c.commandExists(bin) {
			issues = append(issues, at+"'"+bin+"' not found in PATH and not a shell builtin")
			continue
		}
		for _, flag := range longFlags(line) {
			if !c.helpKnows(bin, line, flag) {
				issues = append(issues, at+"flag "+flag+" does not appear in '"+bin+" --help'")
			}
		}
	}

	for _, path := range PathsMentioned(request) {
		if !planUsesPath(pl, path) {
			issues = append(issues, "the request mentions '"+path+"' but no command uses it")
		}
	}
	return issues
}

// planUsesPath reports whether any command in the plan mentions the path.
func planUsesPath(pl plan.Plan, path string) bool {
	for _, c := range pl.Commands {
		if strings.Contains(c.Command, path) {
			return true
		}
	}
	return false
}

// builtins are the shell builtins that need no binary on disk.
var builtins = map[string]bool{
	"cd": true, "echo": true, "type": true, "source": true, "export": true,
	"alias": true, "unalias": true, "true": true, "false": true, "test": true,
	"[": true, "umask": true, "pushd": true, "popd": true, "dirs": true,
	"jobs": true, "fg": true, "bg": true, "unset": true, "set": true,
	"shift": true, "read": true, "wait": true, "eval": true, "local": true,
	"hash": true, "command": true, "builtin": true, "printf": true,
	"which": true, "time": true, "kill": true,
}

// commandExists reports whether the binary can be run.
func (c *Checker) commandExists(bin string) bool {
	if strings.HasPrefix(bin, "/") || strings.HasPrefix(bin, "~") {
		_, err := os.Stat(c.profile.ExpandHome(bin))
		return err == nil
	}
	if builtins[bin] {
		return true
	}
	_, err := exec.LookPath(bin)
	return err == nil
}

// helpKnows reports whether the tool accepts the flag.
//
// For tools with subcommands — git, docker, npm — the flag belongs to the
// subcommand, not to the top-level tool, so the subcommand's own help is
// consulted first.
func (c *Checker) helpKnows(bin, line, flag string) bool {
	help := c.helpFor(bin, line)
	if len(help) < 20 {
		// No usable help output: nothing to check against, so do not
		// invent an issue.
		return true
	}
	return strings.Contains(help, flag)
}

// helpFor returns the help text for the command, using the cache when
// possible.
func (c *Checker) helpFor(bin, line string) string {
	key := bin
	if sub, ok := shell.FirstSubcommand(line); ok {
		key = bin + " " + sub
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if cached, ok := c.helpCache[key]; ok {
		return cached
	}
	help := c.captureHelp(bin, line)
	c.helpCache[key] = help
	return help
}

// captureHelp runs --help for the command and returns what it printed.
//
// The subcommand's help is preferred because that is where the flags of a
// command like "git push" are documented. The top-level help is the
// fallback.
func (c *Checker) captureHelp(bin, line string) string {
	if sub, ok := shell.FirstSubcommand(line); ok {
		if help := c.runHelp(bin, sub); len(help) >= 20 {
			return help
		}
	}
	return c.runHelp(bin)
}

// runHelp runs "<bin> [sub] --help" and returns its combined output.
func (c *Checker) runHelp(args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), helpTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, args[0], append(args[1:], "--help")...)
	cmd.Env = append(os.Environ(),
		"PAGER=cat",
		"MANPAGER=cat",
		"GIT_PAGER=cat",
		"LC_ALL=C",
	)
	out, err := cmd.CombinedOutput()
	if err != nil && len(out) == 0 {
		return ""
	}
	return string(out)
}

// quotesBalanced reports whether every quote in the command is closed.
func quotesBalanced(s string) bool {
	var single, double, escaped bool
	for _, r := range s {
		switch {
		case escaped:
			escaped = false
		case r == '\\' && !single:
			escaped = true
		case r == '\'' && !double:
			single = !single
		case r == '"' && !single:
			double = !double
		}
	}
	return !single && !double && !escaped
}

// longFlagPattern matches a long option such as --recursive.
var longFlagPattern = regexp.MustCompile(`(^|\s)(--[A-Za-z0-9][A-Za-z0-9_-]*)`)

// longFlags returns the long options used in a command line.
func longFlags(line string) []string {
	var flags []string
	for _, m := range longFlagPattern.FindAllStringSubmatch(line, -1) {
		flags = append(flags, m[2])
	}
	return flags
}

// Package platform describes the machine nlsh is running on.
//
// Everything that differs between operating systems lives behind the Profile
// interface: which shell runs the commands, which coreutils dialect the
// planner should target, what counts as a destructive command, and how a
// home directory is spelled. The rest of nlsh is written against this
// interface and never branches on runtime.GOOS directly.
package platform

import (
	"os"
	"regexp"
	"runtime"
	"strings"
)

// Profile describes one operating system.
type Profile interface {
	// Name is the short identifier used in prompts and diagnostics,
	// for example "darwin" or "windows".
	Name() string

	// Shell returns the argv prefix used to run a command line: the
	// program and the flag that takes the command, for example
	// {"/bin/zsh", "-c"}.
	Shell() []string

	// CoreutilsRule is the instruction given to the planner about which
	// flavour of command-line tools it should target.
	CoreutilsRule() string

	// IsDestructive reports whether a command deletes, overwrites,
	// resets or otherwise destroys data.
	IsDestructive(command string) bool

	// IsSudo reports whether a command needs elevated privileges.
	// Such commands are refused outright rather than approved.
	IsSudo(command string) bool

	// ExpandHome expands a leading ~ to the user's home directory.
	ExpandHome(path string) string
}

// Current returns the profile for the operating system nlsh runs on.
func Current() Profile {
	return forOS(runtime.GOOS)
}

// forOS is a variable so tests can exercise every profile on any host.
var forOS = func(os string) Profile {
	switch os {
	case "darwin":
		return Darwin
	case "windows":
		return Windows
	default:
		return Linux
	}
}

// unixProfile is the shared behaviour of the macOS and Linux profiles. They
// differ in the shell, the coreutils dialect and a handful of destructive
// commands, not in how a home directory is spelled.
type unixProfile struct {
	name        string
	shell       []string
	coreutils   string
	destructive []*regexp.Regexp
}

func (p *unixProfile) Name() string          { return p.name }
func (p *unixProfile) CoreutilsRule() string { return p.coreutils }
func (p *unixProfile) Shell() []string       { return p.shell }

func (p *unixProfile) IsDestructive(cmd string) bool {
	return matchesAny(p.destructive, cmd) || rmIsDestructive(cmd)
}

func (p *unixProfile) IsSudo(cmd string) bool { return unixIsSudo(cmd) }

// ExpandHome expands a leading ~ using $HOME, falling back to the user's
// home directory from the environment.
func (p *unixProfile) ExpandHome(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path
	}
	home := homeDir()
	if home == "" {
		return path
	}
	if path == "~" {
		return home
	}
	return home + path[1:]
}

func matchesAny(pats []*regexp.Regexp, s string) bool {
	for _, re := range pats {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}

// Package shell parses command lines.
//
// Both the validator (which needs to know which binary a command runs) and
// the platform profiles (which need to know whether a command deletes
// anything or escalates privileges) work on the same question, so the
// tokenizer lives here rather than in either of them.
package shell

import (
	"path/filepath"
	"regexp"
	"strings"
)

// Segment is one command within a command line.
type Segment struct {
	// Program is the executable the segment runs. It may be empty when
	// the segment contains no executable.
	Program string
	// Wrappers are the wrapper commands that appeared before Program,
	// for example "sudo" in "sudo rm -rf /".
	Wrappers []string
	// Args are the arguments passed to Program.
	Args []string
}

// Base returns the program name without its directory.
func (s Segment) Base() string { return filepath.Base(s.Program) }

var assignmentPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=.*$`)

// Wrappers take another command as their argument.
var Wrappers = map[string]bool{
	"env": true, "command": true, "builtin": true, "nice": true,
	"nohup": true, "time": true, "xargs": true,
}

// PrivilegeTools are wrappers that escalate privileges.
var PrivilegeTools = map[string]bool{
	"sudo": true, "doas": true, "pkexec": true,
}

// wrapperValueFlags are wrapper options whose value is a separate token,
// so that "nice -n 10 ls" still runs ls and not 10.
var wrapperValueFlags = map[string]map[string]bool{
	"nice": {"-n": true, "--adjustment": true},
	"sudo": {"-u": true, "--user": true, "-g": true, "--group": true},
	"doas": {"-u": true, "--user": true},
}

// IsAssignment reports whether a token looks like FOO=bar.
func IsAssignment(token string) bool { return assignmentPattern.MatchString(token) }

// Split breaks a command line into the individual commands it runs.
// Single and double quotes are honoured, so a quoted word is never mistaken
// for a command.
func Split(line string) []Segment {
	var out []Segment
	var tokens []string
	var current strings.Builder
	inToken := false
	quote := byte(0)
	escaped := false

	flushToken := func() {
		if inToken {
			tokens = append(tokens, current.String())
			current.Reset()
			inToken = false
		}
	}
	flushSegment := func() {
		flushToken()
		if len(tokens) > 0 {
			out = append(out, analyze(tokens))
			tokens = nil
		}
	}

	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case escaped:
			current.WriteByte(c)
			inToken = true
			escaped = false
		case c == '\\' && quote != '\'':
			escaped = true
			inToken = true
		case quote != 0:
			if c == quote {
				quote = 0
			} else {
				current.WriteByte(c)
			}
			inToken = true
		case c == '\'' || c == '"':
			quote = c
			inToken = true
		case c == ';' || c == '\n':
			flushSegment()
		case c == '&' || c == '|':
			flushSegment()
		case c == ' ' || c == '\t':
			flushToken()
		default:
			current.WriteByte(c)
			inToken = true
		}
	}
	flushSegment()
	return out
}

// analyze works out which program a token list runs.
func analyze(tokens []string) Segment {
	seg := Segment{}
	i := 0
	for i < len(tokens) && IsAssignment(tokens[i]) {
		i++
	}
	for i < len(tokens) {
		base := filepath.Base(tokens[i])
		if Wrappers[base] || PrivilegeTools[base] {
			seg.Wrappers = append(seg.Wrappers, base)
			i++
			// Flags and assignments belonging to the wrapper, for
			// example `env FOO=1 ls` or `xargs -0 rm`.
			for i < len(tokens) && (strings.HasPrefix(tokens[i], "-") || IsAssignment(tokens[i])) {
				flag := tokens[i]
				i++
				// Some wrapper options take a value of their own,
				// for example `nice -n 10 ls`.
				if i < len(tokens) && wrapperValueFlags[base][flag] {
					i++
				}
			}
			continue
		}
		break
	}
	if i < len(tokens) {
		seg.Program = tokens[i]
		seg.Args = tokens[i+1:]
	}
	return seg
}

// IsPOSIX reports whether a shell is one that understands POSIX shell
// syntax.
//
// It matters for the tests that run commands: the Windows profile runs
// PowerShell, where POSIX commands and flags do not exist.
func IsPOSIX(path string) bool {
	switch filepath.Base(path) {
	case "sh", "bash", "zsh", "dash", "ksh", "ash", "busybox":
		return true
	}
	return false
}

// FirstBinary returns the program the command line runs, skipping
// assignments and wrappers. It reports false when no program could be
// identified.
func FirstBinary(line string) (string, bool) {
	for _, seg := range Split(line) {
		if seg.Program != "" {
			return seg.Program, true
		}
	}
	return "", false
}

// FirstSubcommand returns the first positional argument of the command,
// which for tools like git or docker names the subcommand to run.
//
// Arguments that look like paths are skipped: in "git -C /tmp push" the
// path is an option's value, not the subcommand.
func FirstSubcommand(line string) (string, bool) {
	for _, seg := range Split(line) {
		for _, arg := range seg.Args {
			if strings.HasPrefix(arg, "-") {
				continue
			}
			if strings.ContainsAny(arg, "/\\") {
				continue
			}
			return arg, true
		}
	}
	return "", false
}

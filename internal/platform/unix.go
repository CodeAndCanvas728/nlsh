package platform

import (
	"regexp"
	"strings"

	"github.com/jonah/nlsh/internal/shell"
)

// unixDestructivePatterns are the destructive commands nlsh recognises on
// macOS and Linux.
//
// rm is not here: it needs to know which flags were actually passed, so it
// is handled by rmIsDestructive below.
func unixDestructivePatterns() []*regexp.Regexp {
	return []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bmkfs(\.[a-z0-9]+)?\b`),
		regexp.MustCompile(`(?i)\bdd\b[^|;&]*\bof=`),
		regexp.MustCompile(`(?i)\bdiskutil\s+(erase|eraseVolume|apfs\s+delete|zeroDisk|reformat)`),
		regexp.MustCompile(`(?i)\bgit\s+reset\s+--hard\b`),
		regexp.MustCompile(`(?i)\bgit\s+clean\s+-[a-z]*f`),
		regexp.MustCompile(`(?i)\bgit\s+push\b[^|;&]*(--force\b|--force-with-lease|-f\b)`),
		regexp.MustCompile(`(?i)\bchmod\s+-R\s+777\b`),
		regexp.MustCompile(`(?i)\bshred\b`),
		regexp.MustCompile(`(?i)\btruncate\s+-s\s*0\b`),
		regexp.MustCompile(`(?i)\b(curl|wget)\b[^|;&]*\|\s*(sudo\s+)?(ba|z|d)?sh\b`),
		regexp.MustCompile(`(?i)\bdefaults\s+delete\b`),
	}
}

// rmIsDestructive reports whether the command line deletes with rm.
//
// rm counts as destructive when it is recursive, when it forces, or when it
// is given something to delete. Bare `rm` and `rm --help` do not.
func rmIsDestructive(line string) bool {
	for _, seg := range shell.Split(line) {
		if seg.Base() != "rm" {
			continue
		}
		for _, arg := range seg.Args {
			if strings.HasPrefix(arg, "-") {
				if flagHas(arg, "r") || flagHas(arg, "recursive") {
					return true
				}
				if flagHas(arg, "f") || flagHas(arg, "force") {
					return true
				}
				continue
			}
			// A non-flag argument is a file to delete.
			return true
		}
	}
	return false
}

// flagHas reports whether a command-line flag contains the letter or the
// long name. It looks at the flag's own letters, not at the whole token, so
// --preserve-root is not mistaken for -r.
func flagHas(flag, letter string) bool {
	name := strings.TrimLeft(flag, "-")
	if name == "" {
		return false
	}
	if idx := strings.Index(name, "="); idx >= 0 {
		name = name[:idx]
	}
	if strings.HasPrefix(flag, "--") {
		return name == letter
	}
	return strings.Contains(name, letter)
}

// unixIsSudo reports whether any command in the line escalates privileges.
func unixIsSudo(line string) bool {
	for _, seg := range shell.Split(line) {
		for _, w := range seg.Wrappers {
			if shell.PrivilegeTools[w] {
				return true
			}
		}
		if shell.PrivilegeTools[seg.Base()] {
			return true
		}
	}
	return false
}

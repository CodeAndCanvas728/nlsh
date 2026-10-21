package platform

import (
	"regexp"
	"strings"

	"github.com/jonah/nlsh/internal/shell"
)

// Windows describes a Windows host driven through PowerShell.
//
// Windows support is command-line only: there is no shell integration, and
// the planner is asked for PowerShell rather than a POSIX shell.
var Windows Profile = &windowsProfile{}

type windowsProfile struct{}

func (p *windowsProfile) Name() string { return "windows" }

func (p *windowsProfile) Shell() []string {
	return []string{"powershell.exe", "-NoProfile", "-NonInteractive", "-Command"}
}

func (p *windowsProfile) CoreutilsRule() string {
	return "- Target Windows with PowerShell. The shell is powershell.exe, run with -NoProfile -NonInteractive.\n" +
		"- Use PowerShell cmdlets (Get-ChildItem, Select-String, Copy-Item) rather than POSIX tools.\n" +
		"- Quote paths with single quotes; do not use backslashes as escape characters."
}

// windowsDestructivePatterns are the destructive commands nlsh recognises
// on Windows.
var windowsDestructivePatterns = []*regexp.Regexp{
	// Remove-Item -Recurse -Force, and the same with -Confirm:$false.
	regexp.MustCompile(`(?i)\bremove-item\b[^|;&]*\s-(?:r|recurse|rec)\b`),
	regexp.MustCompile(`(?i)\bremove-item\b[^|;&]*\s-force\b`),
	// cmd builtins: del /s, rd /s /q, erase.
	regexp.MustCompile(`(?i)\b(del|erase)\b[^|;&]*\s/s\b`),
	regexp.MustCompile(`(?i)\b(rd|rmdir)\b[^|;&]*\s/s\b`),
	// Disk and volume tools.
	regexp.MustCompile(`(?i)\bformat\s+[a-z]:`),
	regexp.MustCompile(`(?i)\bdiskpart\b`),
	regexp.MustCompile(`(?i)\bclear-content\b`),
	// Registry.
	regexp.MustCompile(`(?i)\breg\s+delete\b`),
	// Overwriting with an empty file.
	regexp.MustCompile(`(?i)\bset-content\b[^|;&]*\s-value\s+['"]['"]`),
	// Secure erase.
	regexp.MustCompile(`(?i)\bcipher\s+/w\b`),
}

// windowsPrivilegeTools are wrappers that elevate privileges.
var windowsPrivilegeTools = map[string]bool{
	"sudo": true, "gsudo": true,
}

func (p *windowsProfile) IsDestructive(cmd string) bool {
	if matchesAny(windowsDestructivePatterns, cmd) {
		return true
	}
	// Remove-Item is a delete whether or not -Recurse is present.
	for _, seg := range shell.Split(cmd) {
		if strings.EqualFold(seg.Base(), "remove-item") {
			return true
		}
	}
	return false
}

func (p *windowsProfile) IsSudo(cmd string) bool {
	for _, seg := range shell.Split(cmd) {
		for _, w := range seg.Wrappers {
			if windowsPrivilegeTools[w] {
				return true
			}
		}
		if windowsPrivilegeTools[strings.ToLower(seg.Base())] {
			return true
		}
		// Start-Process -Verb RunAs elevates without a wrapper.
		if strings.EqualFold(seg.Base(), "start-process") {
			for _, arg := range seg.Args {
				if strings.EqualFold(arg, "runas") {
					return true
				}
			}
		}
	}
	return false
}

// ExpandHome expands a leading ~ to the user's profile directory.
func (p *windowsProfile) ExpandHome(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") && !strings.HasPrefix(path, `~\`) {
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

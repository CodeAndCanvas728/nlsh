package app

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/CodeAndCanvas728/nlsh/internal/platform"
)

// SystemPrompt returns the instruction sent to the planner.
//
// It says which machine the commands will run on, because the right command
// for macOS and for Linux are not the same, and it tells the planner when
// the request cannot be carried out unattended.
func SystemPrompt(p platform.Profile) string {
	return fmt.Sprintf(`You translate natural-language requests into a minimal, ordered list of shell commands for the user's machine, to be run unattended.

Rules:
%s
- Use as few commands as possible; keep each one readable and quote file paths.
- When the user names a file or folder without a path, treat it as relative to the current working directory.
- Commands must run without interaction: no sudo, no interactive programs (vim, top, less, man pagers), nothing that waits for input. Use non-interactive flags (-y, -f, -q) where tools support them.
- Do only what was asked: no extra installs, no verbose commentary, no echo banners.
- risk: "safe" for read-only commands (ls, cat, grep, find, du, git status, ...); "careful" for anything that creates, moves, or modifies files or state; "destructive" for anything that deletes, overwrites, resets, or force-pushes.
- Self-check before answering: every command must exist on the target system, flags must be valid, and quotes must balance. If unsure about a flag, omit it.
- If the request is impossible unattended, needs elevation, or lacks required information you cannot reasonably assume, return an empty commands array and explain why in caveat.
- If you must assume something (a path, a filename, an interpretation), take the most literal reading of the user's words and state the assumption in caveat.
- explanation: one short sentence, written to the user, saying what will happen.`, p.CoreutilsRule())
}

// UserPrompt returns the per-request part of the prompt.
//
// It carries the facts the model cannot know: where the user is, what they
// are called, which shell will run the commands, and today's date. The date
// matters because the on-device model has no clock and will otherwise
// invent one.
func UserPrompt(request string) string {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "unknown"
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "unknown"
	}
	user := os.Getenv("USER")
	if user == "" {
		user = "unknown"
	}
	return fmt.Sprintf(`Current directory: %s
System: %s
User: %s, shell: %s
Today: %s

Request: %s`, cwd, platformDescription(), user, shell, time.Now().Format("Monday 2 January 2006"), request)
}

// platformDescription names the operating system in a way the model can act
// on.
func platformDescription() string {
	switch runtime.GOOS {
	case "darwin":
		if v := macOSVersion(); v != "" {
			return "macOS " + v
		}
		return "macOS"
	case "windows":
		return "Windows"
	default:
		return "Linux"
	}
}

// macOSVersion reads the product version from the system plist.
func macOSVersion() string {
	data, err := os.ReadFile("/System/Library/CoreServices/SystemVersion.plist")
	if err != nil {
		return ""
	}
	text := string(data)
	const key = "<key>ProductVersion</key>"
	idx := strings.Index(text, key)
	if idx < 0 {
		return ""
	}
	rest := text[idx+len(key):]
	open := strings.Index(rest, "<string>")
	closing := strings.Index(rest, "</string>")
	if open < 0 || closing < 0 || closing < open {
		return ""
	}
	return rest[open+len("<string>") : closing]
}

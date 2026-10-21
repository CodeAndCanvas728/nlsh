// Package report renders everything nlsh shows the user.
//
// The rule that matters: in machine mode the only thing on stdout is the
// plan's commands, one per line. Progress, warnings and errors always go to
// stderr, so that a shell integration can capture the commands without
// picking up anything else.
package report

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/jonah/nlsh/internal/guard"
	"github.com/jonah/nlsh/internal/plan"
)

// Reporter writes to the terminal.
type Reporter struct {
	stdout  io.Writer
	stderr  io.Writer
	machine bool
	style   Style
	input   *bufio.Reader
}

// New returns a Reporter. In machine mode the plan display is suppressed
// and only the commands are written to stdout.
func New(stdout, stderr io.Writer, machine bool) *Reporter {
	return &Reporter{
		stdout:  stdout,
		stderr:  stderr,
		machine: machine,
		style:   Style{Enabled: isTerminal(stdout) && os.Getenv("NO_COLOR") == ""},
		input:   bufio.NewReader(os.Stdin),
	}
}

// Progress writes a progress note. It always goes to stderr so that it can
// never contaminate machine-readable output.
func (r *Reporter) Progress(format string, args ...any) {
	fmt.Fprintf(r.stderr, r.style.Dim("  "+format)+"\n", args...)
}

// Warn writes a warning. It always goes to stderr.
func (r *Reporter) Warn(format string, args ...any) {
	fmt.Fprintf(r.stderr, r.style.Yellow("  ! "+format)+"\n", args...)
}

// Error writes an error message. It always goes to stderr.
func (r *Reporter) Error(format string, args ...any) {
	fmt.Fprintf(r.stderr, r.style.Red("  "+format)+"\n", args...)
}

// Note writes an ordinary message for the user. In machine mode it goes to
// stderr so that stdout stays machine-readable.
func (r *Reporter) Note(format string, args ...any) {
	w := r.stdout
	if r.machine {
		w = r.stderr
	}
	fmt.Fprintf(w, format+"\n", args...)
}

// Command writes one generated command. In machine mode this is the only
// thing written to stdout.
func (r *Reporter) Command(command string) {
	fmt.Fprintln(r.stdout, command)
}

// Plan displays the plan and the risk of each command.
//
// In machine mode nothing is displayed: the commands are written separately
// so that they can be captured.
func (r *Reporter) Plan(p plan.Plan, risks []guard.Risk) {
	if r.machine {
		return
	}
	fmt.Fprintln(r.stdout)
	fmt.Fprintln(r.stdout, r.style.Bold("  "+p.Explanation))
	if p.Caveat != "" {
		fmt.Fprintln(r.stdout, r.style.Yellow("  ! "+p.Caveat))
	}
	if len(p.Commands) == 0 {
		fmt.Fprintln(r.stdout, r.style.Dim("  (no commands — nothing to run)"))
		return
	}
	for i, c := range p.Commands {
		risk := guard.RiskSafe
		if i < len(risks) {
			risk = risks[i]
		}
		fmt.Fprintf(r.stdout, "  %d. %s %s  %s\n", i+1, r.style.Cyan("$"), c.Command, r.style.Badge(risk))
	}
}

// Commands displays a list of commands with their risk.
func (r *Reporter) Commands(commands []string, risks []guard.Risk) {
	if r.machine {
		return
	}
	for i, command := range commands {
		risk := guard.RiskSafe
		if i < len(risks) {
			risk = risks[i]
		}
		fmt.Fprintf(r.stdout, "  %d. %s %s  %s\n", i+1, r.style.Cyan("$"), command, r.style.Badge(risk))
	}
}

// Prompt asks the user a question and reads the answer. Prompts are only
// used in human mode.
func (r *Reporter) Prompt(question string) (string, error) {
	fmt.Fprint(r.stdout, question)
	return r.input.ReadString('\n')
}

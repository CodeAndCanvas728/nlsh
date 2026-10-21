// Package run executes the commands of a plan.
//
// Commands run one at a time, in order, through the platform's shell. The
// first failure stops the run and its exit status becomes nlsh's own, so
// that nlsh can be used inside a script.
package run

import (
	"context"
	"fmt"
	"io"
	"os/exec"

	"github.com/jonah/nlsh/internal/platform"
)

// Execute runs the commands in order and returns the exit status of the
// last one that ran.
//
// A non-zero status stops the run: the commands after it were planned on the
// assumption that the ones before it succeeded.
func Execute(ctx context.Context, p platform.Profile, commands []string, stdout, stderr io.Writer) int {
	for i, command := range commands {
		fmt.Fprintf(stdout, "\n  $ %s\n", command)
		status := runOne(ctx, p, command, stdout, stderr)
		if status != 0 {
			fmt.Fprintf(stderr, "nlsh: command %d exited with status %d; stopping.\n", i+1, status)
			return status
		}
	}
	return 0
}

// runOne runs a single command line through the platform's shell.
func runOne(ctx context.Context, p platform.Profile, command string, stdout, stderr io.Writer) int {
	argv := append(append([]string{}, p.Shell()...), command)

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	configureProcess(cmd)

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(stderr, "nlsh: could not launch %s: %v\n", argv[0], err)
		return 127
	}

	stop := forwardSignals(cmd)
	err := cmd.Wait()
	stop()

	if err == nil {
		return 0
	}
	return exitStatus(err)
}

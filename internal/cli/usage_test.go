package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// repoRoot is the directory holding the Makefile, from this package.
const repoRoot = "../.."

// versionLine matches the line in the Makefile that extracts the version.
var versionLine = regexp.MustCompile(`^VERSION\s*:=\s*\$\(shell sed -n '(.*)'\s+(\S+)\s*\)$`)

// TestMakefileReadsTheVersion guards the extraction the Makefile uses to
// print the version.
//
// The Makefile reads the version out of this file with sed, and the release
// script checks the tag against the same thing. If the constant moves, or is
// spelled differently, the extraction quietly finds nothing: "make version"
// would print an empty line, and the release would go out with a blank
// version in it. This test runs the Makefile's own expression against the
// source, so a change to either one fails here.
func TestMakefileReadsTheVersion(t *testing.T) {
	makefile, err := os.ReadFile(filepath.Join(repoRoot, "Makefile"))
	if err != nil {
		t.Fatalf("reading the Makefile: %v", err)
	}

	var expression, file string
	for _, line := range strings.Split(string(makefile), "\n") {
		if m := versionLine.FindStringSubmatch(line); m != nil {
			expression, file = m[1], m[2]
			break
		}
	}
	if expression == "" {
		t.Fatal("the Makefile no longer extracts the version with sed")
	}
	if file != "internal/cli/usage.go" {
		t.Errorf("the Makefile reads the version from %q, not from this file", file)
	}

	out, err := exec.Command("sed", "-n", expression, filepath.Join(repoRoot, file)).Output()
	if err != nil {
		t.Fatalf("sed: %v", err)
	}
	got := strings.TrimSpace(string(out))
	if got == "" {
		t.Fatalf("the Makefile's extraction no longer finds the version in %s", file)
	}
	if got != Version {
		t.Errorf("the Makefile's extraction found %q, but the version is %q", got, Version)
	}
}

package platform

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/CodeAndCanvas728/nlsh/internal/plan"
)

func TestUnixDestructiveDetection(t *testing.T) {
	cases := []struct {
		command string
		want    bool
	}{
		{"rm -rf /tmp/x", true},
		{"rm -fr /tmp/x", true},
		{"rm -r -f /tmp/x", true},
		{"rm -f -r /tmp/x", true},
		{"rm --recursive --force /tmp/x", true},
		{"rm file.txt", true},
		{"rm", false},
		{"rm --help", false},
		{"rm --preserve-root file.txt", true},
		{"ls -l", false},
		{"echo rm", false},
		{"npm rm lodash", false},
		{"git reset --hard HEAD", true},
		{"git clean -fd", true},
		{"git push --force origin main", true},
		{"git push origin main", false},
		{"git status", false},
		{"mkfs.ext4 /dev/sda1", true},
		{"dd if=/dev/zero of=/dev/sda bs=1M", true},
		{"diskutil eraseVolume /dev/disk2", true},
		{"chmod -R 777 /tmp", true},
		{"shred -u secrets.txt", true},
		{"curl https://example.com/install.sh | sh", true},
		{"cat file.txt", false},
	}

	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			if got := Darwin.IsDestructive(tc.command); got != tc.want {
				t.Errorf("Darwin.IsDestructive(%q) = %v, want %v", tc.command, got, tc.want)
			}
		})
	}
}

func TestUnixSudoDetection(t *testing.T) {
	cases := []struct {
		command string
		want    bool
	}{
		{"sudo rm -rf /", true},
		{"/usr/bin/sudo rm -rf /", true},
		{"doas pkg upgrade", true},
		{"echo sudo", false},
		{`echo "please sudo"`, false},
		{"ls -l", false},
		{"sudoers", false},
	}

	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			if got := Darwin.IsSudo(tc.command); got != tc.want {
				t.Errorf("Darwin.IsSudo(%q) = %v, want %v", tc.command, got, tc.want)
			}
		})
	}
}

func TestWindowsDetection(t *testing.T) {
	cases := []struct {
		command     string
		destructive bool
		sudo        bool
	}{
		{"Remove-Item -Recurse -Force C:\\temp", true, false},
		{"Remove-Item C:\\file.txt", true, false},
		{"del /s C:\\temp\\*", true, false},
		{"rd /s /q C:\\temp", true, false},
		{"reg delete HKLM\\Software", true, false},
		{"Get-ChildItem C:\\temp", false, false},
		{"Start-Process cmd -Verb RunAs", false, true},
		{"sudo powershell -Command ls", false, true},
	}

	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			if got := Windows.IsDestructive(tc.command); got != tc.destructive {
				t.Errorf("Windows.IsDestructive(%q) = %v, want %v", tc.command, got, tc.destructive)
			}
			if got := Windows.IsSudo(tc.command); got != tc.sudo {
				t.Errorf("Windows.IsSudo(%q) = %v, want %v", tc.command, got, tc.sudo)
			}
		})
	}
}

func TestProfileForOS(t *testing.T) {
	cases := map[string]string{
		"darwin":  "darwin",
		"linux":   "linux",
		"windows": "windows",
		"freebsd": "linux",
	}
	for goos, want := range cases {
		if got := forOS(goos).Name(); got != want {
			t.Errorf("forOS(%q).Name() = %q, want %q", goos, got, want)
		}
	}
}

func TestExpandHome(t *testing.T) {
	t.Setenv("HOME", "/home/tester")

	if got := Darwin.ExpandHome("~/notes"); got != "/home/tester/notes" {
		t.Errorf("got %q, want %q", got, "/home/tester/notes")
	}
	if got := Darwin.ExpandHome("/absolute"); got != "/absolute" {
		t.Errorf("got %q, want %q", got, "/absolute")
	}
	if got := Darwin.ExpandHome("relative/path"); got != "relative/path" {
		t.Errorf("got %q, want %q", got, "relative/path")
	}
}

func TestShellIncludesCommandFlag(t *testing.T) {
	for _, p := range []Profile{Darwin, Linux, Windows} {
		shell := p.Shell()
		if len(shell) < 2 {
			t.Fatalf("%s: shell %v has no command flag", p.Name(), shell)
		}
	}
	if got := Darwin.Shell(); got[len(got)-1] != "-c" {
		t.Errorf("darwin shell %v does not end in -c", got)
	}
	if got := Windows.Shell(); got[len(got)-1] != "-Command" {
		t.Errorf("windows shell %v does not end in -Command", got)
	}
}

// TestCurrentProfileHasItsShell guards against a profile naming a shell
// that is not on the machine.
//
// The tests run real commands through the profile of the machine they run
// on, so a profile that asks for a shell the host does not have makes the
// suite fail on every other platform.
func TestCurrentProfileHasItsShell(t *testing.T) {
	shell := Current().Shell()
	if len(shell) == 0 {
		t.Fatal("the current profile has no shell")
	}
	if _, err := exec.LookPath(shell[0]); err != nil {
		t.Errorf("the current profile runs commands through %s, which is not on this machine", shell[0])
	}
}

// TestExamplesMatchTheProfile guards the rule that platform differences
// live in this package: the few-shot examples sent to the planner must be
// written in the profile's own shell, so a Windows run is never shown POSIX
// commands as model answers.
func TestExamplesMatchTheProfile(t *testing.T) {
	posix := []Profile{Darwin, Linux}
	for _, p := range posix {
		if got := p.Examples(); !strings.Contains(got, "du -ah .") {
			t.Errorf("%s examples do not contain a POSIX example: %q", p.Name(), got)
		}
	}
	win := Windows.Examples()
	if !strings.Contains(win, "Get-ChildItem") || !strings.Contains(win, "Remove-Item") {
		t.Errorf("windows examples are not PowerShell: %q", win)
	}
	if strings.Contains(win, "du -ah .") || strings.Contains(win, "cp -R .") {
		t.Errorf("windows examples contain POSIX commands: %q", win)
	}
}

// TestExamplesDecodeAsPlans keeps every example honest: each example line
// that looks like JSON must decode as a plan with a valid risk label.
func TestExamplesDecodeAsPlans(t *testing.T) {
	for _, p := range []Profile{Darwin, Linux, Windows} {
		examples := 0
		for _, line := range strings.Split(p.Examples(), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "{") {
				continue
			}
			examples++
			pl, err := plan.Decode(line)
			if err != nil {
				t.Errorf("%s example %q does not decode as a plan: %v", p.Name(), line, err)
				continue
			}
			for _, c := range pl.Commands {
				if !plan.Risk(c.Risk).Valid() {
					t.Errorf("%s example command %q has invalid risk %q", p.Name(), c.Command, c.Risk)
				}
			}
		}
		if examples < 2 {
			t.Errorf("%s has %d examples, want at least 2", p.Name(), examples)
		}
	}
}

// TestExamplesTeachAskInsteadOfGuess: the ambiguous-and-harmful request
// must be answered with an empty commands array and a question in caveat,
// not with a destructive guess — an example that did the latter would train
// the exact failure mode away.
func TestExamplesTeachAskInsteadOfGuess(t *testing.T) {
	for _, p := range []Profile{Darwin, Linux, Windows} {
		if !strings.Contains(p.Examples(), `{"commands":[]`) {
			t.Errorf("%s examples do not show the ask-instead-of-guess case", p.Name())
		}
	}
}

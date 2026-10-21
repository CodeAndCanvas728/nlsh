package shell

import "testing"

func TestSplitFindsEveryCommand(t *testing.T) {
	cases := []struct {
		name string
		line string
		want []string
	}{
		{"one", "ls -l", []string{"ls"}},
		{"semicolon", "cd /tmp; ls", []string{"cd", "ls"}},
		{"and", "cd /tmp && ls", []string{"cd", "ls"}},
		{"or", "cd /tmp || ls", []string{"cd", "ls"}},
		{"pipe", "cat file | grep x", []string{"cat", "grep"}},
		{"newline", "cd /tmp\nls", []string{"cd", "ls"}},
		{"quoted separator is not a separator", `echo "a; b"`, []string{"echo"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			segments := Split(tc.line)
			if len(segments) != len(tc.want) {
				t.Fatalf("got %d segments %v, want %d", len(segments), segments, len(tc.want))
			}
			for i, seg := range segments {
				if seg.Program != tc.want[i] {
					t.Errorf("segment %d: got program %q, want %q", i, seg.Program, tc.want[i])
				}
			}
		})
	}
}

func TestFirstBinary(t *testing.T) {
	cases := []struct {
		line string
		want string
		ok   bool
	}{
		{"ls -l", "ls", true},
		{"ls -la /tmp", "ls", true},
		{"FOO=bar ls", "ls", true},
		{"env FOO=bar ls", "ls", true},
		{"command ls", "ls", true},
		{"sudo rm -rf /", "rm", true},
		{"nice -n 10 ls", "ls", true},
		{"xargs -0 rm", "rm", true},
		{"/bin/ls", "/bin/ls", true},
		{"", "", false},
		{"FOO=bar", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.line, func(t *testing.T) {
			got, ok := FirstBinary(tc.line)
			if got != tc.want || ok != tc.ok {
				t.Errorf("FirstBinary(%q) = %q, %v; want %q, %v", tc.line, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestFirstSubcommand(t *testing.T) {
	cases := []struct {
		line string
		want string
		ok   bool
	}{
		{"git push --force", "push", true},
		{"git -C /tmp push", "push", true},
		{"docker ps --format json", "ps", true},
		{"ls -l", "", false},
		{"ls /tmp", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.line, func(t *testing.T) {
			got, ok := FirstSubcommand(tc.line)
			if got != tc.want || ok != tc.ok {
				t.Errorf("FirstSubcommand(%q) = %q, %v; want %q, %v", tc.line, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestSplitKeepsQuotedWordsTogether(t *testing.T) {
	segments := Split(`echo "hello world"`)
	if len(segments) != 1 {
		t.Fatalf("got %d segments, want 1", len(segments))
	}
	if got := segments[0].Args[0]; got != "hello world" {
		t.Errorf("got arg %q, want %q", got, "hello world")
	}
}

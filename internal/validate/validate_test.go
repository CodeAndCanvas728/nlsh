package validate

import (
	"reflect"
	"testing"
)

func TestPathsMentioned(t *testing.T) {
	cases := []struct {
		name    string
		request string
		want    []string
	}{
		{
			name:    "absolute path",
			request: "find files in /var/log",
			want:    []string{"/var/log"},
		},
		{
			name:    "home path",
			request: "open ~/notes/today.md",
			want:    []string{"~/notes/today.md"},
		},
		{
			name:    "relative path",
			request: "clean ./dist",
			want:    []string{"./dist"},
		},
		{
			name:    "parent path",
			request: "move it to ../backup",
			want:    []string{"../backup"},
		},
		{
			name:    "and or is not a path",
			request: "sort files A and/or B",
			want:    nil,
		},
		{
			name:    "url is not a path",
			request: "fetch https://example.com/data.json",
			want:    nil,
		},
		{
			name:    "bare word is not a path",
			request: "rename foo.txt to bar.txt",
			want:    nil,
		},
		{
			name:    "path at the start",
			request: "/etc/hosts is broken",
			want:    []string{"/etc/hosts"},
		},
		{
			name:    "path in brackets",
			request: "chmod 644 (./file)",
			want:    []string{"./file"},
		},
		{
			name:    "duplicate paths are reported once",
			request: "copy /tmp/a to /tmp/a",
			want:    []string{"/tmp/a"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := PathsMentioned(tc.request)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("PathsMentioned(%q) = %v, want %v", tc.request, got, tc.want)
			}
		})
	}
}

func TestQuotesBalanced(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{`ls -l`, true},
		{`echo "hello"`, true},
		{`echo 'hello'`, true},
		{`echo "unclosed`, false},
		{`echo 'unclosed`, false},
		{`echo "mixed'`, false},
		{`echo escaped\" quote`, true},
	}

	for _, tc := range cases {
		t.Run(tc.line, func(t *testing.T) {
			if got := quotesBalanced(tc.line); got != tc.want {
				t.Errorf("quotesBalanced(%q) = %v, want %v", tc.line, got, tc.want)
			}
		})
	}
}

func TestLongFlags(t *testing.T) {
	cases := []struct {
		line string
		want []string
	}{
		{"ls -l", nil},
		{"find . --name '*.go'", []string{"--name"}},
		{"git push --force origin main", []string{"--force"}},
		{"tar --exclude=x -c .", []string{"--exclude"}},
	}

	for _, tc := range cases {
		t.Run(tc.line, func(t *testing.T) {
			got := longFlags(tc.line)
			if len(got) != len(tc.want) {
				t.Fatalf("longFlags(%q) = %v, want %v", tc.line, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("longFlags(%q) = %v, want %v", tc.line, got, tc.want)
				}
			}
		})
	}
}

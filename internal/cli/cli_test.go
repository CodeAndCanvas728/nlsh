package cli

import (
	"strings"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		request string
		check   func(t *testing.T, o Options)
	}{
		{
			name:    "plain request",
			args:    []string{"list", "files"},
			request: "list files",
		},
		{
			name:    "options before the request",
			args:    []string{"--print", "list", "files"},
			request: "list files",
			check: func(t *testing.T, o Options) {
				if !o.PrintMode {
					t.Error("PrintMode is false")
				}
			},
		},
		{
			name:    "double dash ends the options",
			args:    []string{"--print", "--", "rm", "--", "-rf"},
			request: "rm -- -rf",
			check: func(t *testing.T, o Options) {
				if !o.PrintMode {
					t.Error("PrintMode is false")
				}
			},
		},
		{
			name:    "short options",
			args:    []string{"-y", "-n", "list"},
			request: "list",
			check: func(t *testing.T, o Options) {
				if !o.AutoYes || !o.DryRun {
					t.Errorf("AutoYes=%v DryRun=%v, want both true", o.AutoYes, o.DryRun)
				}
			},
		},
		{
			name:    "value options",
			args:    []string{"--model", "gpt-5-mini", "--base-url", "http://x/v1", "hello"},
			request: "hello",
			check: func(t *testing.T, o Options) {
				if o.Model != "gpt-5-mini" || o.BaseURL != "http://x/v1" {
					t.Errorf("Model=%q BaseURL=%q", o.Model, o.BaseURL)
				}
			},
		},
		{
			name:    "timeout",
			args:    []string{"--timeout", "30s", "hello"},
			request: "hello",
			check: func(t *testing.T, o Options) {
				if o.Timeout != 30*time.Second {
					t.Errorf("Timeout = %v, want 30s", o.Timeout)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o, err := Parse(tc.args)
			if err != nil {
				t.Fatalf("Parse(%v): %v", tc.args, err)
			}
			if o.Request != tc.request {
				t.Errorf("Request = %q, want %q", o.Request, tc.request)
			}
			if tc.check != nil {
				tc.check(t, o)
			}
		})
	}
}

func TestParseRejectsBadInput(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"unknown option", []string{"--bogus", "hello"}, "unknown option"},
		{"missing value", []string{"--model"}, "expects a value"},
		{"option after the request", []string{"hello", "--print"}, "after the request"},
		{"bad timeout", []string{"--timeout", "soon", "hello"}, "duration"},
		{"negative timeout", []string{"--timeout", "-5s", "hello"}, "negative"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.args)
			if err == nil {
				t.Fatalf("Parse(%v) succeeded, want an error", tc.args)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestParseAllowsFlagsAfterDoubleDash(t *testing.T) {
	// The request itself may contain something that looks like a flag.
	// "--" marks the end of the options, so everything after it is the
	// request.
	o, err := Parse([]string{"--", "remove files matching", "--force"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if o.Request != "remove files matching --force" {
		t.Errorf("Request = %q", o.Request)
	}
}

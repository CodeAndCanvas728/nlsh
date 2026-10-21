package backend

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLauncherAcceptsAnApfelServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			_, _ = w.Write([]byte(`{"model":"apple-foundationmodel","available":true}`))
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"apple-foundationmodel"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	launcher := &ApfelLauncher{BaseURL: server.URL + "/v1"}
	if err := launcher.Ensure(context.Background()); err != nil {
		t.Errorf("Ensure: %v", err)
	}
}

func TestLauncherRefusesAForeignServer(t *testing.T) {
	// Ollama serves an OpenAI-compatible API too. Finding one on the
	// port must not be mistaken for apfel.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"object":"list","data":null}`))
	}))
	defer server.Close()

	launcher := &ApfelLauncher{BaseURL: server.URL + "/v1"}
	err := launcher.Ensure(context.Background())
	if err == nil {
		t.Fatal("expected an error when something else is serving")
	}
	if got := err.Error(); !strings.Contains(got, "something else is already serving") {
		t.Errorf("error = %q, want it to explain the conflict", got)
	}
}

func TestLauncherLeavesRemoteEndpointsAlone(t *testing.T) {
	launcher := &ApfelLauncher{BaseURL: "https://api.openai.com/v1"}
	if err := launcher.Ensure(context.Background()); err != nil {
		t.Errorf("Ensure: %v", err)
	}
}

func TestLauncherNeedsAnExplicitPort(t *testing.T) {
	launcher := &ApfelLauncher{BaseURL: "http://localhost/v1"}
	err := launcher.Ensure(context.Background())
	if err == nil {
		t.Fatal("expected an error when the URL has no port")
	}
	if !strings.Contains(err.Error(), "no explicit port") {
		t.Errorf("error = %q, want it to mention the missing port", err)
	}
}

func TestLauncherReportsAMissingApfel(t *testing.T) {
	// An unusual port, so that nothing else is listening on it.
	launcher := &ApfelLauncher{
		BaseURL: "http://localhost:45999/v1",
		Binary:  "/nonexistent/apfel",
		Timeout: time.Second,
	}
	err := launcher.Ensure(context.Background())
	if err == nil {
		t.Fatal("expected an error when apfel is not installed")
	}
	if !strings.Contains(err.Error(), "apfel is not at") {
		t.Errorf("error = %q, want it to say apfel is missing", err)
	}
}

func TestFindApfelReportsAMissingBinary(t *testing.T) {
	launcher := &ApfelLauncher{Binary: "/nonexistent/apfel"}
	if _, err := launcher.findApfel(); err == nil {
		t.Fatal("expected an error when the binary is not there")
	}
}

func TestServerRoot(t *testing.T) {
	cases := map[string]string{
		"http://localhost:11434/v1":  "http://localhost:11434",
		"http://localhost:11434/v1/": "http://localhost:11434",
		"http://localhost:11434":     "http://localhost:11434",
	}
	for base, want := range cases {
		if got := serverRoot(base); got != want {
			t.Errorf("serverRoot(%q) = %q, want %q", base, got, want)
		}
	}
}

func TestIsLocalhost(t *testing.T) {
	cases := map[string]bool{
		"http://localhost:11434/v1": true,
		"http://127.0.0.1:11434/v1": true,
		"http://127.0.0.5:11434/v1": true,
		"https://api.openai.com/v1": false,
		"http://192.168.1.10:11434": false,
	}
	for base, want := range cases {
		if got := isLocalhost(base); got != want {
			t.Errorf("isLocalhost(%q) = %v, want %v", base, got, want)
		}
	}
}

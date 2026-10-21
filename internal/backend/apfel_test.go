package backend

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
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

func TestLauncherAcceptsAServerOnADefaultPort(t *testing.T) {
	// apfel is already answering, and the URL does not name a port. There
	// is nothing to start, so the missing port is not a problem.
	launcher := &ApfelLauncher{
		BaseURL: "http://localhost/v1",
		probe:   func(context.Context) (bool, bool, error) { return true, true, nil },
	}
	if err := launcher.Ensure(context.Background()); err != nil {
		t.Errorf("Ensure: %v", err)
	}
}

func TestLauncherStartsApfelAndWaitsForIt(t *testing.T) {
	// Nothing is answering at first; apfel comes up on the second poll.
	// This is the path the Mac on-device flow takes, and it must not
	// depend on anything but the probe.
	truePath := standInForApfel(t)
	calls := 0
	var started []string
	launcher := &ApfelLauncher{
		BaseURL: "http://localhost:11434/v1",
		Binary:  truePath,
		Timeout: 5 * time.Second,
		probe: func(context.Context) (bool, bool, error) {
			calls++
			if calls == 1 {
				return false, false, nil
			}
			return true, true, nil
		},
		command: func(name string, args ...string) *exec.Cmd {
			started = append(started, name+" "+strings.Join(args, " "))
			// A command that starts and exits immediately: the launcher
			// does not wait for it, it only waits for the endpoint.
			return exec.Command(truePath)
		},
	}

	if err := launcher.Ensure(context.Background()); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if len(started) != 1 {
		t.Fatalf("apfel was started %d times, want 1", len(started))
	}
	if !strings.Contains(started[0], "--serve") || !strings.Contains(started[0], "11434") {
		t.Errorf("started %q, want it to serve on port 11434", started[0])
	}
}

// TestLauncherPollsTheRealEndpointAfterStartingApfel covers the path the
// Mac on-device flow takes: nothing is listening, the launcher starts
// apfel, and then polls the endpoint until it answers.
//
// It deliberately leaves the probe field nil, so the launcher uses the real
// one. A launcher that reached for the field instead of the accessor would
// dereference a nil function here.
func TestLauncherPollsTheRealEndpointAfterStartingApfel(t *testing.T) {
	// A port that nothing is listening on yet.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"model":"` + ApfelModel + `"}`))
	})}
	t.Cleanup(func() { _ = server.Close() })

	truePath := standInForApfel(t)
	launcher := &ApfelLauncher{
		BaseURL: fmt.Sprintf("http://127.0.0.1:%d/v1", port),
		Binary:  truePath,
		Timeout: 10 * time.Second,
		command: func(name string, args ...string) *exec.Cmd {
			// The "apfel" that gets started is the test server.
			go func() {
				for range 20 {
					l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
					if err == nil {
						// Closing the server closes this listener too.
						_ = server.Serve(l)
						return
					}
					time.Sleep(50 * time.Millisecond)
				}
			}()
			return exec.Command(truePath)
		},
	}

	if err := launcher.Ensure(context.Background()); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
}

func TestLauncherNeedsAnExplicitPort(t *testing.T) {
	// Nothing is answering, so apfel would have to be started, and a URL
	// without a port does not say where to start it.
	launcher := &ApfelLauncher{
		BaseURL: "http://localhost/v1",
		probe:   func(context.Context) (bool, bool, error) { return false, false, nil },
	}
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
		probe:   func(context.Context) (bool, bool, error) { return false, false, nil },
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
		"http://192.0.2.10:11434":   false,
	}
	for base, want := range cases {
		if got := isLocalhost(base); got != want {
			t.Errorf("isLocalhost(%q) = %v, want %v", base, got, want)
		}
	}
}

// standInForApfel returns the path of a program that starts and exits
// immediately, to stand in for the apfel binary.
//
// It is looked up rather than hardcoded: /usr/bin/true does not exist
// everywhere, and the launcher checks that the binary is there before it
// starts it.
func standInForApfel(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("true")
	if err != nil {
		t.Skip("no 'true' program to stand in for apfel")
	}
	return path
}

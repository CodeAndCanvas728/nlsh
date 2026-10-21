package backend

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ApfelModel is the model name apfel serves Apple Foundation Models under.
const ApfelModel = "apple-foundationmodel"

// apfelCandidatePaths are where apfel is usually installed.
var apfelCandidatePaths = []string{
	"/opt/homebrew/bin/apfel",
	"/usr/local/bin/apfel",
	"/usr/bin/apfel",
}

// ApfelLauncher starts apfel when the endpoint nlsh was pointed at is not
// answering.
//
// It checks that whatever is already listening on the port really is apfel.
// Checking only that "something answers" is not enough: Ollama also serves
// an OpenAI-compatible API and usually holds port 11434, so nlsh would
// quietly talk to the wrong server and report a confusing "model not found".
type ApfelLauncher struct {
	// BaseURL is the endpoint nlsh will talk to.
	BaseURL string
	// Binary is the path to the apfel binary. It is discovered when
	// empty.
	Binary string
	// Timeout is how long to wait for apfel to start answering.
	Timeout time.Duration
	// Log receives progress messages.
	Log func(string)

	// command runs the apfel binary. It is a field so that tests can
	// substitute a fake.
	command func(name string, args ...string) *exec.Cmd

	// probe reports whether something is answering at the endpoint and
	// whether it is apfel. It is a field so that the tests can say what
	// is on the other end without depending on what happens to be
	// listening on the machine.
	probe func(ctx context.Context) (served bool, isApfel bool, err error)
}

// DefaultApfelTimeout is how long the launcher waits for apfel to answer.
const DefaultApfelTimeout = 30 * time.Second

// Ensure makes sure an apfel server is answering at the configured URL.
//
// It returns nil when the endpoint is not local, when apfel is already
// serving, or when apfel was started successfully.
func (l *ApfelLauncher) Ensure(ctx context.Context) error {
	if !isLocalhost(l.BaseURL) {
		// A remote endpoint is not ours to start.
		return nil
	}

	// Look before deciding. If apfel is already answering there is
	// nothing to start, and the URL needs no port: a server on the
	// default port is still a server.
	served, isApfel, err := l.check(ctx)
	if err != nil {
		return err
	}
	if isApfel {
		return nil
	}
	if served {
		return fmt.Errorf("something else is already serving on %s, so nlsh cannot start apfel there; "+
			"stop that server or point NLSH_BASE_URL somewhere else", l.BaseURL)
	}

	// Nothing is answering, so apfel has to be started, and starting it
	// needs a port to listen on. A URL without an explicit port does not
	// say where that is.
	port, err := l.port()
	if err != nil {
		return err
	}

	binary, err := l.findApfel()
	if err != nil {
		return err
	}

	l.logf("starting %s --serve --port %d", binary, port)
	cmd := l.run(binary, "--serve", "--port", strconv.Itoa(port))
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not start apfel: %w", err)
	}
	// The server keeps running after nlsh exits, so it is deliberately
	// not waited on.

	timeout := l.Timeout
	if timeout <= 0 {
		timeout = DefaultApfelTimeout
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
		_, isApfel, err := l.check(ctx)
		if err == nil && isApfel {
			return nil
		}
	}
	return fmt.Errorf("apfel did not start answering within %s; try starting it yourself with 'apfel --serve'", timeout)
}

// run builds the command used to start apfel.
func (l *ApfelLauncher) run(name string, args ...string) *exec.Cmd {
	if l.command != nil {
		return l.command(name, args...)
	}
	cmd := exec.Command(name, args...)
	// Detach from nlsh so the server outlives it.
	cmd.SysProcAttr = detachAttributes()
	return cmd
}

// logf writes a progress message if a log function was set.
func (l *ApfelLauncher) logf(format string, args ...any) {
	if l.Log != nil {
		l.Log(fmt.Sprintf(format, args...))
	}
}

// check reports whether something is answering at the endpoint, and whether
// it is apfel.
//
// There is one way to probe, so the two places that need to probe cannot
// drift apart. The probe field is nil in production and set by the tests.
func (l *ApfelLauncher) check(ctx context.Context) (served bool, isApfel bool, err error) {
	if l.probe != nil {
		return l.probe(ctx)
	}
	return l.probeEndpoint(ctx)
}

// probeEndpoint reports whether anything is answering at the URL, and
// whether it is apfel.
func (l *ApfelLauncher) probeEndpoint(ctx context.Context) (served bool, isApfel bool, err error) {
	client := &http.Client{Timeout: 3 * time.Second}
	root := serverRoot(l.BaseURL)
	for _, path := range []string{"/health", "/v1/models"} {
		endpoint := root + path
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		served = true
		if strings.Contains(string(body), ApfelModel) {
			return true, true, nil
		}
	}
	return served, false, nil
}

// serverRoot returns the root of an API server.
//
// The base URL nlsh is given usually ends in /v1, because that is where the
// OpenAI-compatible endpoints live. The health endpoint sits at the root,
// next to it.
func serverRoot(baseURL string) string {
	root := strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(root, "/v1") {
		root = strings.TrimSuffix(root, "/v1")
	}
	return root
}

// port returns the port the apfel server should listen on.
func (l *ApfelLauncher) port() (int, error) {
	u, err := url.Parse(l.BaseURL)
	if err != nil {
		return 0, fmt.Errorf("cannot parse %q: %w", l.BaseURL, err)
	}
	if p := u.Port(); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil {
			return 0, fmt.Errorf("cannot parse port %q: %w", p, err)
		}
		return n, nil
	}
	return 0, fmt.Errorf("%s has no explicit port, so nlsh cannot know where to start apfel; "+
		"write it as http://localhost:11434/v1", l.BaseURL)
}

// findApfel locates the apfel binary.
func (l *ApfelLauncher) findApfel() (string, error) {
	if l.Binary != "" {
		if _, err := os.Stat(l.Binary); err == nil {
			return l.Binary, nil
		}
		return "", fmt.Errorf("apfel is not at %s", l.Binary)
	}
	for _, candidate := range apfelCandidatePaths {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	if path, err := exec.LookPath("apfel"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("apfel is not installed, so there is no local Apple Foundation Models server; " +
		"install it with 'brew install apfel' or point NLSH_BASE_URL at another OpenAI-compatible server")
}

// isLocalhost reports whether a URL points at this machine.
func isLocalhost(baseURL string) bool {
	u, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	switch host {
	case "localhost", "127.0.0.1", "::1", "[::1]":
		return true
	}
	return strings.HasPrefix(host, "127.")
}

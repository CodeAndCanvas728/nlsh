# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project
uses [semantic versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- A Homebrew tap, at `CodeAndCanvas728/homebrew-nlsh`, and a CI job that
  compares the formula in this repository against the copy in the tap, so a
  tap that drifts from the repository fails the build rather than
  installing an old build quietly. (The formula itself was already here.)

### Changed

- The install section of the README says how to install it with Homebrew
  and what the tap is for.

## [0.3.1] — 2026-10-22

The fixes that came out of the review of the first release. The v0.3.0
binaries are the ones before these; this is the release to use.

### Fixed

- `--timeout` was parsed and then never read, so a command that took a long
  time took as long as it liked. Each command now gets the limit, a command
  that runs out of time is reported as such, and the run stops with the
  status 124.
- The command is stopped as a whole group, because a shell that forks leaves
  the command running, still holding the output pipe, so the wait would not
  return. The group is only signalled while the child is provably still
  ours, so a kill cannot reach a process group whose id has been recycled.
- On Windows the stop was a no-op: `os.Process.Signal` only supports Kill
  there, so the liveness probe always failed and the group was never
  stopped. The children are taken with it now, with `taskkill /T`.
- An interrupt is no longer reported as a timeout: the run says which
  command was interrupted and reports the command's own status.
- A command killed by a signal is reported with the shell's convention for
  one, 128 plus the signal number, instead of -1. Windows has no signal
  convention and no way to ask a process how it died, so a command that
  was killed there is reported as 127 — "unknown" — rather than as a
  signal that never happened.
- The Makefile no longer passes a `-X main.version` flag: there is no
  `main.version`, the version is a constant in `internal/cli`, and the flag
  was silently doing nothing. The version lives in the source alone, and
  `make version` fails loudly rather than printing an empty line when it
  cannot be read.
- The release script reads the version the same way the Makefile does, and
  checks the tag against it, so a tag that does not match the source stops
  the build instead of publishing a release that says the wrong version.

### Changed

- The release is published by GitHub, through the Forgejo push mirror: a
  `v*` tag pushed to Forgejo arrives on GitHub and the six binaries are
  published there. The documentation and the Homebrew formula point at the
  GitHub releases, which is where the assets are.

## [0.3.0] — 2026-10-21

The first release of the Go rewrite. The Swift implementation in `legacy/`
is kept for reference while the port is validated.

### Added

- A Go implementation with no third-party dependencies: one static binary
  for macOS, Linux and Windows.
- A `platform.Profile` abstraction so the shell, the coreutils dialect, the
  destructive-command patterns and the home directory are decided in one
  place per operating system.
- Windows support, command line only: PowerShell execution, a PowerShell
  dialect prompt and a Windows set of destructive-command patterns.
- The plan is now requested as a JSON Schema (`response_format.json_schema`),
  so a compliant server returns a plan that always parses.
- Tests for the validator, the risk classifier, the command-line parser, the
  shell tokenizer and the whole plan/validate/approve/run pipeline, driven
  by a fake OpenAI-compatible server.
- CI on ubuntu and on macOS; a release workflow that builds the six
  binaries with checksums and publishes them from GitHub; a Homebrew
  formula.

### Fixed

- nlsh no longer crashes when it starts apfel. The launcher reached for the
  probe field directly in the wait loop after starting the server, and the
  field is nil in production, so the on-device flow died with a nil pointer
  dereference and left an orphan server running. There is now one way to
  probe, and the start path is covered by a test that uses the real one.
- The apfel launcher asks the endpoint before it works out the port, so a
  server that is already answering is accepted even when the URL does not
  name a port. The launcher only needs a port when it has to start apfel.
- The apfel tests no longer depend on what happens to be listening on the
  machine: the probe is injected, so "apfel is serving" and "nothing is
  serving" are both stated by the test rather than by the environment.
- The tests that run commands skip on a platform whose shell is not POSIX.
  They use `false`, `sh` and `ls --not-a-real-flag`, which do not exist
  under the Windows profile's PowerShell.
- The `--backend` option is gone. It was accepted and stored but never read,
  so it silently did nothing.
- The release build is static: `CGO_ENABLED=0`, so the binary runs on a
  machine with a different libc from the one that built it.
- The release build starts from an empty directory, so a stale archive from
  an earlier build cannot end up in the checksums, and it says so when
  `zip` or `sha256sum` is missing instead of failing at tag time.
- The checksums name the files without a `./` prefix.
- The CI pins the Go version from `go.mod` instead of "stable", and one
  job runs `go test ./...` over every package rather than a hardcoded list
  that a new package would fall out of.
- The release runs `make check` before it publishes anything, and has a
  timeout.
- The `--print` mode no longer writes the progress messages to stdout, so
  the zsh integration pastes the command and nothing else into the buffer.
- The OpenAI-compatible backend now records the assistant's own replies, so
  a follow-up such as "now make it read-only" sees the plan that was
  proposed.
- A path mentioned in a request is no longer mistaken for a path when it is
  part of a URL or the word "and/or".
- The flag check consults the help of the subcommand, so `git push --force`
  is no longer reported as using a flag that does not exist.
- A command that deletes with `rm` is recognised whatever the order of its
  flags, and `rm` with separate `-r` and `-f` is recognised too.
- `sudo` is recognised when it is called by its full path.
- A quoted word is never mistaken for a command.
- The editor is run directly rather than through a shell, so a value in
  `$EDITOR` cannot become a shell command.
- The prompt now carries today's date; the on-device model has no clock and
  would otherwise invent one.
- An option after the request is reported instead of being sent to the model
  as part of the request.
- An unrecognised risk label is treated as careful rather than safe.
- The tests ran real commands through the macOS profile, whose shell is
  `/bin/zsh`. The suite now uses the profile of the machine it runs on, and a
  test checks that the profile names a shell the machine actually has.
- The release build was broken in both forges: the target list is `os-arch`
  but the script split it as if it were `os/arch`, so `GOOS` came out as
  `darwin-arm64` and the build stopped on the first target; and `go build`
  does not add an extension to `-o`, so the Windows binary was never named
  `.exe`. The build now lives in `scripts/build-release.sh`, which the
  GitHub release workflow calls, and each archive holds one binary called
  `nlsh`.

### Changed

- The release is published by GitHub, not by Forgejo. The repository reaches
  GitHub through a Forgejo push mirror, so a `v*` tag pushed to Forgejo
  arrives on GitHub and the six binaries are published there. The
  documentation and the Homebrew formula point at the GitHub releases,
  which is where the assets are.
- The repository is set to squash merges and to delete the branch after a
  merge. An early version of the release script left a compiled binary in
  the repository, and it is in the history of the branch; a squash merge
  keeps it out of `main`.

### Known limitations

- The plan contract is checked, not verified. A command can pass every
  check and still be wrong.
- Windows has no shell integration: the Ctrl+O buffer insertion is zsh-only.
- The validator shells out to each command's `--help`, which is slow for
  tools with large help output.

[0.3.1]: https://github.com/CodeAndCanvas728/nlsh/releases/tag/v0.3.1
[0.3.0]: https://github.com/CodeAndCanvas728/nlsh/releases/tag/v0.3.0

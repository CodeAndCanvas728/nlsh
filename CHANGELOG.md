# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project
uses [semantic versioning](https://semver.org/spec/v2.0.0.html).

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
- CI for macOS, Linux and Windows; a release workflow that builds six
  binaries with checksums; a Homebrew formula.

### Fixed

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

### Known limitations

- The plan contract is checked, not verified. A command can pass every
  check and still be wrong.
- Windows has no shell integration: the Ctrl+O buffer insertion is zsh-only.
- The validator shells out to each command's `--help`, which is slow for
  tools with large help output.

[0.3.0]: https://github.com/jonah/nlsh/releases/tag/v0.3.0

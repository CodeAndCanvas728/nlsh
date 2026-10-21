# nlsh

Natural-language shell: you describe a request, a model turns it into shell
commands, the plan is validated locally, shown for approval, and run.

## Layout

```
cmd/nlsh/            the command; shells/zsh.sh is the zsh integration (embedded)
internal/app/        plan → validate → approve → run
internal/backend/    talking to the model; apfel launcher
internal/validate/   checking a plan
internal/guard/      deciding how risky a command is
internal/platform/   what differs between operating systems
internal/shell/      parsing command lines
internal/plan/       the plan contract and its JSON Schema
legacy/              the original Swift implementation (reference only)
```

No third-party Go dependencies. Everything is the standard library.

## Build and check

```sh
make check      # gofmt, go vet, go test -race — this is what CI runs
make build      # ./nlsh
make test       # tests only
```

`make check` is the gate. `gofmt -l .` must print nothing.

## The rules that are easy to break

**stdout carries commands, stderr carries everything else.** In `--print`
mode stdout must contain *only* the generated commands, one per line, because
the zsh integration captures it with `out=$(nlsh --print -- "$@")` and puts
it in the editable buffer. Progress, warnings and errors always go to
stderr. `report.Reporter` owns this; use `Reporter.Command` for the commands
and `Reporter.Progress`/`Warn`/`Error`/`Note` for everything else.

**The plan is requested as a JSON Schema.** `plan.Schema` is sent as
`response_format.json_schema`, so a compliant server returns a plan that
always parses. Servers that reject it answer 400 and the request is retried
with the looser `json_object` format — see `backend.OpenAI.send`. When the
plan shape changes, change `plan.Schema` and the `plan.Plan`/`plan.Command`
types together, or the two drift apart.

**The conversation records the assistant's replies.** `backend.OpenAI.history`
holds the system message, every user message *and* every assistant reply.
Without the assistant replies the model never sees the plan it proposed and
a follow-up such as "now make it read-only" has nothing to refer to. The
`TestOpenAIKeepsAssistantReplies` test guards this.

**The planner's risk label is never trusted alone.** `guard.Classify` takes
the higher of the model's label and `platform.Profile.IsDestructive`. An
unrecognised label is treated as *careful*, not safe.

**Platform differences live in `internal/platform`.** The shell, the
coreutils dialect, the destructive-command patterns and the home directory
are decided by `Profile`. Nothing outside that package should branch on
`runtime.GOOS`. Adding a platform means adding a profile and a test.

## Validation

`validate.Checker` gates obvious breakage, not correctness. Two things about
it are deliberate and easy to undo by accident:

- **The flag check consults the subcommand's help**, not the tool's. For
  `git push --force` it runs `git push --help`, because `git --help` does
  not mention `--force` and every git command with a flag would be reported
  as broken. It runs `<bin> --help` as a fallback.
- **Paths in a request are extracted with a boundary**, and URLs are removed
  first. A naive pattern reads "and/or" as the path `/or` and the path of a
  URL as a local file, which makes the request fail validation for no
  reason. `TestPathsMentioned` covers these.

The checker shells out to each binary's `--help` (3s timeout, cached per
binary). It is the slowest part of a plan.

## The apfel launcher

`backend.ApfelLauncher` starts `apfel --serve` when a local endpoint is not
answering. It checks that whatever *is* answering is really apfel, by
looking for `apple-foundationmodel` in the response. Checking only that a
port answers is not enough: Ollama serves an OpenAI-compatible API and
usually holds port 11434, so nlsh would quietly talk to the wrong server.
The launcher therefore refuses to start apfel when something else is on the
port, and says so.

## Command line

Options come **before** the request. The first non-option word starts the
request, and everything after it belongs to the request, so an option in
that position is an error rather than something sent to the model. Use `--`
to end the options explicitly:

```sh
nlsh -- "remove files matching --force"
```

## Exit codes

`0` handled or cancelled · `1` no plan could be produced · `2` the plan was
rejected (validation issues remained, or it needed elevated privileges) ·
anything else is the exit status of the last command that ran.

## The legacy Swift implementation

`legacy/` holds the original. It is kept for reference while the port is
validated and is not built by CI. It builds with:

```sh
cd legacy && swiftc -O -o nlsh nlsh.swift main.swift
```

It still has the bugs the Go version fixed — `legacy/README.md` lists them.
Do not use it as a reference for behaviour; use it as a reference for
intent.

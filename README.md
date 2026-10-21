# nlsh

Natural-language shell. You describe what you want in plain words; a model
turns it into shell commands; the plan is checked locally, shown to you, and
run once you approve it.

```
$ nlsh "symlink the Designs folder to dist"
```

On a Mac with Apple Intelligence, the model is the one already built into
your machine: no API key, no cloud, no download. On any platform, point nlsh
at any OpenAI-compatible server.

## Install

```sh
brew install jonah/tap/nlsh      # macOS
```

Or download a binary for your platform from the
[releases page](https://github.com/jonah/nlsh/releases), or build it
yourself:

```sh
go install github.com/jonah/nlsh/cmd/nlsh@latest
```

## Use

```sh
nlsh "find the three biggest files in this folder"
nlsh -n "list files here"          # show the plan, run nothing
nlsh --print "list files here"     # print the commands only
nlsh                              # interactive, keeps the conversation
```

Options come **before** the request. Once the first word of the request has
been seen, everything after it belongs to the request, so `nlsh "list files"
--dry-run` is an error rather than a silent surprise. Use `--` when the
request itself contains something that looks like an option:

```sh
nlsh -- "remove files matching --force"
```

### Shell integration

```sh
eval "$(nlsh --zsh-init)"
```

Then type a request on your prompt and press **Ctrl+O**. The generated
command replaces the line you were typing, so you can review and edit it
before pressing Enter. Nothing runs until you press Enter.

`nls` runs the plan the normal way; `nli` puts the command in the buffer
without running it.

## Model

nlsh asks an OpenAI-compatible server for the plan:

| | |
|---|---|
| `--base-url` / `NLSH_BASE_URL` | the endpoint, for example `http://localhost:11434/v1` |
| `--api-key` / `NLSH_API_KEY` | bearer token; `OPENAI_API_KEY` also works |
| `--model` / `NLSH_MODEL` | the model to ask for |

If the endpoint is local and nothing is listening on it, nlsh starts
[apfel](https://github.com/Arthur-Ficial/apfel) for you, which serves Apple
Foundation Models on-device:

```sh
brew install apfel
```

```sh
export NLSH_BASE_URL=http://localhost:11434/v1
export NLSH_MODEL=apple-foundationmodel
nlsh "what is on my disk"
```

Ollama, LM Studio and any other OpenAI-compatible server work the same way.

## Safety

- The plan is always shown before anything runs, and nothing runs without
  approval.
- `-y` approves everything except destructive commands. Destructive commands
  need the word `yes` typed out. `-f` skips even that — carefully.
- A command that needs elevated privileges is refused outright, because it
  would ask for a password nobody can type.
- Every plan is checked before you see it: the binary has to exist, the
  flags have to appear in the tool's own help, quotes have to balance, and
  the paths you mentioned have to be used. If something is wrong, the model
  is asked once to fix it.

The risk shown next to a command is the higher of what the model said and
what nlsh detected itself. The model's own label is never enough on its own.

## Exit codes

| code | meaning |
|---|---|
| 0 | the request was handled, or was cancelled |
| 1 | no plan could be produced |
| 2 | the plan was rejected: validation issues remained, or it needed elevated privileges |
| other | the exit status of the last command that ran |

## Development

```sh
make check      # format, vet, tests with the race detector
make build      # build ./nlsh
make test       # tests only
```

The layout:

```
cmd/nlsh            the command itself
internal/app        plan, validate, approve, run
internal/backend    talking to the model
internal/validate   checking a plan
internal/guard      deciding how risky a command is
internal/platform   what differs between operating systems
internal/shell      parsing command lines
internal/plan       the plan contract and its JSON Schema
cmd/nlsh/shells     the zsh integration
```

`legacy/` holds the original Swift implementation. It is kept for reference
while the port is being validated; it is not built by CI.

## License

MIT. See [LICENSE](LICENSE).

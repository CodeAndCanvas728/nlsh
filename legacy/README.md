# The Swift implementation

This is the original `nlsh`, written in Swift, kept while the Go port in the
parent directory is being validated. It is the reference for the behaviour
the port has to preserve, and the fallback if the port is abandoned.

## Building it

The source is split in two because Swift only allows top-level code in a
file called `main.swift`, and `@main` cannot coexist with the file-scope
constants the program uses:

```sh
swiftc -O -o nlsh nlsh.swift main.swift
```

## What changed when it was converted

The file arrived as `text-274C27D1756C-1.txt`, which `swiftc` refuses to
compile ("ld: unknown file type"). It was renamed to `nlsh.swift` and three
compile errors were fixed:

- a tuple had `.lowercased()` called on it;
- `!await` is not valid Swift — it has to be `!(await …)`;
- `@main` cannot be used in a module that contains top-level code, so the
  entry point moved to `main.swift`.

The mutable global that carried the "Apple Intelligence is off" message was
replaced by a function that returns it, which also cleared a warning about
crossing an actor boundary.

## What it still does wrong

These are fixed in the Go version and are listed here so the differences are
not a surprise:

- `--print` writes the progress messages to stdout, so the zsh integration
  pastes "⋯ thinking" into the buffer.
- The OpenAI-compatible backend never records the assistant's replies, so
  the model does not see the plan it proposed.
- The path heuristic treats "and/or" as the path `/or` and the path of a
  URL as a local file.
- The flag check consults the top-level help of a tool, so every
  `git push --force` is reported as using a flag that does not exist.
- `rm -r -f` with separate flags is not recognised as destructive, and
  `sudo` called by its full path is not recognised at all.
- There is no timeout on the commands that run.
- An option after the request is silently sent to the model as part of it.
- The prompt does not carry today's date, and the on-device model has no
  clock.

## Attribution

The validation-and-regeneration loop and the zsh buffer-insertion
integration were written for this project; the ideas came from `fmc`.

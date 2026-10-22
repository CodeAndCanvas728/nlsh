# Releasing

The code is developed on one forge and the release is published from
another. The two are wired together by settings on the forge rather than by
anything in this repository, so this file records the settings, the steps,
and how to check that they still hold.

## The two homes

| | |
|---|---|
| Developed on | a private forge — the only place it is pushed to |
| Published from | `github.com/CodeAndCanvas728/nlsh` — the public copy, and where the release is published |

The public copy is a **push mirror** of the development copy: every branch
and every tag is carried over, so the public copy is never behind and
nothing is private in the code itself. The mirror pushes over SSH with a
deploy key scoped to this one repository, so the credential cannot reach
anything else.

The module path in `go.mod` is the public one,
`github.com/CodeAndCanvas728/nlsh`, so that `go install` works for anyone
and the public tree does not name a private host.

## Which directory each forge reads

Forgejo runs the workflows in `.forgejo/workflows/`; GitHub runs the ones in
`.github/workflows/`. The two are not interchangeable, and the forge's own
setting is what decides it.

This was measured rather than assumed: with both release workflows present,
three tags were pushed and the development forge ran exactly one release
workflow for each — always the one cloning `forgejo/release-action`, never
the one cloning `softprops/action-gh-release`. A fourth run would have
shown up if it had discovered `.github/workflows/` as well.

## Cutting a release

1. Merge the pull request, **squashing**.
2. Tag the merge commit and push the tag:

   ```sh
   git tag -a v0.3.0 -m "v0.3.0" <merge-commit>
   git push origin refs/tags/v0.3.0
   ```

   Push the tag to the **development** forge, not to the public copy: the
   mirror carries it over from there, and the workflow on the public copy is
   what publishes the release.
3. The workflow runs `make check` and then `scripts/build-release.sh`, which
   builds all six binaries and writes `SHA256SUMS`.
4. Check what was published:

   ```sh
   gh release list --repo CodeAndCanvas728/nlsh
   gh release view v0.3.0 --repo CodeAndCanvas728/nlsh --json assets
   ```

5. Update `homebrew/nlsh.rb`: the version, the tag in the URLs, and the
   checksums. The checksums are in the release's `SHA256SUMS` asset.
6. Update `CHANGELOG.md`: move what is under `## [Unreleased]` into a new
   dated section, and add the link at the bottom.

The version in `internal/cli/usage.go` is the one the binary reports. The
tag, the formula and that constant have to agree, and the release script
checks the tag against it: a tag that does not match the source stops the
build rather than publishing a release that says the wrong version.

## When a build fails

The workflow that publishes the release runs `make check` first, so nothing
is published unless the tests, the vet and the race detector all pass. The
log of a failed step is in the workflow run on the public copy:

```sh
gh run list --repo CodeAndCanvas728/nlsh
gh run view <run-id> --repo CodeAndCanvas728/nlsh --log-failed
```

The development forge runs the same checks, so a failure there is the same
failure the release would have hit. Where that forge is, and how to read
its logs, is in the maintainer's runbook rather than here: this document
is mirrored into the public copy.

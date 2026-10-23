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

The published copy is a **push mirror** of the development copy: every branch
and every tag is carried over, so the published copy is never behind and
nothing is private in the code itself. The mirror pushes over SSH with a
deploy key scoped to this one repository, so the credential cannot reach
anything else.

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
   git tag -a v0.3.1 -m "v0.3.1" <merge-commit>
   git push origin refs/tags/v0.3.1
   ```

   Push the tag to the **development** forge, not to the public copy: the
   mirror carries it over from there, and the workflow on the public copy is
   what publishes the release.
3. The workflow runs `make check` and then `scripts/build-release.sh`, which
   builds all six binaries and writes `SHA256SUMS`.
4. Check what was published:

   ```sh
   gh release list --repo CodeAndCanvas728/nlsh
   gh release view v0.3.1 --repo CodeAndCanvas728/nlsh --json assets
   ```

5. Update `homebrew/nlsh.rb`: the tag in the four URLs (one per
   platform/architecture combination), and the checksums. The version is
   read from the URL by Homebrew, so there is no separate `version`
   line to bump, but the tag has to be changed in all four places. The
   `tap` job's first check catches intra-file drift (it requires all four
   URLs to carry the same tag, matching `internal/cli/usage.go`), but it
   cannot catch a consistent-but-stale bump, so the tag still has to be
   changed deliberately in all four. The README's `go install …@v0.3.1`
   line is a fifth place that carries the version and is not checked by
   anything. The checksums are in the release's `SHA256SUMS` asset.
6. Update `CHANGELOG.md`: move what is under `## [Unreleased]` into a new
   dated section, and add the link at the bottom.
7. Commit the formula and move the tag onto that commit, then publish
   again.
8. Copy the formula to the tap, `CodeAndCanvas728/homebrew-nlsh`, and commit
   it there.

Step 7 is not optional tidiness: the checksums of a release are only known
once the release has been built, and the formula lives in the repository the
release is built from. So the tag and the formula cannot be settled in one
commit. Moving the tag is safe because the archives are reproducible —
`scripts/build-release.sh` fixes the timestamps, the owner and the order —
so a rebuild of the same source produces the same checksums. Check that
before moving it, and check that the formula at the tag installs exactly
what the tag published.

Step 8 is what puts the formula in front of the people using Homebrew. The
formula here is the source of truth; the tap holds the copy, so copy it
rather than editing it there.

```sh
cp homebrew/nlsh.rb /path/to/homebrew-nlsh/Formula/nlsh.rb
```

**The diff against the copy is what catches a stale tap.** A tap that is a
release behind downloads the old asset, the old binary reports the old
version, and Homebrew derives that version from the URL in the old formula
— so the two agree and nothing inside the tap can tell you it is behind.
The difference against this repository can:

```sh
diff -u homebrew/nlsh.rb /path/to/homebrew-nlsh/Formula/nlsh.rb
```

The CI job `tap` does that same comparison on every push to `main`, so a
tap that drifts from the repository fails the build rather than installing
an old build quietly. Copy the formula and commit it in the tap, and the
next run of the CI says whether it took.

The `tap` job runs only on pushes to `main`, so `main` is red in the
window between release steps 7 and 8, until the matching copy lands in
the tap. That failure is expected and is not a broken build; it is the
check that catches a forgotten copy. A formula-changing pull request is
green, because the job does not run there.

To check the tap file the way Homebrew would, use the checks that do not
need the published assets — `brew style` and `brew audit --strict` read the
formula and nothing else. They resolve a tap-qualified name, so the tap
must be tapped first (`brew tap CodeAndCanvas728/nlsh`) and they only
mean something after step 8 has landed the formula in the tap plus a
local refresh (`brew update`); running them earlier checks the previous
release's formula, not the one you edited. After the release this is
post-release verification.

```sh
brew style CodeAndCanvas728/nlsh/nlsh
brew audit --strict --formula CodeAndCanvas728/nlsh/nlsh
```

`brew install` is not a check: it installs. It also needs the published
assets, so it cannot run against an uncommitted formula.

Measured on Homebrew 7.0.10. The commands below are tap-qualified, so
they resolve to whatever is checked out in the tap — which, at the time
of measurement, still held the released formula carrying `version
"0.3.1"`. They therefore confirm that `brew audit --strict` and `brew
style` pass on the released form, and that Homebrew reports `0.3.1`,
but they do not by themselves exercise the version-in-URL form now in
the repository. The version-in-URL form is the same pattern Homebrew's
own `Version.detect_from_url` handles (it strips up to the last
`/download/` and reads `v0.3.1` from the remaining segment), which is
why the runbook accepts it; the `test do` block is the check that
actually exercises the derived version against the binary, run by
`brew test` (manually, or by the tap's own CI if it has one) — nothing
in this repository's CI runs it.

```
$ brew info --json=v2 CodeAndCanvas728/nlsh/nlsh
... "version": "0.3.1" ...
$ brew audit --strict --formula CodeAndCanvas728/nlsh/nlsh
(no output, exit 0)
$ brew style CodeAndCanvas728/nlsh/nlsh
1 file inspected, no offenses detected
```

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

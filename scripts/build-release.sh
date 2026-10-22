#!/usr/bin/env bash
#
# Build the release binaries and their checksums into the output directory.
#
# The GitHub workflow calls this: .github/workflows/release.yaml. The
# repository reaches GitHub through a Forgejo push mirror, so the tag that
# triggers the release is the tag pushed to Forgejo.
#
# Each archive holds one binary named "nlsh" (or "nlsh.exe"), so unpacking
# it gives a runnable program whatever the platform.
#
# Usage: scripts/build-release.sh [output-directory]
set -euo pipefail

out="${1:-dist}"

for tool in zip sha256sum; do
	command -v "$tool" >/dev/null 2>&1 || {
		echo "$tool is not installed: cannot build the release" >&2
		exit 1
	}
done

# The tag, the version in the source and the Homebrew formula have to agree.
# The version cannot be stamped into the binary at build time, so this is
# where the agreement is checked instead.
source_version="$(sed -n 's/^const Version = "\(.*\)"/\1/p' internal/cli/usage.go)"
if [ -z "$source_version" ]; then
	echo "cannot read the version from internal/cli/usage.go" >&2
	exit 1
fi
if [ -n "${GITHUB_REF_NAME:-}" ]; then
	tag="${GITHUB_REF_NAME#v}"
	if [ "$tag" != "$source_version" ]; then
		echo "the tag $GITHUB_REF_NAME does not match the version in the source ($source_version)" >&2
		exit 1
	fi
fi

# The script deletes the output directory before it writes to it, so it
# must be a directory inside the repository: "rm -rf ." would take the
# checkout with it, and "rm -rf /" would take the machine.
root="$(pwd)"
case "$out" in
	/* | "" | "." | "..") echo "the output directory must be inside the repository: '$out'" >&2; exit 1 ;;
esac
parent="$(cd "$(dirname "$out")" 2>/dev/null && pwd)" || {
	echo "the output directory does not exist yet: '$out'" >&2
	exit 1
}
resolved="$parent/$(basename "$out")"
case "$resolved" in
	"$root"/*) ;;
	*) echo "the output directory must be inside the repository: '$out'" >&2; exit 1 ;;
esac

# Start from an empty directory: a stale archive from an earlier build must
# not end up in the checksums.
rm -rf "$out"
mkdir -p "$out"

targets="darwin-arm64 darwin-amd64 linux-amd64 linux-arm64 windows-amd64 windows-arm64"

# Reproducible archives: the same source has to give the same bytes, or
# the checksums change on every build and the Homebrew formula has to be
# rewritten for no reason.
if tar --sort=name --help >/dev/null 2>&1; then
	tarflags=(--sort=name --mtime=2024-01-01T00:00:00Z --owner=0 --group=0 --numeric-owner)
else
	# BSD tar cannot pin the timestamp this way, so the archives are
	# reproducible only where GNU tar is available. The release is
	# built on Linux.
	tarflags=()
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

for target in $targets; do
	# The target is "os-arch": split on the hyphen, not on a slash.
	os="${target%-*}"
	arch="${target#*-}"
	name="nlsh-$target"

	# CGO off makes the binary static, so it runs on a machine with a
	# different libc from the one that built it.
	# -buildvcs=false keeps the revision out of the binary: otherwise
	# every commit produces different bytes and the checksums change
	# for no reason.
	# go build does not add an extension to -o, so build to a plain name
	# and name the artefact afterwards.
	CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
		go build -trimpath -buildvcs=false -ldflags "-s -w" -o "$work/nlsh" ./cmd/nlsh

	if [ "$os" = "windows" ]; then
		cp "$work/nlsh" "$out/nlsh.exe"
		# A fixed timestamp keeps the archive reproducible; -X drops
		# the extra fields the platform would otherwise add.
		touch -t 202401010000 "$out/nlsh.exe"
		(cd "$out" && zip -X -q "$name.zip" nlsh.exe)
		rm -f "$out/nlsh.exe"
	else
		tar "${tarflags[@]+"${tarflags[@]}"}" -czf "$out/$name.tar.gz" -C "$work" nlsh
	fi
done

(cd "$out" && sha256sum ./*.tar.gz ./*.zip | sed 's|\./||' >SHA256SUMS)

ls -1 "$out"

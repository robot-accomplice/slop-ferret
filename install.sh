#!/bin/sh
# slop-ferret installer — downloads a released binary and deploys the skill.
#
#   curl -fsSL https://raw.githubusercontent.com/robot-accomplice/slop-ferret/main/install.sh | sh
#
# It fetches the platform binary from the GitHub release, VERIFIES its sha256 against the release's
# checksums.txt (this is a security tool; an unverified download would be the joke), installs it, and
# runs `ferret install` to deploy the skill — without it, `ferret plan` refuses. macOS and Linux
# only; Windows is not published because `ferret install` creates symlinks and no Windows path in
# this tool has ever run.
#
# Overrides: VERSION=v0.1.0 to pin a release; BINDIR=/path to choose where the binary lands.
set -eu

REPO="robot-accomplice/slop-ferret"
fail() { printf 'install: %s\n' "$1" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

have curl || fail "curl is required"
have tar || fail "tar is required"

# 1. Platform — refuse anything we do not publish, by name, rather than downloading a 404.
os="$(uname -s)"
arch="$(uname -m)"
case "$os" in
	Darwin) os="darwin" ;;
	Linux) os="linux" ;;
	*) fail "unsupported OS '$os' — slop-ferret publishes macOS and Linux binaries only" ;;
esac
case "$arch" in
	x86_64 | amd64) arch="amd64" ;;
	arm64 | aarch64) arch="arm64" ;;
	*) fail "unsupported architecture '$arch' — amd64 and arm64 only" ;;
esac

# 2. Version — the latest release, or a pinned VERSION. Resolve "latest" via the redirect so the
#    unauthenticated GitHub API rate limit cannot make a fresh machine fail to install.
version="${VERSION:-}"
if [ -z "$version" ]; then
	version="$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest" | sed -E 's#.*/tag/##')"
	[ -n "$version" ] || fail "could not resolve the latest release — pin one with VERSION=vX.Y.Z"
fi

asset="slop-ferret_${version}_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/$version"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# 3. Download the archive and the checksums.
printf 'install: fetching %s (%s)\n' "$asset" "$version" >&2
curl -fsSL -o "$tmp/$asset" "$base/$asset" || fail "download failed: $base/$asset (is $version published for $os/$arch?)"
curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt" || fail "could not fetch checksums.txt for $version"

# 4. Verify sha256 before trusting a byte of it.
if have sha256sum; then
	got="$(sha256sum "$tmp/$asset" | awk '{print $1}')"
elif have shasum; then
	got="$(shasum -a 256 "$tmp/$asset" | awk '{print $1}')"
else
	fail "need sha256sum or shasum to verify the download"
fi
# checksums.txt lists names with a leading "./"; strip it before matching.
want="$(awk -v a="$asset" '{f=$2; sub(/^\.\//, "", f); if (f == a) print $1}' "$tmp/checksums.txt")"
[ -n "$want" ] || fail "no checksum for $asset in checksums.txt"
[ "$got" = "$want" ] || fail "checksum mismatch for $asset: got $got, expected $want — NOT installing"

# 5. Extract and place the binary.
tar -xzf "$tmp/$asset" -C "$tmp" || fail "could not extract $asset"
[ -f "$tmp/ferret" ] || fail "archive did not contain a 'ferret' binary"
chmod +x "$tmp/ferret"

bindir="${BINDIR:-}"
if [ -z "$bindir" ]; then
	if [ -w /usr/local/bin ]; then
		bindir="/usr/local/bin"
	else
		bindir="$HOME/.local/bin"
	fi
fi
mkdir -p "$bindir" || fail "cannot create $bindir"
mv "$tmp/ferret" "$bindir/ferret" || fail "cannot install to $bindir (set BINDIR to a writable dir)"
printf 'install: ferret %s installed to %s\n' "$version" "$bindir" >&2

# Warn if the chosen dir is not on PATH — a binary nobody can invoke is not installed.
case ":$PATH:" in
	*":$bindir:"*) ;;
	*) printf 'install: NOTE %s is not on your PATH — add it: export PATH="%s:$PATH"\n' "$bindir" "$bindir" >&2 ;;
esac

# 6. Deploy the skill. The H vocabulary lives in the deployed skill, not the binary; without it
#    `ferret plan` refuses rather than handing back an empty worklist.
printf 'install: deploying the skill (ferret install)...\n' >&2
if "$bindir/ferret" install; then
	printf 'install: done. Verify with: ferret doctor\n' >&2
else
	printf 'install: the binary is installed but `ferret install` did not complete. Run it yourself, then `ferret doctor`.\n' >&2
	exit 1
fi

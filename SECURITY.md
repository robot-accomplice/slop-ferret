# Security Policy

## Reporting a vulnerability

Open a [private security advisory](https://github.com/robot-accomplice/slop-ferret/security/advisories/new)
rather than a public issue. Include what you did, what happened, and what you expected.

## Supported versions

Only the most recent tagged release. There is currently **no tagged release**, so nothing is
supported yet.

## What this tool touches

Worth stating plainly, because the surface is wider than a linter's:

- **It writes into `~/.claude/`.** `install` deploys the skill and creates two command entries.
  Those entries determine which tools a sweep session is granted, so a partial or tampered install
  has security consequences beyond a broken command: a skill that cannot be invoked is a skill
  whose `allowed-tools` never applies.
- **`update` fetches over the network.** It resolves a ref through the GitHub API and downloads a
  tarball over HTTPS from `codeload.github.com`. It extracts only `skill/`, rejects entries with
  traversing paths, skips symlinks and device entries, and bounds the read. It stages to a temp
  directory and installs from there, so a failed or truncated fetch cannot half-apply.
- **It runs `git` against a target repository**, read-only (`ls-files`, `diff --name-only`).
- **It never modifies the repository being swept.** The sweep method it supports is additive-only
  against the target tree by design.

## Install integrity

`@latest` resolves to whatever `HEAD` happens to be at install time; a semver tag resolves to a
fixed, reviewable commit. Both are supported — **pinning is recommended**, and semver tags are
published so that anyone who prefers to pin can. Release archives carry SHA-256 checksums; verify
them before running a downloaded binary.

## Known gaps

Recorded rather than implied.

- Skill assets fetched by `update` are **not signature-verified**. Integrity rests on HTTPS and on
  GitHub's control of the repository; a compromised repo or a forged TLS chain would serve prose
  that a sweep then treats as its class definitions.
- The tarball is trusted to the extent that its extraction is bounded and path-checked; there is no
  content policy on what the skill prose may say.

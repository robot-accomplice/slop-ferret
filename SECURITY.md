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
- **It runs `git` against a target repository.** The full set is `ls-files`, `diff --name-only`,
  `remote get-url origin`, `cat-file -e`, and `show -s --format=%cs`. This list was previously
  incomplete, and the omitted call — `remote get-url` — was the source of a path-traversal defect
  found in review: the origin URL became a filesystem path. **"Read-only" is not a security
  property here:** `git -C <repo>` honours the *target's* `.git/config`, so a hostile checkout is a
  config-driven execution surface regardless of which subcommand is run.
- **It never modifies the repository being swept.** The sweep method it supports is additive-only
  against the target tree by design.

## Install integrity

`@latest` resolves to whatever `HEAD` happens to be at install time; a semver tag resolves to a
fixed, reviewable commit. Both are supported — **pinning is recommended**, and semver tags are
published so that anyone who prefers to pin can. Release archives carry SHA-256 checksums; verify
them before running a downloaded binary.

## Known gaps

Recorded rather than implied.

- **Skill assets are not signature-verified, and the blast radius is larger than "bad class
  definitions".** The prose is installed as a Claude Code skill whose frontmatter grants
  `Bash` and `Write`, so an agent following it executes with those tools. The default source is a
  **git tag, which is mutable** — nothing in the binary pins an expected content digest. One
  compromised push, one retargeted tag, or one forged TLS chain therefore means attacker-authored
  instructions running with those grants on every machine that installs. Integrity currently rests
  entirely on HTTPS and on GitHub's control of the repository.
- **Archive extraction is bounded per entry but not in total.** `Fetch` caps each file and caps the
  compressed read, but does not cap the entry count or the total decompressed size, so a
  compromised tarball can exhaust disk before any error path fires.
- The tarball is trusted to the extent that its extraction is bounded and path-checked; there is no
  content policy on what the skill prose may say.

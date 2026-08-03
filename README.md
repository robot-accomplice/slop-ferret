<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/assets/ferret-mascot-dark.png">
    <img src="docs/assets/ferret-mascot.png" alt="Slop Ferret" width="220">
  </picture>
</p>

# slop-ferret

[![CI](https://github.com/robot-accomplice/slop-ferret/actions/workflows/ci.yml/badge.svg)](https://github.com/robot-accomplice/slop-ferret/actions/workflows/ci.yml)
[![Release](https://github.com/robot-accomplice/slop-ferret/actions/workflows/release.yml/badge.svg)](https://github.com/robot-accomplice/slop-ferret/actions/workflows/release.yml)
[![coverage ≥80%](https://img.shields.io/badge/coverage-%E2%89%A580%25%20enforced-brightgreen)](.github/workflows/ci.yml)
[![Go](https://img.shields.io/badge/go-1.26-00ADD8?logo=go&logoColor=white)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Ferrets AI slop out of a codebase — **work that looks finished and is not**: a feature nothing
calls, a test that cannot go red, a comment describing behaviour the code lacks, a guard whose
condition can never be true.

The name is the hunter, not the quarry.

## Why

A sweep for slop is mostly reading, and reading is what a model does. But a sweep also has a
mechanical half — *which files exist, which has anything looked at, what did the map say is
unreachable, how much of this repository has actually been read* — and that half was being done by
hand, in prose, differently each time.

Doing it by hand produced the failure this tool exists to prevent: a sweep that reported
`COMPLETE` having read 17 of 25 source files, because what it had completed was its own worklist
rather than the repository. The tool's job is to make that distinction impossible to misreport.

**The split: transforms belong in the tool, judgement belongs in the skill.** Enumerating files and
computing coverage fractions need no model. Deciding whether a finding clears its pre-filing bar
does — and no amount of Go will do it.

## Prerequisites

Three, and none of them are optional:

| | why | get it |
|---|---|---|
| **[magma](https://github.com/robot-accomplice/magma)** ≥ 0.2.0 | builds the call map `ferret plan` reads. There is no fallback: without a map, `plan` refuses. | `go install github.com/robot-accomplice/magma@latest` |
| **magma-rust-helper** | only for Rust targets. magma shells out to it for rust-analyzer name resolution; without it magma refuses with a message naming this binary. | `cargo install --path rust-helper` from a magma checkout |
| **[Claude Code](https://claude.com/claude-code)** | the skill half is a Claude Code skill. `ferret install` deploys into `~/.claude/`, and the sweep itself is run by an agent reading `SKILL.md`. The binary alone does not sweep anything. | — |

Rust maps are **slow**: measured 68 minutes for 834 files (rust-analyzer, single pass). Budget for
it, and reuse the map — `ferret plan` refuses a map of a different tree, so a stale one cannot
silently be reused.

## Install

Pinning is the recommended path — a pinned version is reproducible and reviewable, and semver tags
are published so anyone who prefers to pin can:

```bash
go install github.com/robot-accomplice/slop-ferret/cmd/ferret@v0.1.0   # pinned (recommended)
go install github.com/robot-accomplice/slop-ferret/cmd/ferret@latest   # tracks the latest tag
ferret install
```

`ferret install` is **required, not optional**: the H-signal vocabulary lives in the deployed
lexicon, not in the binary. Without it `ferret plan` refuses rather than handing back an empty
worklist. Run `ferret doctor` afterwards — it checks the deployment on its own, with no network.

**macOS and Linux only.** Windows builds are not published: `install` creates symlinks, which needs
privilege or developer mode on Windows, and no Windows path in this tool has ever been executed.
Publishing a binary nobody has run is the failure class this project exists to find.

`install` deploys the skill into `~/.claude/skills/slop-ferret/` and writes **both** command
entries (`/slop-ferret` and `/slop-ferret:report`).

## Usage

```bash
magma --depth 1 <repo> <name> ~/.slop-ferret/maps       # build the call map first
ferret plan ~/.slop-ferret/maps/<name> <sha> <repo> [--since <ref>] > plan.json
#   ... run the sweep, write discharge.json ...
ferret enumerate plan.json discharge.json             # 0 accounted · 3 items open
ferret report plan.json discharge.json findings.json report.html
```

| command | does |
|---|---|
| `plan` | reads the magma map, raises candidates with their pre-filing bars, enumerates the family-H worklist **and its complement** |
| `enumerate` | reports two attested fractions and a work queue |
| `report` | renders the sweep page; every figure derived from the plan and the discharge, never typed |
| `install` / `update` | synonyms — acquire the skill and deploy it |
| `doctor` | drift between the deployed skill and its source, in both directions |
| `records` | prior sweeps of a repository, newest first |

### Two fractions, no verdict word

```
attested.repo   production source files read / total     "was the repo covered"
attested.plan   items dispositioned / items raised       "was the plan worked through"
```

They are different numbers and **the gap between them is the point**. A
`COMPLETE / PARTIAL / INCOMPLETE` verdict was removed because one token cannot carry two
quantities: a real sweep scored 10/10 on the plan and 17/25 on the repo and reported COMPLETE,
having never enumerated the highest-consequence file in the tree.

| exit | means |
|---|---|
| 0 | nothing raised is undispositioned |
| 2 | misuse — wrong arity, unreadable file |
| 3 | items still open — read the work queue |
| 4 | a refusal — wrong tree, unknown contract, missing map. **Nothing was measured.** |

`3` and `4` are separate because a script must be able to tell an unfinished sweep from a map of
the wrong tree; those want opposite responses. None of them says whether the repository was
covered, because that is a fraction and a fraction does not fit in a byte.

### Waivers

A waiver settles the accounting and **does not** raise `attested.repo`. Deciding not to read a file
is a normal, correct move and costs nothing to record — but a waived file genuinely was not read,
and the fraction exists to tell you what you actually looked at. No coverage floor is enforced:
there is no defensible number, and a red build for reading 67% instead of 90% would only teach you
to waive to clear it.

## Two artifacts, two cadences

**The skill is not welded to the binary.** SKILL.md and the lexicon are prose; they change far more
often than the code, and usually for reasons the code does not care about.

**No prose is compiled into the binary.** That makes the two cadences structural rather than
conventional: a binary that cannot carry prose cannot quietly re-couple them, and what gets
deployed stays reviewable as files.

| source | command | who uses it |
|---|---|---|
| **repo @ this binary's version** | `ferret install` | the normal case: a downloaded binary, no checkout |
| **repo @ a ref** | `ferret install --ref main` | tracking a branch, or an older skill against a newer binary |
| **a checkout** | `ferret install --from .` | development |

The default is **self-pinning**: a `0.3.0` binary installs the `v0.3.0` skill — the prose that
version was tested with — without the user needing to know a ref exists. Before the first tag it
says so and names the alternatives rather than falling back to `HEAD`.

Releases also publish `slop-ferret-skill_<tag>.tar.gz` checksummed alongside the binaries: a
supported acquisition path, not a required one.

`doctor` reports the binary version and the deployed skill's version **separately**, with the
provenance of the deployed copy, and works with no source reachable. An install stages to a temp
dir first: a half-applied update is worse than a stale one.

## Sweep records

```bash
ferret enumerate plan.json discharge.json ~/code/target   # writes a record
ferret records ~/code/target                           # prior sweeps, newest first
```

Records live in `~/.slop-ferret/records/<repo>/<sha>.json`, never inside the repository being
swept. They carry a **computed** half the tool derives and an **attested** half the discharge
supplies — including classes checked clean *with the method used*, because "clean" with no method
is not checkable, and an unchecked clean is how a later sweep skips ground nobody covered.

Writing refuses a sha that does not resolve: a boundary nobody can re-derive leaves the next sweep
unable to scope itself.

## Symbiosis: magma, architext, slop-ferret

Three tools over **one** derived artifact. magma parses the repository once and emits a versioned
call graph; the other two consume it rather than re-deriving it.

```
                 ┌───────────────────────────────┐
   repository ──►│ magma  — parse once           │
                 │  reachability (RTA), dead code│
                 │  test-only code, call graph   │
                 └──────┬─────────────────┬──────┘
      codemap-rows/1    │                 │   magma-code-graph/1
                        ▼                 ▼
                 ┌──────────────┐   ┌──────────────┐
                 │ slop-ferret  │   │  architext   │
                 │ audit order, │   │ architecture │
                 │ coverage     │   │ docs, C4     │
                 └──────────────┘   └──────────────┘
```

**The contracts are the seam and they are not interchangeable:** `codemap-rows/1` (row files — the
only one this accepts), `codemap-graph/1` (`graph.json`), `magma-code-graph/1` (the architext
emit). `plan` refuses a map whose `contract_version` it does not know, and refuses a map of a
different tree by `sha`, so a stale map fails loud rather than seeding rows from the wrong commit.

**The signal vocabulary is a reading-order hint, not a completeness signal.** Measured across five
real repositories on 2026-08-02: **59% label precision**, 20% of production files matched, and
**0-of-6 recall on the files that actually produced findings** — including the one this tool exists
because of. It guesses semantics from names the *target's* authors chose, so it works when they
happen to have used one of its words and not otherwise.

What makes that safe is not the vocabulary's quality. It is that **a file no signal reaches is
reported as unread, never as clean** — the ranking can be wrong without the report becoming wrong.

**It is part of the lexicon** (`skill/references/ai-slop-lexicon.md`), not the binary — the tables
there define what a class *is*, the signals define where it tends to *live*, and one `version:`
now covers both. So it iterates from usage without a binary release: add a word after a sweep that missed something, reinstall the skill,
done. Extend per-repo with a `.slop-h-signals` file in the target, same `reason: regex` format. It
is expected to improve by accumulation, and the tier split it feeds is pinned by a committed fixture
so a change to it is a deliberate re-measurement rather than a silent drift.

**Where consequence ranking is going.** Into magma, which holds the call graph and the types:
*"does this file reach `os/exec`, `net/http`, `os.OpenFile`, `crypto/*`"* is a graph query over
imports and a real signal. No vocabulary of names will ever be one.

See [`docs/architecture/`](docs/architecture/) for C4 diagrams and the dataflow walkthrough.

## Development

```bash
just            # list recipes
just ci         # full local validation — the same gates as GitHub Actions
just test       # go test ./... -race
just cover      # coverage + the 80% gate
just doctor     # is the deployed skill in sync with this checkout?
```

Requires Go 1.26 and [`just`](https://github.com/casey/just); `golangci-lint` for `just lint`.

### Correctness

- **Coverage is gated at 80%** in CI and in `just cover`.
- **Deploying the checkout's skill tree is a build gate.** A deployed skill missing its lexicon
  produces a sweep with no vocabulary — indistinguishable from a real one, and not one.
- **Stale prose is a build gate.** `TestNoStaleCommandNamesOrRemovedFeatureClaims` scans every
  tracked file for command names that no longer exist and claims about removed features. Prose
  describing behaviour the code lacks is this tool's own subject, and it shipped here four times in
  one day because each manual sweep covered fewer places than the last. When it was first written it
  found seven more instances, in files a by-hand pass had just been declared clean.
- **The release gate parses `--version` positionally**, so both field offsets are pinned by a test.
  Reword that line and the tag check silently starts comparing the wrong token, which is how a
  release stops being verified without anything going red.
- Constants in `internal/gate` (signal anchoring, the tier split, the defer floor) were each
  **measured against a real repository**; the measurements live in comments beside the tests that
  pin them, because a refactor is the easiest place to lose one.

## Releasing

Tags exist so that users who want to pin can. Pushing a semver tag re-runs the CI gates, verifies
the tag matches the binary version, checks the skill stamp is date-style and was bumped if `skill/`
changed, cross-compiles every target, and publishes a GitHub Release with checksummed archives —
including the skill tree as its own artifact.

### Release checklist

1. `just ci` green locally.
2. Bump `binVersion` in `main.go` to the version being released.
3. `just release-dry vX.Y.Z` — confirm the archives build.
4. Land on `main`; run the [ABORT](docs/releases/) go/no-go gate.
5. `git tag vX.Y.Z && git push origin vX.Y.Z`.
6. Confirm the Release workflow published assets and checksums.

## Licence

[MIT](LICENSE) -- Copyright (c) 2026 Jonathan Machen

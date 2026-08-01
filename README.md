# slop-ferret

[![CI](https://github.com/robot-accomplice/slop-ferret/actions/workflows/ci.yml/badge.svg)](https://github.com/robot-accomplice/slop-ferret/actions/workflows/ci.yml)
[![Release](https://github.com/robot-accomplice/slop-ferret/actions/workflows/release.yml/badge.svg)](https://github.com/robot-accomplice/slop-ferret/actions/workflows/release.yml)
[![coverage ≥80%](https://img.shields.io/badge/coverage-%E2%89%A580%25%20enforced-brightgreen)](.github/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/robot-accomplice/slop-ferret)](https://goreportcard.com/report/github.com/robot-accomplice/slop-ferret)
[![Go](https://img.shields.io/badge/go-1.26-00ADD8?logo=go&logoColor=white)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Ferrets AI slop out of a codebase — **work that looks finished and is not**: a feature nothing
calls, a test that cannot go red, a comment describing behaviour the code lacks, a guard whose
condition can never be true.

The name is the hunter, not the quarry.

> **Status — not usable yet.** There is no tagged release, and the current
> [ABORT record](docs/releases/) is a NO-GO. The sections below describe the intended shape; the
> abort record says exactly which of it is not yet true.

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

## Install

**Pin a version.** `@latest` resolves to whatever `HEAD` happens to be, which means installing an
unreviewed commit by default:

```bash
go install github.com/robot-accomplice/slop-ferret@v0.1.0   # no tag published yet — see Status
slop-ferret install
```

Until a tag exists, build from source:

```bash
git clone https://github.com/robot-accomplice/slop-ferret.git
cd slop-ferret && just install && slop-ferret install
```

`install` deploys the skill into `~/.claude/skills/slop-ferret/` and writes **both** command
entries (`/slop-ferret` and `/slop-ferret:report`).

## Usage

```bash
magma --depth 1 <repo> <name> ~/.slop-ferret/maps       # build the call map first
slop-ferret plan ~/.slop-ferret/maps/<name> <sha> <repo> [--since <ref>] > plan.json
#   ... run the sweep, write discharge.json ...
slop-ferret verify plan.json discharge.json             # 0 settled · 3 items open
```

| command | does |
|---|---|
| `plan` | reads the magma map, raises candidates with their pre-filing bars, enumerates the family-H worklist **and its complement** |
| `verify` | reports two coverage fractions and a work queue |
| `update` | pulls the skill from this repo at a ref |
| `install` | deploys a skill tree (embedded, or `--from <dir>`) |
| `doctor` | drift between the deployed skill and this binary, in both directions |

### Two fractions, no verdict word

```
coverage.repo   production source files read / total     "was the repo covered"
coverage.plan   items dispositioned / items raised       "was the plan worked through"
```

They are different numbers and **the gap between them is the point**. A
`COMPLETE / PARTIAL / INCOMPLETE` verdict was removed because one token cannot carry two
quantities: a real sweep scored 10/10 on the plan and 17/25 on the repo and reported COMPLETE,
having never enumerated the highest-consequence file in the tree.

The exit code carries bookkeeping only — 0 when nothing raised is still undispositioned, 3 when
something is — the way a test runner reports outstanding failures. It says nothing about whether
the repository was covered, because that is a fraction and a fraction does not fit in a byte.

### Waivers

A waiver settles the accounting and **does not** raise `coverage.repo`. Deciding not to read a file
is a normal, correct move and costs nothing to record — but a waived file genuinely was not read,
and the fraction exists to tell you what you actually looked at. No coverage floor is enforced:
there is no defensible number, and a red build for reading 67% instead of 90% would only teach you
to waive to clear it.

## Two artifacts, two cadences

**The skill is not welded to the binary.** SKILL.md and the lexicon are prose; they change far more
often than the code, and usually for reasons the code does not care about.

| source | command | when |
|---|---|---|
| **embedded** | `slop-ferret install` | bootstrap floor — offline, what a fresh install has before it has talked to anything |
| **repo** | `slop-ferret update [--ref v0.1.0]` | the live `skill/` tree, pinned to the commit the ref resolved to |
| **dir** | `slop-ferret install --from .` | the edit-build-install loop |

`version` and `doctor` print the binary version and the skill version **separately**, with the
provenance of the deployed copy. An update stages to a temp dir first: a half-applied update is
worse than a stale one, and a failed fetch leaves the deployment untouched.

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

**Where the division is going.** Ranking by consequence belongs in magma, not here: *"does this
file reach `os/exec`, `net/http`, `os.OpenFile`, `crypto/*`"* is a graph query over imports, and
magma already holds the graph and the types. The path-name signals in this repo guess semantics
from names the target's own authors chose, which is why they under-enumerate silently.

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
- **The embedded skill's completeness is a build gate.** A deployed skill missing its lexicon
  produces a sweep with no vocabulary — indistinguishable from a real one, and not one.
- **The release gate parses `--version` positionally**, so both field offsets are pinned by a test.
  Reword that line and the tag check silently starts comparing the wrong token, which is how a
  release stops being verified without anything going red.
- Constants in `internal/gate` (signal anchoring, the tier split, the defer floor) were each
  **measured against a real repository**; the measurements live in comments beside the tests that
  pin them, because a refactor is the easiest place to lose one.

## Releasing

Tags are the only supported install path. Pushing a semver tag re-runs the CI gates, verifies the
tag matches the binary version **and** stamps the embedded skill, cross-compiles every target, and
publishes a GitHub Release with checksummed archives.

### Release checklist

1. `just ci` green locally.
2. Bump `binVersion` in `main.go` to the version being released.
3. `just release-dry vX.Y.Z` — confirm the archives build.
4. Land on `main`; run the [ABORT](docs/releases/) go/no-go gate.
5. `git tag vX.Y.Z && git push origin vX.Y.Z`.
6. Confirm the Release workflow published assets and checksums.

## Licence

[MIT](LICENSE) -- Copyright (c) 2026 Jonathan Machen

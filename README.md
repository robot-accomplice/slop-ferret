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
[![Go Report Card](https://goreportcard.com/badge/github.com/robot-accomplice/slop-ferret)](https://goreportcard.com/report/github.com/robot-accomplice/slop-ferret)
[![Go](https://img.shields.io/badge/go-1.26-00ADD8?logo=go&logoColor=white)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Ferrets AI slop out of a codebase — **work that looks finished and is not**: a feature nothing
calls, a test that cannot go red, a comment describing behaviour the code lacks, a guard whose
condition can never be true.

The name is the hunter, not the quarry.

> **Status — NOT RELEASED. The go/no-go review returned NO-GO.**
> Five independent red-team stations all voted against shipping. Read
> [`docs/releases/v0.1.0-abort.md`](docs/releases/v0.1.0-abort.md) before relying on anything below.
>
> The headline reason: **`coverage.repo` is computed from a self-reported list of files and nothing
> corroborates it.** It is not a measurement of what was read. Treat every number this tool emits as
> a claim by whoever ran it, not as evidence.

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

Both paths work. **Pinning is recommended** — a pinned version is reproducible and reviewable, and
publishing semver tags exists so that anyone who prefers to pin can:

```bash
go install github.com/robot-accomplice/slop-ferret/cmd/ferret@v0.1.0   # pinned (recommended)
go install github.com/robot-accomplice/slop-ferret/cmd/ferret@latest   # tracks HEAD
ferret install
```

No tag is published yet, so today that means `@latest` or a source build:

```bash
git clone https://github.com/robot-accomplice/slop-ferret.git
cd slop-ferret && just install && ferret install
```

`install` deploys the skill into `~/.claude/skills/slop-ferret/` and writes **both** command
entries (`/slop-ferret` and `/slop-ferret:report`).

## Usage

```bash
magma --depth 1 <repo> <name> ~/.slop-ferret/maps       # build the call map first
ferret plan ~/.slop-ferret/maps/<name> <sha> <repo> [--since <ref>] > plan.json
#   ... run the sweep, write discharge.json ...
ferret verify plan.json discharge.json             # 0 settled · 3 items open
```

| command | does |
|---|---|
| `plan` | reads the magma map, raises candidates with their pre-filing bars, enumerates the family-H worklist **and its complement** |
| `verify` | reports two coverage fractions and a work queue |
| `install` / `update` | synonyms — acquire the skill and deploy it |
| `doctor` | drift between the deployed skill and its source, in both directions |
| `records` | prior sweeps of a repository, newest first |

### Two fractions, no verdict word

```
coverage.repo   production source files read / total     "was the repo covered"
coverage.plan   items dispositioned / items raised       "was the plan worked through"
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

A waiver settles the accounting and **does not** raise `coverage.repo`. Deciding not to read a file
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
ferret verify plan.json discharge.json ~/code/target   # writes a record
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
- **Deploying the checkout's skill tree is a build gate.** A deployed skill missing its lexicon
  produces a sweep with no vocabulary — indistinguishable from a real one, and not one.
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

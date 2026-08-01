# slop-ferret — design

**Date:** 2026-08-01 · **Status:** proposed, pending review · **Scope:** the `ferret` binary and
its relationship to the skill assets, magma, and the sweep method.

> **This spec is retrospective.** The code was written first, across a single session, without a
> design discussion. That produced rework that a spec would have caught: Python then Go, a rename
> after publishing, prose embedded in the binary then removed, a private repo then public, and two
> defects of the exact class the tool exists to find. This document records the design as it should
> have been agreed, marks which parts the built code already satisfies, and states what is
> deliberately not being built yet.

---

## 1. What this is

The sweep method (`slop-ferret`, the skill) hunts **work that looks finished and is not**. Most of
it is reading, and reading is what a model does. But it has a mechanical half — which files exist,
which has anything looked at, what the map says is unreachable, how much of the repository has
actually been read — and that half was being done by hand, in prose, differently each time.

`ferret` is that mechanical half, and nothing more.

**The split: transforms belong in the tool, judgement belongs in the skill.** Enumerating files and
computing coverage fractions need no model. Deciding whether a finding clears its pre-filing bar
does, and no amount of Go will do it.

### The failure that motivates it

A real sweep reported `COMPLETE` having read **17 of 25** source files. What it had completed was
its own worklist, not the repository — and the file that produced the sweep's most serious finding
(an unauthenticated localhost HTTP server making arbitrary outbound fetches) was never enumerated
at all. Coverage of the enumeration was being reported as coverage of the repository.

Everything below follows from making that specific misreport impossible.

---

## 2. Decisions

Settled in discussion. Each records who decided, because a spec that cannot be traced to a decision
is a spec nobody agreed to.

| # | Decision | Rationale |
|---|---|---|
| D1 | **One binary per project.** `ferret`, `magma`, `architext` are separate projects, one binary each. | Operator. Not a fleet of slop-ferret executables; equally not a merge — magma has other consumers. |
| D2 | **Binary is `ferret`**, project/repo is `slop-ferret`. | Operator. `ferret plan` reads better than `slop-ferret plan`. Requires `cmd/ferret/`. |
| D3 | **No skill or lexicon prose embedded in the binary.** | Operator. Makes the two cadences structural rather than conventional: a binary that cannot carry prose cannot quietly re-couple them. Also keeps what is deployed reviewable as files. |
| D4 | **`install` and `update` are synonyms.** | Operator. Both acquire prose and deploy it; they differ only in whether something is already there, which the tool can see for itself. |
| D5 | **Skill version stays date-style** (`YYYY-MM-DD.N`), never the binary's semver. | Operator. It labels prose moving on its own cadence; semver names something else. |
| D6 | **Semver tags are published so users can pin. `@latest` remains legitimate.** | Operator. Publishing tags makes the choice available; it does not remove one. |
| D7 | **Records store at `~/.slop-ferret/records/`.** | Operator ("seems like a good idea"). Design in §6. |
| D8 | **`--from` is required when no `--ref` is given.** | Author's call, overturnable. An implicit CWD default deploys a skill from whatever directory you happened to be standing in. |
| D9 | **Slash commands stay `/slop-ferret` and `/slop-ferret:report`.** | Author's call, overturnable. They name the skill, not the binary. |

---

## 3. Scope: this is a prerelease

**There are no tags and no releases, and the tool has not passed a ship review.** The design below
is scoped to what a prerelease actually needs. Mechanisms that only make sense once releases exist
are named in §8 with the reason they are deferred, not silently omitted.

The working path today is a checkout:

```bash
ferret install --from ~/code/slop-ferret
ferret plan … / ferret verify … / ferret doctor
```

---

## 4. Architecture

```
                 ┌───────────────────────────────┐
   repository ──►│ magma  — parse once           │
                 │  reachability (RTA), dead code│
                 └──────┬─────────────────┬──────┘
      codemap-rows/1    │                 │  magma-code-graph/1
                        ▼                 ▼
                 ┌──────────────┐   ┌──────────────┐
                 │   ferret     │   │  architext   │
                 └──────┬───────┘   └──────────────┘
                        │ deploys
                        ▼
              ~/.claude/skills/slop-ferret/  ──►  sweeping agent
```

Three tools over **one** derived artifact. The contracts are the seam and are not interchangeable:
`codemap-rows/1` (row files — the only one `ferret` accepts), `codemap-graph/1`,
`magma-code-graph/1`. `plan` refuses a map whose `contract_version` it does not know, and refuses a
map of a different tree by `sha`.

### Components

| package | responsibility | depends on |
|---|---|---|
| `cmd/ferret` | dispatch only — `run(argv, stdout, stderr) int` | both internals |
| `internal/gate` | `BuildPlan`, `Verify`, the measured constants, the coverage denominator | git, a magma map |
| `internal/install` | `Source`, `Fetch`, `Install`, `Doctor`, manifest | filesystem, network (fetch only) |

`main()` contains nothing that can be wrong except wiring a compiler already checks. The measured
constants live in one package so that changing one forces you past the comment recording what it
cost to learn.

**Not a component: a report generator.** The HTML report is authored against the spec in
`skill/commands/`. Judging severity and writing an honest narrative is the judgement half; keeping
the spec in the skill means it revises without a binary release.

---

## 5. The source model

`ferret` deploys a skill tree into `~/.claude/skills/slop-ferret/` plus **both** command entries.
With D3 there is no compiled-in copy, so a source must always be named:

| invocation | source | availability |
|---|---|---|
| `ferret install --from <dir>` | a checkout | now — the only path that works prerelease |
| `ferret install --ref <ref>` | repo tarball at a resolved commit | code exists; no caller until there is a tag |
| `ferret install` | — | **hard error** naming both options (D8) |

`update` is a synonym of `install` (D4).

**Both command entries, or neither.** `~/.claude/commands/slop-ferret/report.md` once existed while
`/slop-ferret` did not, so the parent skill could not be invoked, so its `allowed-tools` never
applied — and the withholding of `Edit` and `Artifact`, which the skill names as its runtime
enforcement of *additive-only* and *never-publish*, was prose with nothing behind it. A sweep ran
the whole method holding both tools it was meant to be denied. The two entries are therefore one
table with no code path that writes a subset, and `doctor` fails on a half-install.

**Refusing to clobber.** A deployed file that matches neither the source nor the hash this
installer last wrote was edited in place. `install` stops and prints what would be lost. This is
not a permission check — there is one user and nothing to defend against — it is *"you edited the
wrong copy"*.

**Two versions, reported separately.** The binary's semver and the skill's date-style stamp answer
different questions. `doctor` prints both plus the provenance of the deployed copy.

---

## 6. Records store (new)

**Problem.** The method says read the prior sweep record before starting (Step 0.2) and write one
at the end (Step 7). Neither works: there is no store, and the vault that held them is gone. The
skill currently references `~/.slop-ferret/records/` — prose ahead of code, in the tool that hunts
that.

**Location.** `~/.slop-ferret/records/<repo-key>/<sha>.json`. `repo-key` is the normalised `origin`
URL (`github.com/robot-accomplice/ghola`), falling back to a hash of the absolute path for
remoteless repos. Never inside the target repo.

**Contents — two kinds of field, and the split is the point:**

- **Computed** — sha, date, `coverage.repo`, `coverage.plan`, denominator, waived count, worklist
  and complement sizes, binary/skill/lexicon versions.
- **Attested** — tier, families not run, checked-clean *with the method used*, near-misses, finding
  counts, report file path. Supplied via optional `discharge.json` fields.

The tool records what it computes and what the sweep attests. It never invents the second kind.

**One gate at write time:** verify the sha resolves in the target repo before writing. Two prior
sweeps recorded boundaries that no longer exist as objects, both taken from dirty maps, which made
their denominators unreproducible.

**Commands:**

```bash
ferret verify plan.json discharge.json     # writes a record by default
ferret verify … --no-record                # suppress
ferret records <repo>                      # prior sweeps, newest first
```

Always-write with an opt-out: a record you must remember to request is one that will not exist when
the next sweep looks for it.

**Not building:** no index, no pruning, no query language, no cross-repo comparison table. Records
are small JSON and `records --last` feeding Step 0.2 is the only read path with a caller.

---

## 7. Error handling, and what the exit code means

`verify` reports **two fractions and no verdict word**:

```
coverage.repo   production source files read / total     "was the repo covered"
coverage.plan   items dispositioned / items raised       "was the plan worked through"
```

`COMPLETE / PARTIAL / INCOMPLETE` was removed because one token cannot carry two quantities. The
exit code carries bookkeeping only — `0` when nothing raised is undispositioned, `3` when something
is, `2` for misuse, `3` for a refusal — the way a test runner reports outstanding failures. It says
nothing about whether the repository was covered, because that is a fraction and a fraction does
not fit in a byte.

**Waivers settle the accounting and never raise `coverage.repo`.** Choosing not to read a file is a
normal move and costs nothing to record, but a waived file genuinely was not read. **No coverage
floor is enforced:** there is no defensible number, and a red build for reading 67% instead of 90%
would only teach the operator to waive to clear it.

**This is an instrument reading, not a score.** One user, no adversary, nothing to defend against.
Outputs are a work queue and an honest measurement.

**Refusals fail loud:** unknown `contract_version`, a map sha that is not the pinned sha (a dirty
map stamps `<sha>+<diffhash>` and can never match, by construction), a missing required map file, a
non-repo target. A failed fetch leaves the deployment untouched and says so.

---

## 8. Deferred, with reasons

Every deferral states why, in terms of risk, dependency or sequencing.

| deferred | why |
|---|---|
| Fetch-at-a-ref as the documented path | Code exists and is tested, but it has **no caller until a tag exists**. Kept reachable via `--ref`; not documented as the primary path. |
| Default-ref-derived-from-binary-version | Designing the mechanism before the thing it mechanises exists. Meaningless with zero tags. |
| Consequence ranking by sink-reachability | **Belongs in magma**, which holds the graph and the types. `ferret`'s path-name signals guess semantics from names the target's own authors chose, which is why they under-enumerate silently. Cross-repo change; needs its own spec. |
| Signature verification of fetched skill assets | Depends on the fetch path having users. Recorded in `SECURITY.md` as a known gap rather than implied to be solved. |
| Family D / E map seeding | magma emits no `_duplicates.json` (deliberately — it has no notion of similarity, and a false duplicate row is a refactor order for code that should be left alone) and no `_interfaces.json` yet. Reported as NOT SEEDED so a missing input cannot read as a clean family. |

---

## 9. Testing

- **Coverage gated at 80%**, in CI and `just cover`. Currently 82%.
- **Differential testing was used for the Python→Go port** and is the standard for any future
  reimplementation: both ran over a real repository and were compared field by field before the
  original was deleted (`plan` matched 17/18 fields, `verify` 23/23 with the same exit code).
- **Measured constants are pinned by tests carrying their measurements.** A vocabulary change once
  took a repo from 10 required paths to 1 while every test stayed green; the unit tests cannot see
  that, so changing a constant in `internal/gate` requires re-measuring against a real repository.
- **The release gate parses `--version` positionally**, so field offsets are pinned by a test.
- **Deployment is tested against a synthetic skill tree**, not the real one, so these tests do not
  fail because someone edited `SKILL.md`.

---

## 10. Delta from the built code

What already satisfies this spec, and what does not.

| | state |
|---|---|
| `plan` / `verify`, two fractions, complement enumeration | **built** |
| `Source` abstraction, `Doctor` drift, refuse-to-clobber, both command entries | **built** |
| CI, release workflow, lint, docs, 82% coverage | **built** |
| D2 — binary named `ferret` at `cmd/ferret/` | **not built** — currently `slop-ferret` from repo root |
| D3 — no embedded prose | **not built** — `//go:embed all:skill` still present |
| D4 — `install`/`update` synonyms | **not built** — separate subcommands |
| D8 — hard error with no source | **not built** — currently falls back to embedded |
| D7 — records store | **not built** |
| `--version` reporting a skill version | **to remove** — the binary will not have one |
| `release.yml` reading the skill stamp from `--version` | **to change** — must read `skill/VERSION` from the checkout |
| CI "embedded skill is complete" job | **to change** — becomes `install --from .` |

---

## 11. Open

- **Nothing blocking.** D8 and D9 are author's calls and are marked overturnable.
- **Exit-code split (§7)** — proposed, not yet agreed. Found by this spec's own self-review, which
  is the first thing in this project that a design document caught before the code shipped.
- The go/no-go ship review has not been run. Nothing here should be read as a statement that the
  tool is ready; §3 is explicit that it is not.

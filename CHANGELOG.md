# Changelog

All notable changes to this project are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
this project will use [semantic versioning](https://semver.org/) once it has a release.

## [Unreleased]

**Nothing is released. Two go/no-go ship reviews have been run and both returned NO-GO** — see
[`docs/releases/v0.1.0-abort.md`](docs/releases/v0.1.0-abort.md) for the first. The second (five
stations, 2026-08-02) found the remediation for the first had not bound: the tests written to pin
its two headline defects both still passed under mutation of those exact defects.

Every fix listed below was verified by breaking it and observing the test fail. That is the
process change, not a claim about care: twice a remediation was reported complete while its
guards did not bind, both times because the guard was written from the diff rather than from the
failure mode.

### Changed
- **`ferret report` takes the plan and the discharge.** `ferret report <plan.json>
  <discharge.json> <findings.json> <out.html>`. Every coverage figure is derived by the same code
  `ferret enumerate` runs; the findings file has no field to type one into and is refused if it
  carries the retired ones. The previous single-file form had never worked at all — the renderer
  read flat `attested_repo` while `enumerate` emits nested `attested: {repo}`.
- **`ferret discharge` removed.** <!-- staleprose:allow --> Measured 3.2x worse than hand-writing (12,040 bytes against
  3,778), because the skill grants `Write` and not `Edit`, and one `jq` filter reproduced its
  output byte-for-byte.
- **Records are keyed on root commits, not `origin`.** The origin URL is configuration and is
  asserted by the audited repo, so the store both lost history across checkouts and accepted
  records written by any repository claiming another's URL. Records now carry `schema`; one from
  before this is reported as unreadable rather than rendered with blank figures.
- **Windows builds are no longer published** — no Windows path in this tool has ever run.

### Fixed
- `ferret plan` refuses an empty or uncompilable H vocabulary instead of exiting 0 with an empty
  worklist, which is what an uninstalled skill looked like and read identically to a clean repo.
- `ferret doctor` checks the deployment without needing a source: it reported `ok` with the
  lexicon deleted, the default path for a `go install`ed binary offline.
- `ferret install` creates all command entries before writing anything and rolls back on failure.
  Three shapes could previously leave the skill deployed with one entry linked and the other
  missing — which means the skill's `allowed-tools` never apply.
- Unknown finding severities are refused rather than ranked; one used to sort as the most severe
  while rendering as the least.
- `.slop-h-signals` is bounded (500 signals / 256 KiB): it comes from the audited repo and
  matching is O(files x signals).
- Plans record `vocab_provenance`, so a sweep run against a half-loaded lexicon is distinguishable
  after the fact from one over a repo that genuinely matched nothing.

### Added
- **Sweep records** — `ferret enumerate … <repo>` writes `~/.slop-ferret/records/<repo>/<sha>.json`;
  `ferret records <repo>` reads them back. Carries checked-clean *with the method used*, and
  refuses a sha that does not resolve.
- `plan` / `enumerate` — the coded seam between a magma code map and a sweep. Reports two coverage
  fractions (`attested.repo`, `attested.plan`) and a work queue.
- `h_unmatched` — the complement of the signal-matched worklist. Every production file is raised;
  signals only rank. Closes the gap where a file no signal reached was indistinguishable from a
  file that had been cleared.
- `install` / `doctor` — deploy the skill and report drift in both directions, distinguishing
  "the binary moved on" from "you edited the deployed copy".
- `update` — pull skill assets from the repository at a ref, resolved to a commit, staged to a temp
  dir so a failed fetch cannot half-apply.
- CI (build, vet, race tests, 80% coverage gate, golangci-lint, skill-tree deployment) and a
  tag-driven release workflow that verifies the tag against the binary version and the skill stamp.
- Architecture docs (C4 context, C4 component, dataflow), CONTRIBUTING, SECURITY.

### Changed
- **The binary is `ferret`** (project stays `slop-ferret`), built from `cmd/ferret`.
- **`install` and `update` are synonyms.**
- **No skill or lexicon prose is compiled into the binary.** `install` acquires it from the
  repository at the tag matching the binary's own version, a `--ref`, or a `--from` checkout.
- **Exit code `4` introduced for refusals**, separating "the tool declined to run" from "the sweep
  is not finished" — they had shared `3`, so a script could not tell them apart.
- **Renamed from `slop` to `slop-ferret`.** The old name named the quarry; a tool called "slop"
  reads as a slop generator.
- **Removed the `COMPLETE / PARTIAL / INCOMPLETE` verdict.** One token cannot carry two
  quantities: a real sweep scored 10/10 on the plan and 17/25 on the repo and reported COMPLETE.
- **Ported from Python to Go**, verified differentially against the retiring implementation over a
  real repository before the original was deleted (`plan` matched on 17 of 18 fields, `verify` on
  all 23 with the same exit code).
- **Unwelded the skill from the binary.** Skill and binary carry separate versions; a lexicon edit
  no longer needs a rebuild to reach a sweep.
- Excluded vendored and generated trees from the coverage denominator, which the skill had always
  mandated and the code had never done.

### Fixed

- **The dirty-map refusal had never once fired.** It compared `sha`; magma puts the marker in
  `tree`. The safety property the whole pinned-SHA discipline rests on was prose for its entire
  life, and a unit test had asserted the wrong behaviour using a fixture shape magma never produces.
- **A refused map read as a clean one.** magma distinguishes `rows: null` (could not run) from
  `rows: []` (ran, found nothing); the gate discarded the distinction, so a refusal produced a plan
  with zero candidates and family A absent from `unseeded_families`.
- **Arbitrary file write outside the records root.** The audited repository's `origin` URL was used
  as a path key with no containment check; `filepath.Join` cleans after joining, so `..` escaped.
  Escalated in review to overwriting a file in `~/.claude`.
- **Records persisted claims from sweeps that established nothing** — a run that read zero files and
  exited 3 could record two classes clean with an empty method, which the next sweep was told to
  trust.
- **`install` destroyed a user's own `~/.claude/commands/slop-ferret.md`** without warning, and a
  refused install left the tree deployed but unmanifested, so the *next* install accused the user of
  editing files ferret had written itself.
- **The fidelity table carried four values magma never emits** and lacked `semantic`, so every Rust
  candidate was labelled weakest-evidence. **`limitations` was parsed away** despite magma's contract
  naming this gate as the consumer it exists for. **`plan.Contract` was written and never read.**
- **A record failure discarded the enumerate result** and reported exit 2 (misuse) for a sweep that
  was merely unfinished.
- **`pct` rounded**, so 1999/2000 rendered as `100.0%`.
- **Stale prose in `internal/` and `cmd/`** — including the `instructions` string emitted into every
  `plan.json`, telling agents to run a command that does not exist. Now a build gate.
- `update` recorded provenance as `repo@main (main)` — it read the "sha" from the archive's
  top-level directory, which for a branch is named after the branch. The recorded resolution was a
  restatement of the input, and the comment claiming otherwise was a fabricated claim shipped
  inside the updater. Now resolves the ref through the commits API and downloads that commit.

### Removed
- The Python implementation and its digest-stamping script. Version control replaces both.
- The Obsidian vault dependency. The lexicon was symlinked into a note app so wikilinks resolved,
  which made the vault part of the tool's correctness and left two copies of one definition under
  one name — the drift class this tool hunts.

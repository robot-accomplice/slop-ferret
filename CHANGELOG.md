# Changelog

All notable changes to this project are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
this project will use [semantic versioning](https://semver.org/) once it has a release.

## [Unreleased]

**Nothing is released. The current [ABORT record](docs/releases/) is a NO-GO.**

### Added
- `plan` / `verify` — the coded seam between a magma code map and a sweep. Reports two coverage
  fractions (`coverage.repo`, `coverage.plan`) and a work queue.
- `h_unmatched` — the complement of the signal-matched worklist. Every production file is raised;
  signals only rank. Closes the gap where a file no signal reached was indistinguishable from a
  file that had been cleared.
- `install` / `doctor` — deploy the skill and report drift in both directions, distinguishing
  "the binary moved on" from "you edited the deployed copy".
- `update` — pull skill assets from the repository at a ref, resolved to a commit, staged to a temp
  dir so a failed fetch cannot half-apply.
- CI (build, vet, race tests, 80% coverage gate, golangci-lint, embedded-skill completeness) and a
  tag-driven release workflow that verifies the tag against the binary version and the skill stamp.
- Architecture docs (C4 context, C4 component, dataflow), CONTRIBUTING, SECURITY.

### Changed
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
- `update` recorded provenance as `repo@main (main)` — it read the "sha" from the archive's
  top-level directory, which for a branch is named after the branch. The recorded resolution was a
  restatement of the input, and the comment claiming otherwise was a fabricated claim shipped
  inside the updater. Now resolves the ref through the commits API and downloads that commit.

### Removed
- The Python implementation and its digest-stamping script. Version control replaces both.
- The Obsidian vault dependency. The lexicon was symlinked into a note app so wikilinks resolved,
  which made the vault part of the tool's correctness and left two copies of one definition under
  one name — the drift class this tool hunts.

# Handover — 2026-08-03

**Delete this file when the work it describes is finished.** It is a working note between
sessions, not a permanent document.

## Start here

```bash
cd ~/code/slop-ferret
git branch --show-current      # feature/abort-ii-binding-tests
git rev-list --count develop..HEAD   # 14 (including this note)
just ci                        # must exit 0
```

Nothing is pushed. 14 commits sit on `feature/abort-ii-binding-tests`, local only. `just ci` is
green at 86.2% coverage. No release, no tag.

Read `docs/releases/v0.1.0-abort.md` before anything else — it is the first of three go/no-go ship
reviews, all of which returned **NO-GO**.

## The one rule that matters

**A fix is not done when the code changes. It is done when a test fails without it.**

Write the test, then *break the thing it guards and watch it go red* — not a similar test, that one.
`CONTRIBUTING.md` has the full version and two named failure shapes that actually shipped here.

```bash
just mutate ./internal/gate/     # ~35 min, ~240 mutants
just mutate ./internal/install/  # ~2 min
```

This is a pre-release step, not a CI step. It needs
`go install github.com/go-gremlins/gremlins/cmd/gremlins@latest`.

**Triage survivors; do not chase the number.** Some are equivalent mutants (`n++` where `n` is only
compared to zero). Some are display-only. Record which. An untriaged efficacy percentage is exactly
the kind of figure this project exists to distrust — reporting one would be the same defect in a new
place.

## The failure pattern — read this before fixing anything

Three reviews found the same thing three times. Each round, the guards written to close the previous
round did not bind, because each was written **from the diff rather than from the failure mode**:

| Round | Guarded | Left open |
|---|---|---|
| I | the four stale strings just fixed | the class |
| II | the one install shape imagined | two other shapes |
| III | `findings.json`, the file being edited | `plan.json` / `discharge.json`, which supply every figure |
| III | one hand-picked mutation per fix | 21 others, found by the mechanical set |

It recurred *after* being named, and after tooling was built against it. The mechanical mutation set
then caught it a fourth time: three of four fixes certified "verified by mutation" still had a live
survivor, because gremlins flipped a *different* operator on the same line than the one imagined —
`+` → `−` on a term that happened to be zero in the chosen fixture.

**Practical consequence: when you fix something below, do not only fix the instance named. Ask what
class it belongs to, and check the other members.**

## Verified green

- Three Review III blockers closed, each mutation-checked: the complement clause (the enforcement of
  the founding ghola defect) is now tested with its counterfactual; `0/0` is refused; the plan
  contract is mandatory.
- `FromSweep` asserted field by field (was 0.0% coverage, 10 of 12 derived fields replaceable by
  constants).
- `gate.CheckableMethod` is one shared rule for the record and the page; rejects `-` / `n/a`.
- A record cannot claim a family both checked-clean and not-run.
- Records key on root commits, carry `schema`, and report both legacy locations.
- `install` links first and rolls back — three shapes tested.
- `doctor` checks the deployment with no source reachable.
- Mutation counts, `internal/gate`, three measured runs: 191/50/45 → 202/40/44 → 206/37/43
  (killed / lived / not-covered).

## Open work, in priority order

Every item below was re-verified on 2026-08-03, not recalled.

### 1. `ferret plan` rejects a full-length sha — BLOCKING

`internal/gate/gate.go:475` is `if doc.SHA != pinnedSHA` — raw string equality. `git rev-parse HEAD`
(40 chars) and 8 chars both exit 4; only magma's 7-char abbreviation works. The error blames the map
and says "regenerate the map at `<40-char>`", which magma can never produce — a reviewer ran the
prescribed remedy and proved the loop is infinite.

Worse: `internal/gate/magma_integration_test.go:51` uses `rev-parse --short HEAD`. **The one test
that runs real magma is written around the bug.** `README.md:99` never mentions `--short`, so a user
following the README cannot get past step 2.

Fix by comparing on the shorter length (abbreviation-tolerant), not by documenting `--short`.

### 2. `README.md:71` sends users to `develop` — BLOCKING

`develop` still has `ferret discharge` and the 2-arg `report`. The install section rewritten to fix
"no documented path works" now points at a binary whose command set contradicts the README that sent
them there. `CONTRIBUTING.md:19` has the same defect. Neither can be right until this branch merges.

### 3. The report's own documented guarantees are unreachable

`FromSweep` reads `tier`, `near_misses` and `checked_clean` from the discharge. Those three field
names appear in **no** artifact an auditor reads — verified: zero occurrences in `README.md`,
`skill/SKILL.md`, and `skill/commands/slop-ferret-report.md`. Meanwhile that command file lists
"near-misses are shown" and "checked-clean carries its method" as guarantees. Both are unsatisfiable
from any documented input.

Deleting `ferret discharge` removed the only machine-readable definition of the discharge shape. The
`instructions` string in `plan.json` documents the enumerate-facing fields only.

### 4. Regressions introduced by this round's own changes

- `SECURITY.md:25` claims a "full set" of five git subcommands. The root-commit change added
  `rev-list --max-parents=0` and `rev-parse --is-shallow-repository`. The sentence narrating a
  security defect *caused by an incomplete list* is itself short by two.
- `justfile` still builds `windows/amd64` (3 references) although `release.yml` no longer does, so
  `just release-dry` — the release checklist's verification step — checks an asset set that will
  never ship and skips the one that will. `release.yml` also retains `$os = windows` guards whose
  condition can never be true.

### 5. `vocab_provenance` is written and read by nothing

Verified: zero occurrences in `internal/gate/record.go` and `internal/report/report.go`. It is on the
plan only. Its doc comment says it exists so a half-loaded lexicon is distinguishable from a repo
that genuinely matched nothing — which it cannot do, because nothing reads it. **That is a comment
describing behaviour the code lacks, in the field added to fix that class.** Carry it into `Record`
and onto the page, and derive the page's lexicon label from it instead of `Authored.LexiconVer`.

### 6. Integrity items still surviving from Review III

- `plan.json` and `discharge.json` are `json.Unmarshal`ed with no `DisallowUnknownFields` and no
  required-field check. The contract is now mandatory, which raises the cost, but a hand-written plan
  still drives everything. The durable fix is for `plan` to stamp a digest over its own content and
  `enumerate` to require it.
- Waiving still inflates `attested.plan` to 100% (the waived count is surfaced, which is a real
  mitigation, but the figure is unchanged).
- A zero-findings sweep produces no warning — `internal/report/report.go:119` fires only on
  `verified == 0 && suspected > 0`.
- `FamiliesRun` is model-supplied (`report.go:193`) while `FamiliesNot` is derived, so a page can
  print every family as both run and not-run.
- `ferret report` exits 0 on an incomplete accounting. The page warns; a wrapper script is not told.

### 7. Mutation survivors not yet worked

37 in `internal/gate`, 7 in `internal/install`, triaged as display assembly, a truncation cap, date
extraction, and unreachable defensive guards. One real gap recorded: `resolveRef`'s response
validation is untested because it does unmockable network I/O — closing it needs injection.

## Environment

- **magma 0.2.0** must be on PATH. Tests **fail** rather than skip without it, on purpose.
  `just check-deps` reports it.
- **magma-rust-helper** is needed only for Rust targets: `cargo install --path rust-helper` from a
  magma checkout. A Rust map is slow — measured 68 min for 834 files, so it can never run in CI. The
  committed fixtures `internal/gate/testdata/real-magma-rust-*.json` are verbatim magma output from
  `roboticus-rust @ 7e5f0d6d`; regeneration is documented only in a test comment, which is a gap.
- **gremlins** needs `--timeout-coefficient 20`. At the default, 22 of 32 mutants report "timed out",
  which is indistinguishable from surviving. `just mutate` sets this.
- `python3` is hook-intercepted in this environment — use `uv run python`.
- **Do not pipe `go test` or `just ci` to `tail`** — it masks the exit code. That mistake pushed a red
  branch once and committed a failing test once, both in this project.
- `zsh` does not word-split unquoted variables; `for f in $FILES` iterates once over the whole
  string. Use `while IFS= read -r`.

## Constraints

- **Nothing is pushed. Do not push, open a PR, or tag without asking.** Gitflow here is modified:
  `feature/*` → `develop`, release PR `develop` → `main`.
- **No Claude/Anthropic attribution** in commits or PR bodies.
- Reports are **local `.html` files delivered with `SendUserFile`** — never published, never a hosted
  URL, never committed into a target repo.
- `~/.slop-ferret/records/` was **cleared on 2026-08-03** for a clean slate. The five schema-0
  records from earlier campaigns (ghola, counterspy, roboticus-rust, two for slop-ferret) are gone.
  A sweep run from here writes the first record in the current schema, so the legacy-location
  reporting path in `ListRecords` is now exercised only by its tests — do not read a passing
  `ferret records` as evidence that path works.
- `~/.slop-ferret/maps/` was kept. It holds the roboticus-rust map that cost 68 minutes to build and
  is the source of `internal/gate/testdata/real-magma-rust-*.json`.

## Outstanding, not code

**A live GitHub PAT was printed into a transcript earlier in this work** (`github_pat_11ATHU2IA0…`,
from dumping `~/.claude/settings.json`). Rotation was recommended and has **not been confirmed**.
Worth raising once.

## How to know when this is done

Not by self-assessment. Three rounds of confident "done" were each refuted, and the failures got
subtler rather than rarer. The gate should be a clean adversarial review — five independent
red-team stations, blind to each other, with a mutation auditor among them that re-runs every
"verified by mutation" claim against the tree.

The mutation auditor is the highest-value station: it is the only mechanism in this project that has
repeatedly caught the author rather than confirmed them.

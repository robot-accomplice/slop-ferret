# slop

Companion tool for the `slop-ferret` sweep — the deterministic half of the method.

The split it exists to enforce: **transforms belong in the tool, judgement belongs in the skill.**
Enumerating files, computing coverage fractions and laying out a report need no model, and all
three were being done by hand. Deciding whether a finding clears its pre-filing bar does need one,
and no amount of Go will do it.

## Install

```
go install github.com/robot-accomplice/slop@latest
slop install
```

The skill is embedded in the binary, so that pair of lines is also the whole upgrade path. It
deploys `~/.claude/skills/slop-ferret/` and **both** command entries, and re-points the vault
lexicon symlink at the installed copy.

```
slop doctor     # drift, in both directions
```

## Why the installer exists

The skill used to be installed by hand, and the install was never finished.
`~/.claude/commands/slop-ferret/report.md` existed, so `/slop-ferret:report` resolved. Nothing
linked `/slop-ferret` itself, so the parent skill could not be invoked, so its `allowed-tools`
never applied — and `allowed-tools` is what withholds `Edit` and `Artifact`, which `SKILL.md`
names as the runtime enforcement of *additive-only* and *never-publish*. A pre-registered control
ran the entire method that way, holding both tools it was supposed to be denied. It didn't use
them; that was discipline, not enforcement.

That is a distribution defect wearing a safety defect's clothes, and hand-installation is what
produced it: someone linked one of two entries, once, and nothing ever looked for the other.
`slop doctor` now fails on exactly that state, and says why it matters rather than only that it
happened.

The installer also retires the digest-stamping scheme. With no repo, the deployed copy was the
only copy, so a script hashed it to detect edits — which could say *something changed* and never
*what changed*. The repo is the version control now; `doctor` answers the real question, and
distinguishes "the binary moved on" from "you edited the deployed copy by mistake."

## Status

One user, work in progress, not ready to share.

- `install`, `doctor`, `version` — done, in Go, tested.
- `plan`, `verify` — still `python/gate.py`, awaiting port. Deferred deliberately: their
  behaviour is pinned by 44 tests whose constants (signal anchoring, tier split, defer floor)
  were each measured against a real repository, and porting them in the same pass as the
  extraction would have left neither half checkable.
- `report` — not built. The report is still assembled by hand, which is where two HTML defects
  shipped in a single sweep on 2026-08-01. It is a pure transform of plan + discharge + findings
  and should never have been model work; it is next after the `plan`/`verify` port.

## Known temporary state

`python/gate.py` is the source of truth for `plan`/`verify` until the port lands. The deployed
`~/.claude/skills/slop-ferret/scripts/gate.py` is a **symlink** to it, not a copy — two copies of
one rule with no gate holding them equal is a lexicon class (*duplicated implementation*), and
shipping one inside the tool that hunts it would be absurd. A symlink cannot drift.

## Symbiosis: magma, architext, slop

Three tools over **one** derived artifact. magma parses the repository once and emits a versioned
call graph; the other two consume it rather than re-deriving it, and none of them re-parses what
another already knows.

```
                 ┌───────────────────────────────┐
   repository ──►│ magma  — parse once           │
                 │  reachability (RTA), dead code│
                 │  test-only code, call graph   │
                 └──────┬─────────────────┬──────┘
                        │                 │
        codemap-rows/1  │                 │  magma-code-graph/1
        (row files)     │                 │  (architecture emit)
                        ▼                 ▼
                   ┌─────────┐       ┌────────────┐
                   │  slop   │       │ architext  │
                   │ sweeps  │       │ architecture│
                   │ for AI  │       │ docs, C4,   │
                   │ slop    │       │ flows       │
                   └─────────┘       └────────────┘
```

**The contracts are the seam, and they are not interchangeable.** `codemap-rows/1` (row files —
the only one `slop` accepts), `codemap-graph/1` (`graph.json`), `magma-code-graph/1` (the architext
emit). `slop plan` refuses a map whose `contract_version` it does not know, and refuses a map of a
different tree by `sha` — a stale or reshaped map fails loud rather than silently seeding rows from
the wrong commit.

**What each is for.** magma answers *what reaches what*. architext turns that into architecture
documentation a human reads. slop turns it into a reading order for an audit — which code is
worth reading first, and what has not been looked at yet.

**Where the division is going.** Ranking by consequence belongs in magma, not here: "does this file
reach `os/exec`, `net/http`, `os.OpenFile`, `crypto/*`" is a graph query over imports, and magma
already holds the graph and the types. `slop`'s current path-name signals are a guess at semantics
from names the target's own authors chose, which is why they under-enumerate silently. Moving the
ranking down to magma makes it a fact instead of a guess, and every magma consumer gets it.

**No note-app dependency.** State lives in `~/.slop/` (`maps/`, `records/`). An earlier version
symlinked the lexicon into an Obsidian vault so `[[wikilinks]]` resolved; that made the vault part
of the tool's correctness, and two copies of one definition under one name is precisely the drift
class this tool hunts. The dependency is removed rather than symlinked around.

# slop-ferret

Companion tool for the `slop-ferret` sweep — the deterministic half of the method.

The split it exists to enforce: **transforms belong in the tool, judgement belongs in the skill.**
Enumerating files, computing coverage fractions and laying out a report need no model, and all
three were being done by hand. Deciding whether a finding clears its pre-filing bar does need one,
and no amount of Go will do it.

The name is the hunter, not the quarry: this ferrets slop **out**, it does not produce it.

## Install

```
go install github.com/robot-accomplice/slop-ferret@latest
slop-ferret install     # deploys the copy compiled in — works offline
slop-ferret update      # pulls the current skill from this repo instead
slop-ferret doctor      # drift, in both directions
```

## Two artifacts, two cadences

**The skill is not welded to the binary.** SKILL.md and the lexicon are prose; they change far more
often than the code does, and usually for reasons the code does not care about. Binding them to one
release meant a new class definition could not reach a sweep without a rebuild, a `go install` and a
reinstall — a 1:1 coupling between two things with genuinely different rhythms.

So there are three sources and one install path:

| source | command | when |
|---|---|---|
| **embedded** | `slop-ferret install` | bootstrap floor — always present, works offline, what a fresh `go install` has before it has talked to anything |
| **repo** | `slop-ferret update [--ref main]` | the live `skill/` tree from this repository, pinned to the commit the ref resolved to |
| **dir** | `slop-ferret install --from ~/code/slop-ferret` | the edit-build-install loop |

`slop-ferret version` and `doctor` print the binary version and the skill version **separately**,
along with which source the deployed copy came from. "Which skill am I running" and "which binary am
I running" stopped being the same question, so they stopped being one number.

An update writes to a temp dir and installs from there: a half-applied update is worse than a stale
one, and a failed fetch leaves the deployed skill untouched.

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
`slop-ferret doctor` now fails on exactly that state, and says why it matters rather than only that it
happened.

The installer also retires the digest-stamping scheme. With no repo, the deployed copy was the
only copy, so a script hashed it to detect edits — which could say *something changed* and never
*what changed*. The repo is the version control now; `doctor` answers the real question, and
distinguishes "the binary moved on" from "you edited the deployed copy by mistake."

## Status

One user, work in progress, not ready to share.

- `plan`, `verify`, `install`, `update`, `doctor`, `version` — done, in Go, tested. No Python
  survives: the port was verified differentially against the retiring implementation first, over a
  real repository, before the original was deleted.
- `report` — **authored, not generated, by choice.** The HTML report is written by Claude against
  the spec in `skill/commands/`. That spec is part of the skill, so it now revises without a binary
  release — which is the arrangement that makes an authored report cheap to improve.


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
the only one `slop-ferret` accepts), `codemap-graph/1` (`graph.json`), `magma-code-graph/1` (the architext
emit). `slop-ferret plan` refuses a map whose `contract_version` it does not know, and refuses a map of a
different tree by `sha` — a stale or reshaped map fails loud rather than silently seeding rows from
the wrong commit.

**What each is for.** magma answers *what reaches what*. architext turns that into architecture
documentation a human reads. slop turns it into a reading order for an audit — which code is
worth reading first, and what has not been looked at yet.

**Where the division is going.** Ranking by consequence belongs in magma, not here: "does this file
reach `os/exec`, `net/http`, `os.OpenFile`, `crypto/*`" is a graph query over imports, and magma
already holds the graph and the types. `slop-ferret`'s current path-name signals are a guess at semantics
from names the target's own authors chose, which is why they under-enumerate silently. Moving the
ranking down to magma makes it a fact instead of a guess, and every magma consumer gets it.

**No note-app dependency.** State lives in `~/.slop-ferret/` (`maps/`, `records/`). An earlier version
symlinked the lexicon into an Obsidian vault so `[[wikilinks]]` resolved; that made the vault part
of the tool's correctness, and two copies of one definition under one name is precisely the drift
class this tool hunts. The dependency is removed rather than symlinked around.

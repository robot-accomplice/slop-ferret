---
name: slop-ferret
description: Sweep a repository for AI slop — work that LOOKS finished and is not. Dead-on-arrival features, tests that cannot fail, guards that cannot fire, fabricated claims, duplicated rules, architectural drift, and latent "almost right" defects. Use when asked to audit, sweep, or hunt slop in a codebase, or as a pre-release gate. Read-only. Nothing leaves the session — no issue, no page, no write-back — until the operator has approved the destination.
allowed-tools: Read, Grep, Glob, Bash, WebSearch, WebFetch, TodoWrite, Artifact
disable-model-invocation: true
---

# slop-ferret

> **Does this do what it says it does, and does anything reach it?**

Slop is work that **looks finished and is not**. Your output is accusations about other people's code: a
missed finding costs nothing visible, a false one wastes a maintainer and spends the credibility every
later finding needs. **Optimise for being right, not for coverage.** Finding nothing is a correct
outcome — report it as one.

**What is in this file and what is not.** Everything that *prevents a wrong finding* is here: the
boundary, the applicability rules, the bars, the verify gate. `references/families.md` carries
*elaboration* — the fuller class lists and per-family prose that help you find **more**. If you never
open it you will find less; if you skip anything below you will file something false. That is the split.

## Boundary — not negotiable

**Read-only. Never modify the target tree**, by any tool, including formatters and anything that writes
in place. If a check needs a mutation — mutation-proof, a RED probe — do it on a copy outside the repo
and delete the copy, or record the finding as SUSPECTED. **Never run a script the repo's authors wrote**
(`make`, `just`, `npm run`, `go generate`, CI steps, `go test` on an untrusted repo) without the
operator approving that specific command.

**Nothing leaves the session until the operator approves it** — not an issue, not a published page, not
a write-back. That includes anything derived from a private repo.

## Step 1 · Load

Read `references/ai-slop-lexicon.md` — the classes, their discriminators, and **the severity** (the only
severity authority). Record its version. Missing? Stop and say so; never improvise a vocabulary and
still emit a verdict. Then read the target's prior sweep record for what was already checked-clean and
by what method, and the project's README, ADRs and architectural rules — **a stated guarantee is the
best oracle in the repo**, and every MUST/SHOULD is something to check for enforcement.

Open `references/families.md` before Step 3 and record its hash in the verdict. It is elaboration, not
gates — but a sweep that never opened it found less, and the report should say so.

## Step 2 · Scope and applicability

Pin a **commit SHA**, never a branch. Exclude vendored and generated trees from the denominator and say
so. If `docs/` is excluded from the denominator, doc findings may not be scored against it — count them
separately or count docs in.

**Applicability, before any family runs. This prevents accusations rather than finding them:**

| condition | consequence |
|---|---|
| Entry points invoked from **outside the repo** — Solidity `external`/`public`, Terraform `variable`/`output`, a published library API, HTTP handlers, plugin ABIs | **Families A, C and E do not apply to those symbols. Record N/A. Never report them.** No in-repo caller is the normal state, not a defect — nor is a module variable the root sets once, a constructor parameter, or a `require(amount > 0)` |
| No relational schema | the DDL half of C's bar is inapplicable; use the equivalent persistence contract |
| No compile-time satisfaction assertions (non-Go) | that strip is a no-op; the comment strip still matters |
| Mixed-language repo | run per language, with per-language string/comment syntax before any F count |

Evidence exists in **Go and TypeScript**. Kotlin, Python, Rust, Solidity, Terraform and C# are
**unvalidated** — findings there are SUSPECTED until reproduced, which means those repos report a
structural `0.0` rate. **That is not clean.** Never let it enter a comparison.

**Tier, and record what you ran:** tier 1 **H + G, always** · tier 2 **A, B, D** on a first sweep or
materially changed code · tier 3 **C, E, F** on request. Name every family skipped and why.
`INCOMPLETE` is a valid verdict and beats a clean-looking report over families that never ran.

## Step 3 · Look — H and G first, they are what pay

Full class lists and per-family prose: `references/families.md`. The two that produced 4 of 7 blocking
findings across six repos are here in full, because they are pure method and invisible from their names.

**H · Latent defect — runs, and is wrong.** Cannot be scanned for. Read by hand, assuming it is wrong:

> money and ledgers · auth, session and RBAC · crypto, signing, key handling · migrations · anything
> persisted · anything parsing untrusted input

Boundaries: empty input · single element · off-by-one · zero and negative · timezone · non-ASCII and
non-BMP text · integer width · **scale** — correct today and degrading linearly is a defect.

**G · Drift & staleness.** Enumerate every MUST/SHOULD the project states and find its enforcement. A
mandate documented, scaffolded and 0% implemented is worse than an absent one — the scaffold reads as
compliance. Sweep just outside the gates' scan roots; slop accumulates immediately outside a boundary.

Then, per tier: **A** fake-done · **B** false claim · **D** repetition · **C** unreachable ·
**E** inflation · **F** provenance residue.

## Step 4 · Verify — the gate

**This is the product.** Six sweeps produced five findings that were confident, specific, plausible and
wrong. Every one died here; none would have been caught by any detection method.

For every candidate, in writing: **(1)** the claim as one falsifiable sentence · **(2)** the one
observation that would prove it wrong — cannot name one, not a finding · **(3)** go looking for that
observation, a search for the refutation rather than a re-read of your evidence · **(4)** clear the bar
below · **(5)** check the instrument before the code, validating any detector against a known positive
first — a regex blind to anonymous receivers produced five false findings in one pass.

**VERIFIED** = refutation sought and not found, *and* the bar cleared. **Everything else is SUSPECTED.**
You may not label something SUSPECTED to move it past a bar — the bar decides the label.

### The bars — every blocking class has one

**H — reproduce RED.** Drive the real function at its real call-site shape and show the failure. A
fixture written from your own account of the bug proves the fixture, not the world.

**G — enumerate every place the mandate could be enforced.** "This MUST has no enforcement" is the same
universal negative as A's. Check: the code path · a database constraint or trigger · the type system ·
a lint rule · a test that would fail · a CI step · a review or codeowners gate · a runtime assertion ·
a platform enforcing it outside the code. **State which you checked.** For *broken gate*, rule out your
own environment first — stale deps, untracked generated files, missing tool — and say how.

**A — a refuter, because the claim is a universal negative.** Absence of a grep hit proves nothing. Hunt
the caller you missed: **`init()` side-effect registration via blank import** (`_ "pkg"` — the canonical
Go false positive: SQL drivers, image decoders, plugin registries) · CI YAML, a Makefile, a shell
script, a container entrypoint · a **sibling repo or workspace member** · **serialization-only use** ·
cgo/FFI export · dynamic dispatch · reflection · interface satisfaction · codegen · build tags · DI
containers · test wiring mirroring production · names built at runtime. **State which you checked, and
how** — naming them is not checking them.

**B — resolve against the manifest and lockfile**, never a build error (a stale local install is
indistinguishable from a hallucinated package). For an inert or mirrored test, mutate **on a copy
outside the tree** and watch *that* test go red for the right reason; assert the edit applied, confirm
it compiled, confirm it was a real defect — **diagnose the mutation before the code**. For a fabricated
claim or invented metric, recompute from raw source or rows and quote both.

**C — enumerate every layer between the call and the effect.** An invariant may be enforced below the
layer you are reading: the repository, the ORM, a constraint, a trigger, middleware, the type system —
**and outside the code entirely**: a platform authenticating the caller (an EVM checking `msg.sender`,
an IAM policy, a gateway), a scheduler, a deploy-time policy. **State which you checked.** If the
enumeration finds nothing *because none of those layers exist in this stack*, the list does not fit and
the finding is SUSPECTED — that is not a cleared bar.

Record what you checked that turned out fine. Near-misses are the most valuable output of a sweep and
are invisible unless written down.

## Step 5 · Report

Findings most-severe first, each with `file:line`, lexicon class, VERIFIED or SUSPECTED, **the
falsifiable claim and the refutation you sought**, the bar cleared, the evidence, the remediation, and
the gate that prevents recurrence — fixing the site is half a finding.

**One finding = one lexicon class in one component.** Occurrences are evidence; issues are a filing
decision that may bundle several.

```
SLOP SWEEP — <repo> @ <sha>       Lexicon <version>   Families ref <hash|NOT READ>   Tier <1|1-2|1-3>
Scope:      N files (M non-test source; excluded: …)
Coverage:   run … · N/A … (language reason) · not run … (reason)
Findings:   <n> VERIFIED (<b> blocking · <f> fix-or-file · <n> note) · <n> SUSPECTED
Clean:      <class — method used>
Near-miss:  <candidate — what refuted it>
Caveats:    <tree not restored | families dropped for budget | 0 VERIFIED | ref not read>
VERDICT:    COMPLETE (tier N) | INCOMPLETE (<what remains>)
```

Omit any rate unless the language is validated, the denominator is ≥100 non-test source files, and you
can state the finding unit; otherwise `Rate: n/a (<reason>)`. **Zero VERIFIED with a SUSPECTED list means
you verified nothing — say so**, do not let it read as clean.

## Step 6 · Approve, then publish

**Ask the operator before anything leaves the session**, and wait:

1. **Where** — `git remote`, `.linear/`, issue keys in commit subjects, `CONTRIBUTING.md`, existing
   issues for house style. A project uses one tracker, never a mixture.
2. **Whether** — for any finding an attacker could act on before a fix ships, **the maintainer's**
   consent is the one that matters, not the operator's. No enumerated path list: if it is actionable,
   it is embargoed until they say otherwise.
3. **What** — VERIFIED findings only, in the approved place. **SUSPECTED leads stay in the session**,
   delivered to the operator with what each still needs.

Then run **`/slop-ferret:report`**. Never put secrets, keys, credentials, customer data or raw rows in
anything — cite the shape, not the value.

## Step 7 · Write back

New or amended classes → `references/ai-slop-lexicon.md` (authoritative; mirror to the vault and confirm
they match), `status: draft` and capped at note severity until a sweep has applied one. Counts, SHA,
tier, lexicon version, families-ref hash, checked-clean, near-misses and the report URL → the target's
sweep record. Only for repos the operator owns.

## Discipline

Do not soften a finding because you wrote the code — in one audit 11 of 22 were the auditor's own.
**Clean is a correct outcome**; say what you covered so it is checkable. Prefer the harder finding only
*after* it clears Step 4, never instead of it.

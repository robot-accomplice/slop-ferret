---
name: slop-ferret
description: Sweep a repository for AI slop — work that LOOKS finished and is not. Dead-on-arrival features, tests that cannot fail, guards that cannot fire, fabricated claims, duplicated rules, architectural drift, and latent "almost right" defects. Use when asked to audit, sweep, or hunt slop in a codebase; as a pre-release gate on a frozen branch; or after a large burst of work. Additive-only against the target tree — never edits in place. Every finding must clear a verification gate before it is written up; SUSPECTED findings are reported to the operator, never filed; nothing is filed until the destination tracker is confirmed. Always ends with /slop-ferret:report, which builds the HTML report.
allowed-tools: Read, Grep, Glob, Bash, Write, WebSearch, WebFetch, TodoWrite, SendUserFile, Skill
disable-model-invocation: true
---

# slop-ferret

One question about every line in scope:

> **Does this do what it says it does, and does anything reach it?**

Slop is work that **looks finished and is not**: a feature nothing calls, a test that cannot go red, a
comment describing behaviour the code lacks, a guard whose condition can never be true.

**Your output is accusations about other people's code.** A missed finding costs nothing visible. A
false finding wastes a maintainer, insults working code, and spends the credibility every later finding
needs. **Optimise for being right, not for coverage.** Finding nothing is a correct outcome and must be
reported as one.

---

## Operating rules (read before anything else)

**ADDITIVE-ONLY, and never in place.** `Edit` is deliberately not granted: you may not modify an
existing file in the target repo, ever. The one write you may perform is **creating a new file** (the
reproduce-RED probe), which is trivially removable. `Bash` is granted for reading and analysis and does
not licence what this rule forbids — a grant is not a permission.

**Before that write:** confirm `git status --porcelain` is empty, or record exactly what was already
dirty. **After it:** delete the probe and re-check `git status` against that baseline in the same step.
Name the probe `zz_ferret_probe_*` so an abandoned one is identifiable if a session dies mid-sweep. If
you cannot verify the tree returned to its baseline, **say so in the report** — do not let it pass
silently.

**Mutation-proof is therefore out of scope for the target tree.** If you need to prove wiring by
breaking it, do it in a scratch copy outside the repo, or record the claim as SUSPECTED. This is a
deliberate downgrade: an unrevertable edit to a release candidate is worse than an unproven finding.

**Never run a repo-declared script without asking.** `make`, `just`, `npm run`, `go generate` and CI
steps execute code the repo's authors wrote. Read them; run only what the operator approves for this
target. Formatters, compilers, linters and test runners you invoke directly are fine.

**Never publish anything until you have confirmed where it goes.** For *findings*, that means the
tracker — see Step 6. For the *report*, there is no destination to confirm: it is a local file and is
never published (Step 5). Do not read this rule as "confirm, then publish" for the report.

**Never put secrets, keys, tokens, credentials, customer data or raw production rows into a finding.**
Cite the shape, not the value. A finding on a crypto, auth, signing or key-handling path is **not filed
publicly without explicit operator authorisation** — say so and wait. File evidence only in the repo it
came from; never quote a private repo's source into another repo's tracker.

**One writer per repo.** If another session owns the tree you are read-only *including* the exceptions —
report as SUSPECTED rather than mutating a shared worktree.

---

## Running a pre-registered control

**If this run is a control, read `controls/README.md` in this skill directory first.** It is the
brief for the executing session: what is withheld, what must be declared, and what invalidates the
run. That file said "read this whole file before anything else" and, until 2026-08-01, nothing
anywhere pointed at it — so a control could complete without it ever being opened, which is exactly
what happened on the ghola run. Same shape as the `families.md` gap below: a file the method depends
on, unreachable from its own entry point. If you are not running a control, skip it.

---

## Step 0 — load the vocabulary

**1. Read the lexicon** — `references/ai-slop-lexicon.md` in this skill directory. It defines every
class: what decides membership, the discriminator against its nearest neighbour, a detection method, and
**the severity**. Record its version in your report.

**There is ONE lexicon file.** `references/ai-slop-lexicon.md` in this skill directory is it.
`~/Claude Vault/wiki/practices/ai-slop-lexicon.md` is a **symlink** to that file, so Obsidian can browse
it and `[[ai-slop-lexicon]]` keeps resolving. Write to either path; they are the same bytes.

This used to be two real files kept in step by hand, and the hand-mirroring is exactly what broke: on
2026-08-01 they had drifted while both still declared version `2026-07-26.1`, because a version string
cannot detect an edit that did not bump it. Two copies under one name make every recorded version
meaningless, and the reconciliation ritual that was supposed to catch it only ever ran at sweep time,
which is far too late to be a control. A symlink cannot drift.

**Two stop conditions, both cheap to check:**
- **The vault path is a regular file rather than a symlink** — someone replaced the link, so a second
  definition exists again. Diff it against the in-skill file, reconcile deliberately, restore the
  symlink, and tell the operator what differed.
- **The in-skill file is missing.** Do not improvise a vocabulary and do not emit a verdict block — a
  sweep without its class definitions produces a report indistinguishable from a real one, and is not
  one.

**1b. Record THIS SKILL's own identity.** Run `scripts/skill_version.py check` and put the
`<version>+<digest>` it prints in the report beside the lexicon version. **A non-zero exit is a
stop condition:** the skill was edited without stamping, so its recorded version is not true and
any sweep citing it is citing a fiction. Stamp it (`skill_version.py stamp <version>`) or say on
the face of the report that the sweep ran against an unstamped skill.

This exists because this skill mandates "pin a commit SHA, never a branch" and lives outside
git. A typed version alone would not do: on 2026-08-01 both copies of the lexicon declared
`2026-07-26.1` while their contents had diverged, which is the same failure. The digest is what
makes the version falsifiable, and it is the only property of a git sha this skill needs.

**2. Read the target's prior sweep record** if one exists, for its counts and — more useful — the
classes recorded CLEAN *with the method used*. Do not re-spend budget there unless the method has since
improved or the code has materially changed.

**3. Refresh the lexicon from the web** *(skippable if its last web-diff is under 30 days old; say which
you did).* Search for how AI slop and AI-generated-code smells are currently defined, and diff against
the lexicon. **New classes enter with `sev` = `N (draft)`** and stay there until a sweep has applied
them — a class is not validated until it has been used, and two of the first batch were wrong on first
use. The `sev` cell is the carrier because the tables have no `status` column; this instruction named
one that never existed, so the draft cap was unenforceable from the day it was written.

**4. Read the project's own claims.** README, ARCHITECTURE, CONTRIBUTING, ADRs, architectural rules. A
product's stated guarantee is the best oracle in the repo — *"a signal it couldn't collect is reported as
a gap, never an all-clear"* produced the sharpest finding of six sweeps. Note every MUST/SHOULD; you will
check each for enforcement in family G.

**5. Run the Magma skill.** Magma deterministically creates a call map from code. Using it will reveal 
dead code and other possible defects

---

## Step 1 — scope, applicability, depth

**Scope.** Whole repo by default; `git diff <last-tag>..HEAD` as a release gate. Exclude vendored and
generated trees from the denominator and say so.

**Docs rule — state it, because tier-1 G produces doc findings on every sweep.** If `docs/` is excluded
from the denominator, findings *in* `docs/` may not be scored against it. Either count doc files in the
denominator, or report doc findings as a separate un-rated list. Mixing them inflates doc-heavy repos and
was one of the four defects that made the first four sweeps non-comparable.

**Pin a commit SHA, never a branch** — a branch denominator cannot be re-derived later.

**Language applicability — settle this before running any family.** Several detections assume in-repo
callers and a package-visibility model.

| condition | consequence |
|---|---|
| Entry points invoked from outside the repo — Solidity `external`/`public`, Terraform `variable`/`output`, a published library API, HTTP handlers, plugin ABIs | **Families A, C and E do not apply to those symbols. Record N/A. Never report them.** "No in-repo caller" is the normal state, not a defect — and neither is a module variable the root sets once, a constructor parameter, or a `require(amount > 0)` |
| No relational schema | the read-the-DDL gate is inapplicable; use the equivalent persistence contract |
| No compile-time satisfaction assertions (non-Go) | that strip is a no-op; the comment strip still matters |
| Mixed-language repo | run per language, with per-language string/comment syntax before any family F count |

Evidence for these classes exists in **Go and TypeScript**. Kotlin, Python, Rust, Solidity, Terraform and
C# are **unvalidated** — findings there are SUSPECTED until reproduced.

**Depth is tiered, and the tier is recorded.** Fixed depth was tried and abandoned: unachievable under
budget, and its own template offered a skip field, so sweeps ran unequal anyway while claiming not to.

- **Tier 1, always:** **H** (latent defect) and **G** (drift, incl. mandate enforcement).
- **Tier 2, first sweep or materially changed code:** **A**, **B**, **D**.
- **Tier 3, on request or when tiers 1-2 point at it:** **C**, **E**, **F**.

Record the tier, and every family not run **with its reason**. A low coverage fraction is a valid and
useful result, and is strictly better than a clean-looking report over families that never ran. If budget runs short,
checkpoint and say where you stopped — never silently drop tier 1.

**Severity comes from the lexicon's per-class column. There is no family-level severity.** Two
authorities produced rates differing by 8% on identical data.

---

## Step 1½ — seed from the map (the coded gate)

When the target has a real code map (Go with a `main` today), seed the sweep from it — **do not
re-derive by reading what the map already answers, and do not trust the map to replace reading.** The
seam is a script contract, not a narrated one:

```bash
SHA=$(git -C <repo> rev-parse --short HEAD)                      # CLEAN tree, always
magma --depth 1 <repo> <name> ~/Claude\ Vault/codemap            # rows land in <name>/.magma/
gate.py plan  ~/Claude\ Vault/codemap/<name> "$SHA" <repo>  > plan.json
# ... do the sweep: read every plan.h_required path; account for EVERY candidate ...
gate.py verify plan.json discharge.json   # two fractions + a work queue; 0 settled · 3 items open
```

**The generator is `magma` (`~/go/bin/magma`), and four of its properties bite.** Confirmed with
its maintainer 2026-08-01:

- **Row files live in `<map>/.magma/`, not at the map root.** The gate reads either, so pass the
  map dir. `--depth 1` bounds the markdown to ~100 notes and finishes in seconds; the JSON contract
  is complete regardless, so use it whenever you only need the data.
- **`--force` whenever magma itself changed.** Freshness is keyed on the ANALYSED repo's sha, not on
  magma's version, so an unchanged repo reports "already fresh" and writes nothing — silently
  producing a stale map from a magma you just fixed. `generator` in the envelope names the build
  that wrote a map; check it rather than the run's exit code.
- **Never gate on a dirty tree.** magma stamps a dirty map's `sha` as `<sha>+<diffhash>`, which can
  never equal a pinned commit, so the gate refuses by construction. That is the point: a dirty map
  reports in-flight, not-yet-wired code as dead, and its sha is *disproportionately* likely to
  evaporate because in-flight commits get amended or rebased away. Two earlier roboticus sweeps
  pinned dirty-map shas and neither resolves today.
- **Three contract strings, not interchangeable.** `codemap-rows/1` (row files — the only one this
  gate may accept), `codemap-graph/1` (`graph.json`), `magma-code-graph/1` (the architext emit).

**A family with no map seed DID NOT RUN.** magma emits `_dead.json` and `_test-only.json` today;
`_interfaces.json` (family E) is approved and coming, and `_duplicates.json` (family D) is
deliberately not built, because magma has no notion of similarity and a false duplicate row is a
refactor order for code that should be left alone. The gate reports these in
`plan.unseeded_families` and they must appear as NOT RUN in the verdict block. They may never be
reported as checked-clean.

`gate.py` (`scripts/gate.py`, with its own suite in `tests/`) **refuses** unless the map is the right tree
(`sha`) and a shape it parses (`contract_version`) — so a stale or reshaped map fails loud, not
silently. It turns map rows into per-family **candidates carrying each class's pre-filing bar** (and a
heavier bar when the map's `fidelity` is weaker than a real call graph), and it enumerates the
**family-H worklist** (money/auth/crypto/consensus/arithmetic/migration/persist/parse/network paths).
A candidate is a candidate until it clears its bar in Step 3; the map never files anything.

**The worklist is split by consequence, and the split decides the verdict.** `h_required` is the
blast-radius tier — money, crypto, auth, consensus, arithmetic, plus anything you added via
`.slop-h-signals`; `h_deferred` is the volume tier. **`h_unmatched` is the complement — every other
production source file — and it is enumerated too.** A signal match is a RANKING, not an admission
gate: matching nothing no longer means a file is absent from the sweep, it means nobody has looked at
it yet.

**`verify` reports two fractions and no verdict word.** `coverage.repo` is production source files
read over the total; `coverage.plan` is items dispositioned over items raised. They are different
numbers, and the gap between them is the point. Exit 0 means the accounting is settled and 3 means
items are still open — bookkeeping only, the way a test runner reports outstanding failures. **Carry
BOTH fractions into the report banner**, not one of them and not a word.

COMPLETE/PARTIAL/INCOMPLETE were removed on 2026-08-01 because one token cannot carry two
quantities. Measured on ghola @4f33b3c: 10/10 on the plan, ~16/24 on the repo, reported **COMPLETE** —
and the enumeration had never named `internal/bridge/bridge.go`, an unauthenticated localhost HTTP
server making arbitrary outbound fetches, which a hand read turned into the sweep's worst finding.
Coverage of the enumeration was being read as coverage of the repository. PARTIAL was in any case
unreachable on any worklist at or below `H_DEFER_FLOOR` — ghola's `h_deferred` was empty, so the
verdict had one live state.

**What the gate establishes, and what it does not.** It checks you ACCOUNTED for every item the plan
raised. It cannot establish that you read a file — `read_paths` is your own report and nothing
corroborates it. Attestation is still worth requiring, because it makes an omission a statement someone
made rather than a gap nobody owns. Do not let the gate's exit code stand in for the reading.

**If the domain vocabulary misses your repo, extend it.** `enumerate_h_worklist` is path-based and
therefore domain-bound: add `reason: regex` lines to `.slop-h-signals` in the target repo and re-plan.
An empty worklist is a hard stop, never a clean result — `ghola` enumerated zero H-paths as an HTTP
client because the vocabulary had no network terms, and a short worklist reads as a clean repo.

**Pass `--since <ref>` and read the unmatched-change list first.** Extending the vocabulary fixes an
instance and never the class: the next unenumerated subsystem is exactly as silent, and nothing checks
whether your `.slop-h-signals` additions were sufficient. `--since` inverts the question — it compares
the enumeration against a set already known to matter, what actually changed, and reports every changed
production file **no signal reached**. Those are the gate's own blind spots, made countable. They cannot
be deferred on a consequence argument, because nobody assessed their consequence: attest each, or list
it in the discharge's `coverage_waived`.

This exists because on 2026-08-01, roboticus's worklist ran to 387 paths and **none of the six
production files its release had changed were on it** — including a *financial rules engine*, missed by
a money vocabulary that lists `pay|ledger|billing|fee|gas|price` and not the word "financial", and
fifteen files of authorization logic under `internal/agent/policy/`, missed by an auth vocabulary that
lists `auth|acl|rbac` and not "policy". The gate would have returned a clean verdict for reading 387
paths while never opening one line the release touched. **A worklist is evidence about what you
enumerated, never about what is there.**

Non-Go / no-`main` / magma refuses → skip the gate, run the full read below, and say "map unavailable,
sweep-only." Never fake a map.

## Step 2 — the families

Running order is **value-first**, not cheapest-first: H and G lead, because that is where the blocking
findings were and because they are what dies when budget runs out.

**Read `references/families.md` before running them.** Everything that *prevents* a wrong finding is
here in `SKILL.md`; that file holds the elaboration — worked examples, the shapes that recur, where the
yield historically was. Skipping it costs **recall**, not correctness, which is why it is a reference
and not a gate. But record which you did: the verdict block carries `Families ref`, because a sweep that
never opened it must not be able to claim it did. This skill named that file in exactly one place — a
parenthetical inside the lexicon's detection column — for the ten days after the split, so the file the
method's recall half lives in was unreachable from its own entry point, and the compliance field that
was the stated condition of accepting the split had been dropped.

### H · Latent defect — runs, and is wrong  *(tier 1)*
*"Almost right" logic · command-query mixing · silent failure.*

**The family that pays.** It cannot be scanned for. Read the highest-consequence paths by hand, assuming
they are wrong:

> money and ledgers · auth, session and RBAC · crypto, signing, key handling · migrations · anything
> persisted · anything parsing untrusted input

Boundaries to target: empty input, single element, off-by-one, zero and negative, timezone, non-ASCII and
non-BMP text, integer width, and **scale** — a value correct at today's volume that degrades linearly is
a defect.

**Pre-filing bar — reproduce RED.** Drive the real function at its real call-site shape and show the
failure. A fixture written from your own account of the bug proves your fixture, not the world.

### G · Drift & staleness — was true once  *(tier 1)*
*Stale reference · phantom backlog · stale dependency · orphaned artefact · mandate drift · broken gate.*

Enumerate every MUST/SHOULD from Step 0.4 and find its enforcement. A mandate documented, scaffolded and
0% implemented is worse than an absent one — the scaffold reads as compliance.

Read declared gates; **run only what the operator approved.** A gate failing in your local checkout is an
environment finding until proven otherwise — check for stale installs and untracked generated files
before calling a gate broken.

**Sweep just outside the gates' scan roots.** Slop accumulates immediately outside a boundary.

**Pre-filing bar — enumerate every place the mandate could be enforced.** "This MUST has no enforcement"
is the same universal negative as family A's "nothing calls this", and it carries two blocking classes,
so it gets the same treatment. Before claiming *mandate drift*, check each of: the code path itself · a
database constraint or trigger · a type or the compiler · a lint rule or custom analyzer · a test that
would fail · a CI step · a codeowners or review gate · a runtime assertion · documentation the team
treats as binding. **State which you checked.** Before claiming *broken gate*, rule out your own
environment first — stale dependencies, untracked generated files, an unprimed cache, a missing tool —
and say how. A gate red only on your machine is not a finding.

### A · Fake-done — looks finished, does nothing  *(tier 2)*
*Dead on arrival · tautological self-proof · constant stub · unfinished-work marker · gated empty seam.*

Applicability first: where entry points are invoked from outside the repo, this family does not apply to
them.

Find a non-test caller; follow the chain to a real entry point. Strip **comments and compile-time
satisfaction assertions** before counting references — a doc comment names its own symbol, so any
`count <= 1` filter finds nothing without the strip, and an assertion makes dead code look live.
Unused-symbol tools also assume exported means externally consumed, false for any private module.

**Pre-filing bar — a refuter, because the claim is a universal negative.** "Nothing calls this" is not
proven by the absence of a grep hit. Actively hunt the caller you missed:

- **`init()` side-effect registration via blank import** (`_ "pkg"`) — the canonical Go false positive:
  SQL drivers, image decoders, plugin and codec registries
- invocation from **CI YAML, a Makefile/justfile, a shell script, or a container entrypoint**
- consumption by a **sibling repo or another workspace member** of a monorepo
- **serialization-only use** — a type that exists to be marshalled, or referenced by a struct tag
- **cgo / FFI export**, or a symbol resolved by name at link time
- dynamic dispatch · reflection · interface satisfaction · code generation · build tags · DI containers ·
  test wiring that mirrors production · names constructed at runtime

**State which of these you checked, and how** — naming them is not checking them. If you cannot
enumerate them, the finding is SUSPECTED.

### B · False claim — asserts something untrue  *(tier 2)*
*Fabricated claim · inert test · mirrored test · documented-vs-implemented drift · phantom dependency ·
invented metric.*

Every "always / never / regardless / cannot" omits a precondition — find it. Recompute at least one
reported number from raw data.

**Pre-filing bars.** *Phantom dependency:* resolve the import against the **manifest and lockfile**,
never against a build error — a stale local install is indistinguishable from a hallucinated package.
*Inert test and mirrored test:* mutate the behaviour and watch **that** test go red for the right reason.
A green mutation proves nothing about the code: assert the edit actually applied, confirm it compiled,
confirm the mutation was a real defect. **Diagnose the mutation before the code.**
*Fabricated claim and invented metric:* recompute the claim or the figure from raw source or raw rows,
and quote both what the prose says and what the recomputation gives. A claim is not fabricated because it
reads oddly — only because the recomputation disagrees with it.

### D · Repetition  *(tier 2)*
*Duplicated implementation · copy-paste family · synonym helper · magic literal.*

A comment explaining why two copies are identical is not a justification — it is the drift warning. Check
whether any test pins the copies to each other; usually none does.

### C · Unreachable — cannot run  *(tier 3)*
*Dead guard · sited guard · orphan computation.*

For each guard find the code that makes its condition true, and **enumerate the siblings** — other callers
of the same sink. A protection applied where the author was looking is the most repeated defect shape
across every repo swept.

**Pre-filing bar — enumerate every layer between the call and the effect.** An invariant may be enforced
below the layer you are reading: the repository, the ORM, a database constraint, a trigger, middleware, or
the type system — **and outside the code entirely**: a platform that authenticates the caller (the EVM
checking `msg.sender`, an IAM policy, a service mesh, an API gateway), a scheduler, or a deploy-time
policy. If the enumeration finds no layer *because none of the listed layers exist in this stack*, that
is not a cleared bar — it means the list does not fit, and the finding is SUSPECTED. Read the DDL *and* the data-access layer *and* any middleware on that path. **State which
layers you checked.** The most expensive near-miss in six sweeps was an expiry check enforced one layer
down from where it was looked for; a DDL-only rule would not have caught it.

### E · Inflation — more structure than the problem has  *(tier 3)*
*Single-impl interface · speculative surface · over-defensive boilerplate · helper sprawl · blind generics
· inappropriate concurrency · god function · deep nesting.*

Name the caller that needs the flexibility. **Exempt consumer-declared narrow ports** — an interface
declared in the module that consumes it, naming only the methods it uses, is interface segregation done
right, in any language. Applying this family bluntly once produced 23 hits and zero real findings.

### F · Provenance residue — reads as machine-written  *(tier 3, never blocking)*
*Register tells · comment restating code · ceremonial structure · cross-language idiom · placeholder
naming.*

**Separate runtime output from comments before reporting any count.** The difference between "7,022
em-dashes" and "14 in operator-facing error text" is the entire finding.

---

## Step 3 — VERIFY (mandatory; nothing is written up before this)

**This step is the product.** Six sweeps produced five near-misses — findings that were confident,
specific, plausible and wrong. Every one was caught here. None would have been caught by the detection
methods above.

For **every** candidate finding, in writing:

1. **State the claim as a falsifiable sentence.** "X is never called." "Y accepts Z and corrupts W."
2. **Name the one observation that would prove it wrong.** If you cannot name one, it is not a finding.
3. **Go looking for that observation** — a search for the refutation, not a re-read of your evidence.
4. **Clear the family's pre-filing bar** and record what you did:
   **H** reproduce RED · **G** enforcement-site enumeration + environment rule-out · **A** refuter ·
   **B** lockfile (phantom dep) / mutation diagnosis (inert + mirrored test) / recomputation from raw
   data (fabricated claim + invented metric) · **C** layer enumeration.
   **Every blocking class has a bar. If you cannot name the one you cleared, the finding is SUSPECTED.**
5. **Check the instrument before the code.** If a detector produced this, validate it against a known
   positive first. A regex blind to anonymous receivers produced five false findings in one pass — the
   detector was wrong before the code was.
6. **Classify VERIFIED or SUSPECTED.** VERIFIED means the refutation was sought and not found *and* the
   family's bar was cleared. Everything else is SUSPECTED.

**SUSPECTED findings are not filed.** They go to the operator in the report, as a list of leads with what
each still needs, and are filed only if the operator says so. A label in the body of an issue is not a
protection — the maintainer still receives the accusation and still has to disprove it. If everything you
have is SUSPECTED, that is the report: *N leads, none verified*, and it is an honest and useful outcome.

**You may not label a finding SUSPECTED to move it past a bar.** The bars decide the label; the label
does not excuse the bars. A sweep reporting `Rate: 0.0` with a long SUSPECTED list has not found nothing
— it has not finished.

Then **record what you checked that turned out fine.** Near-misses are among the most valuable output of
a sweep and are invisible unless written down.

---

## Step 4 — report

**One finding = one lexicon class in one component.** Not one occurrence, and not one filed issue.
`cosineSimilarity` implemented seven times in one package is **one** finding of *duplicated
implementation* with seven sites; the same class in an unrelated package is a second. Occurrences are
evidence and belong in the body; issues are a filing decision and may bundle several findings. State the
occurrence count beside each finding so a reader can see the difference. Without this rule the same sweep
scores three different ways — 7,888 occurrences, 16 classes and 11 issues were all recorded as the count
for one repo.

Findings most-severe first. Each carries `file:line`, its lexicon class, **VERIFIED or SUSPECTED**, and:

- **the claim, as one falsifiable sentence** (Step 3.1)
- **the observation that would refute it, and where you went to look for it** (Steps 3.2-3.3)
- the bar cleared, and the layers/instruments checked (Steps 3.4-3.5)
- the evidence, and the remediation

The first two are not ceremony. Without them a sweep that skipped Step 3 produces a byte-identical
report to one that performed it, and the difference between those two sweeps is the entire point.

**Fixing the site is half a finding.** Name the gate that prevents recurrence — a ratcheted check, an
enumeration that fails the build on an unclassified sibling, a derived number instead of a typed one.

```
SLOP SWEEP — <repo> @ <sha>
Skill:         <version>+<digest>     (skill_version.py check)
Lexicon:       <version>              Families ref: read | NOT READ
Tier:          1 | 1-2 | 1-3
Scope:         N files (M non-test source; excluded: <vendored/generated>)
Applicable:    run: <families> · N/A: <family + language reason> · not run: <family + reason>
Findings:      <n> VERIFIED  (<b> blocking · <f> fix-or-file · <n> note)
               <n> SUSPECTED (excluded from the rate)
Rate:          <severity-weighted, VERIFIED only> per 1,000 non-test source  [denominator: M]
Checked-clean: <class — method used>
Near-misses:   <candidate — what refuted it>
H-coverage:    <r>/<R> required · <d>/<D> deferred attested   (gate.py verify)
Blind spots:   <n> changed files no H signal reached (<w> waived)  [baseline: <ref> | n/a]
Coverage:      repo <r>/<R> source files read (<p>%) · plan <d>/<D> dispositioned
               <w> waived (counted as unread) · <u> unclassified
Still open:    <the work queue, or "nothing">
```

**Omit the `Rate:` line by default. Emit it only when all four hold:** the repo is in a validated
language, the denominator is ≥100 non-test source files, the tier is recorded, and you can state the
finding unit you applied. Otherwise print `Rate: n/a (<reason>)`. The metric has been wrong once
already, and a number is harder to retract than a blank.

**A rate from an unvalidated language is structurally zero, not clean.** Findings there are SUSPECTED
until reproduced, and SUSPECTED is excluded from the rate — so a Solidity, Terraform, Python, Rust,
Kotlin or C# sweep reports `0.0` by construction. **Never compare a validated-language rate against an
unvalidated-language one**, and never let a structural zero enter a cross-repo table.

**Name the tell if it applies.** A sweep reporting SUSPECTED findings with **zero VERIFIED** has verified
nothing — say that on the face of the report rather than letting a `Rate: 0.0` read as a clean result.
Likewise a settled accounting over a tier that skipped families is settled *for that tier*, and must say
which. **A high `coverage.plan` beside a low `coverage.repo` means the enumeration was narrow, not that
the repo is clean** — say that in words when the two disagree, because a reader who sees one full
fraction will generalise from it.

Rates below ~100 source files are noise — one finding moves them 13-50 points. **Publish the denominator
with every rate, or omit the rate.** Never compare rates computed under different lexicon versions or
different tiers.

The block above is the text half. **The report is the other half, and it is Step 5.**

---

## Step 5 — build the report (`/slop-ferret:report`)

**Run `/slop-ferret:report`. A sweep is not finished without it.** The text verdict block is for the
record; the report is what a maintainer actually reads, and it is the only artifact that shows coverage,
calibration and near-misses together.

It must carry, in this order: **both coverage fractions in the banner** — `coverage.repo` and
`coverage.plan`, each with its denominator, and never one without the other; **what was and was not covered** before
any result; findings **severity-first, never volume-first** (count runs inverse to severity — the largest
class in the first campaign was 7,022 occurrences and cosmetic); VERIFIED and SUSPECTED **visually**
distinct rather than captioned; every rate beside its denominator, with the rate suppressed below ~100
non-test source files; the near-misses you refuted; checked-clean with the method; and the lexicon
version and SHA.

**The report is a self-contained local `.html` file — all CSS and JS inline, no external calls — handed
back with `SendUserFile`. Never publish it, never mint a hosted URL, never commit it into the target
repo.** `Artifact` is deliberately not granted, for the same reason `Edit` is not: a prose rule the
runtime cannot enforce is not a rule. Publishing is the operator's decision, taken in the moment and
never inherited from an earlier sweep — if they ask for it, they can publish the file they were handed.

That rule is stated **here** and not only in the command file, because on 2026-08-01 it lived only in
the command — which was not registered, so it could not load — while `Artifact` was granted. A sweep
that fell back to "build the report to that spec anyway" therefore had the publishing tool and no
prohibition. This skill's own principle caught it: *a gate in a file that might not be read is not a
gate*, and that applies to this file's own dependencies, not just to the repos it sweeps.

The command file holds the full spec. If you are running the sweep without the command available, build
the report to that spec anyway — the requirement is the report, not the command.

## Step 6 — file (only after confirming the destination)

**Determine the tracker, then ask.** Check `git remote -v`; the repo for `.linear/` or issue keys in
commit subjects (`fix(abc-123):`); `CONTRIBUTING.md`; and existing issues for house conventions. **A
project uses one tracker, not a mixture** — three issues went to GitHub on a Linear-tracked project
because this was not checked.

Then **confirm with the operator**: which tracker, which team or repo, and — for anything on a crypto,
auth, signing or key-handling path — whether it may be filed publicly at all. Wait for the answer.

**File VERIFIED findings only.** SUSPECTED leads are reported to the operator, never filed unbidden.
Match the house style of existing issues. Carry the VERIFIED label into the body so a reader knows the
refutation was sought.

---

## Step 7 — write back

New or amended classes → **the lexicon** (`references/ai-slop-lexicon.md`), with all three fields and
provenance, `status: draft` if new. There is nothing to mirror: the vault path is a symlink to that file.
Counts, denominator, SHA, tier, lexicon version, checked-clean results, near-misses **and where the
report file lives** → the target's sweep record. Never counts in the lexicon; never universal classes on
a repo page.

**Record the sweep boundary as a SHA that resolves in the target repo, and check that it does before
writing it down.** Two prior sweeps recorded boundaries (`c48904b9`, `0afd2ef2`) that no longer exist as
objects, both taken from `-dirty` code maps, one of them under a note claiming the denominator was
re-derivable. A boundary nobody can resolve makes the next sweep unable to scope itself, which is how a
whole-repo re-read gets spent re-covering ground.

**Only write cross-repo comparison data for repos the operator owns.** Do not copy a client's or
employer's unpatched findings into a personal store.

---

## Discipline

**Do not soften a finding because you wrote the code.** In one audit, 11 of 22 findings were the auditor's
own and 9 had been written that same day.

**Clean is a correct outcome.** An empty result is not evidence you have not looked hard enough. Say what
you covered and how, so "clean" is checkable.

**Prefer the harder finding — after it clears Step 3, never instead of it.** Repeated sweeping drives the
rate down and leaves the classes no automated check can see. That is a statement about where to look
next, not a licence to file the unverifiable.

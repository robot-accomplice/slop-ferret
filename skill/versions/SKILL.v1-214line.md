---
name: slop-ferret
description: Sweep a repository for AI slop — work that LOOKS finished and is not. Dead-on-arrival features, tests that cannot fail, guards that cannot fire, fabricated claims, duplicated rules, architectural drift, and latent "almost right" defects. Use when asked to audit, sweep, or hunt slop in a codebase; as a pre-release gate on a frozen branch before soak; or after any large single-session burst of work. Always begin with Step 0 — load the AI-slop lexicon from the knowledge vault and refresh it against current web definitions — and always write findings back.
---

# slop-ferret

One question about every line in scope:

> **Does this do what it says it does, and does anything reach it?**

Slop is not style and it is not a linter's job. Slop is work that **looks finished and is not**: a
feature nothing calls, a test that cannot go red, a comment describing behaviour the code lacks, a
guard whose condition can never be true, a metric measuring something other than its name.

**Language-agnostic.** Every class below has been observed in Go, TypeScript and Kotlin. Where a
detection needs a language-specific tool, the *class* is the durable thing; the grep is not.

---

## Step 0 — load the language, then refresh it (MANDATORY, both, before reading any code)

A sweep finds only what its vocabulary can name, so the vocabulary is loaded first and fed from
outside itself.

**1. Read the lexicon.** `~/Claude Vault/wiki/practices/ai-slop-lexicon.md` — 40 classes in 8
families, each with a definition that decides membership, a **discriminator** against its nearest
neighbour, a detection method, and a severity. Then read this repo's own sweep page if one exists
(`.../<repo>/slop-sweep-<date>.md`) for prior counts and, more importantly, **the classes recorded
CLEAN with the method used** — do not re-spend budget there.

**2. Search the web for how AI slop is currently defined**, and diff what you find against the
lexicon. This is not ceremony. On 2026-07-23 it contributed **nine classes the internal registry
could not name**, three of which measured non-zero on their first run — after the registry had
already been built from two prior audits of the same repo.

**3. Write back at the end.** New class → the lexicon, with all three fields and provenance. Counts
and clean-with-method results → that repo's sweep page. Never counts in the lexicon; never universal
classes on a repo page. A sweep that discovers a class and does not record it has paid for the
discovery and kept none of it.

---

## Step 1 — scope, and fix the depth

**Scope.** Whole repo by default. As a release gate, `git diff <last-release-tag>..HEAD`. If the diff
is too large for one pass, split by directory and cover all of it — a partial audit reported as
complete is itself slop.

**Depth is fixed, not discretionary.** Sweeps are being accrued across repos to fit a quality rubric
later, and unequal depth is what makes that impossible. Every sweep runs **all eight families** and
records a result for each, including "clean, by this method". Budget proportional to **non-test source
files**, not to total files.

**Report rate, not count.** Findings per 1,000 non-test source files, severity-weighted
(**blocking×5, fix-or-file×2, note×1**). Raw counts across repos of different sizes produce a
backwards ranking — this has already happened once.

---

## Step 2 — the eight families

Full definitions and discriminators are in the lexicon. This is the running order, cheapest first.

### A. Fake-done — looks finished, does nothing
Dead on arrival · tautological self-proof · constant stub · unfinished-work marker · gated empty seam.

For every new exported symbol, config key, route, table and tool: find a **non-test caller**, then
follow the chain to a real entry point. Zero callers → dead. A config key needs a **read**, not a
definition.

Two traps that make this class invisible to tooling:

- **Strip comments AND compile-time satisfaction assertions before counting references.** A doc
  comment names its own symbol, so any `count <= 1` filter finds nothing without the strip. Measured:
  1 → 1 → 15 dead symbols as each strip was added.
- **Unused-symbol tools assume exported means externally consumed.** False for anything under
  `internal/`, or any private module. The entire exported surface goes unchecked.

Then **mutation-prove the wiring**: neutralise the call site, run the owning package's tests, confirm
something goes red. Grep decays; a later commit removes the call and every test stays green.

### B. False claim — asserts something untrue
Fabricated claim · inert test · mirrored test · documented-vs-implemented drift · phantom dependency ·
invented metric.

Every "always / never / regardless / cannot" omits a precondition — find it. Open every test whose
name makes a claim and confirm it drives the function it names. **Recompute at least one reported
number from raw data.** Verify doc counts against source.

*Phantom dependency:* resolve unfamiliar imports against the **manifest and lockfile**, never against
a build error. A stale local install looks exactly like a hallucinated package — one check away from a
false finding.

### C. Unreachable — cannot run
Dead guard · sited guard · orphan computation.

For each guard, find the code that makes its condition true. For each cap, find the test that exceeds
it. **For each guard, enumerate the siblings** — other callers of the same sink. A protection applied
where the author was looking is the single most repeated defect shape across every repo swept.

**Before writing any C or H finding up, read the DDL.** Three ledger candidates in one payments repo
looked like real defects in the code and were all stopped by database constraints. Enforcement in the
schema is *better* than enforcement in the service — it holds against every writer. The inverted
finding is the real one: an invariant enforced only in application code where a constraint could hold
it.

### D. Repetition
Duplicated implementation · copy-paste family · synonym helper · magic literal.

Normalised-body hashing across packages; sliding-window hash for copy-paste families. **A comment
explaining why two copies are identical is not a justification** — it is the drift warning. Check
whether any test pins the copies to each other; usually none does.

### E. Inflation — more structure than the problem has
Single-impl interface · speculative surface · over-defensive boilerplate · helper sprawl · blind
generics · inappropriate concurrency · god function · deep nesting.

The test: **name the caller that needs the flexibility.** "A future one" is speculation.

**Exempt consumer-declared narrow ports before reporting.** An interface declared in the module that
consumes it, naming only the methods it uses, is interface segregation done right. Applying the
single-impl rule bluntly produced 23 hits and zero real findings in one repo.

### F. Provenance residue — reads as machine-written
Register tells (em-dashes) · comment restating code · ceremonial structure · cross-language idiom ·
placeholder naming.

**Never blocking. Separate runtime output from comments before reporting a count** — the difference
between "7,022 em-dashes" and "14 em-dashes in operator-facing error text" is the whole finding.
Sweep operator- and user-facing copy; ratchet the rest.

### G. Drift & staleness — was true once
Stale reference · phantom backlog · stale dependency · orphaned artefact · **mandate drift** · broken
gate.

Run every gate the repo declares and record whether it currently passes. Enumerate each stated
architectural MUST/SHOULD and find its enforcement — a mandate that is documented, scaffolded, and
0% implemented is worse than an absent one, because the scaffold reads as compliance.

**Sweep just outside the gates' scan roots.** A gate defines a boundary and slop accumulates
immediately outside it. The better the gate, the more precisely it tells you where to look.

### H. Latent defect — runs, and is wrong
"Almost right" logic · command-query mixing · silent failure.

**This is the expensive family and the one that pays.** It cannot be scanned for. Read the
highest-consequence paths by hand, assuming they are wrong:

> money and ledgers · auth and session · crypto and signing · migrations · anything persisted ·
> anything parsing untrusted input

On a payments repo the mechanical families returned **nothing** and three of four findings came from
reading the ledger. Target boundaries: empty input, single element, off-by-one, zero and negative,
timezone, non-ASCII and non-BMP text, integer width, and scale — a value correct at today's volume
that degrades linearly is still a defect.

**Reproduce RED before claiming.** Drive the real function at its real call-site shape, in a temporary
probe inside the real package. Delete the probe afterwards and verify the tree is clean.

---

## Step 3 — verdict

Report **most severe first**, each with `file:line`, the class from the lexicon, the evidence, and the
remediation. Then:

```
SLOP SWEEP — <repo> @ <sha>
Scope:        <whole repo | tag..HEAD>, N files (M non-test source)
Families run: A B C D E F G H   (all eight, or say which were skipped and why)
Findings:     <n> blocking · <n> fix-or-file · <n> note
Rate:         <severity-weighted> per 1,000 non-test source files
Clean:        <classes checked and empty, WITH the method used>
```

**After every sweep, recompute and report the cross-repo stats table** in
`~/Claude Vault/wiki/practices/adversarial-review-predicts-cleanliness.md`: the severity-weighted
rate for every repo swept so far, plus whether each candidate churn signal still tracks. Signals
die as n grows — churn concentration survived n=3 and broke at n=4 — and that is only visible if
the table is refreshed each time rather than at the end. Attach the denominator to any rate
computed on fewer than ~100 source files; below that a single finding moves it 13-50 points.

**Blocking:** A, B, C, and H. A dead feature, a false claim, a test that cannot fail, a guard that
cannot fire, and code that is quietly wrong all ship as capability.
**Fix-now-or-file:** D, E, G. **Note:** F.

**Fixing the site is half a finding.** Every remediation names the gate that stops recurrence — a
ratcheted check, an enumeration that fails the build on an unclassified sibling, a derived number
instead of a typed one. Four separate hand-fixes of the same defect, each at the site in front of the
author, is a real observed history.

---

## Discipline

**Verify before filing.** Two false-positive classes have already been caught mid-sweep: a detector
whose receiver regex could not see anonymous receivers (5 findings, all false), and a stale local
install that looked exactly like a phantom dependency. **Report the false-positive rate.** A class
that accuses working code is worse than a class that misses something.

**Do not soften a finding because you wrote the code.** In one audit, 11 of 22 findings were the
auditor's own and 9 had been written that same day. Your own recent work is the highest-yield place to
look, not the place to skip.

**Say what came back clean, with the method.** "Clean" is only useful if someone can check it and the
next sweep can skip it.

**A single writer per repo.** If another session owns the tree, you are read-only: file issues, do not
implement. Say so plainly if asked.

**Prefer the harder finding.** Repeated sweeping drives the rate down and leaves the classes no
automated check can see. In the most-swept repo of three, every blocking finding was invisible to
every gate it already had.

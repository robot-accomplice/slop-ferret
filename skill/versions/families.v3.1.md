# Families — elaboration

**This file contains no gates.** Every rule that prevents a wrong finding — the applicability table, the
five bars, the verify step — lives in `SKILL.md`, because a gate in a file that might not be read is not
a gate. What follows helps you find **more**: the full class lists, and the detail that makes a family
productive rather than safe.

Severity is never here. It is the lexicon's per-class column and nowhere else.

---

## H · Latent defect — runs, and is wrong  *(tier 1)*
*"Almost right" logic · command-query mixing · silent failure.*

4 of 7 blocking findings across six repos, and 4 of the last 4. The target list and boundary checklist
are in `SKILL.md`; what follows is how to actually read.

Pick the path by consequence, not by suspicion — the question is *"if this were wrong, what would it
cost?"*, not *"does this look odd?"*. Then read it as though a competent author wrote it under time
pressure and got one thing subtly wrong, because that is what happened.

Shapes that recur:

- **A limit that is announced but not enforced.** A `maxMessage` that sets a flag and keeps reading; an
  error constant declared for an enforcement never written. Check what happens *past* the limit.
- **A parser more permissive than its standard, in the direction that hides data.** Compare against the
  language's own stdlib implementation — it is a free oracle and it disagrees more often than you expect.
- **Correct-at-this-volume.** A 24-bit identifier space with no retry; an index that is fine until the
  table grows. Compute the failure rate at 10× and 100× today's scale.
- **The product's own promise as the oracle.** If the README says a signal that cannot be collected is
  reported as a gap, find the path where it is reported as clean instead. This produced the sharpest
  finding of six sweeps.

## G · Drift & staleness — was true once  *(tier 1)*
*Stale reference · phantom backlog · stale dependency · orphaned artefact · mandate drift · broken gate.*

The MUST/SHOULD enumeration is in `SKILL.md`. Where the yield actually is:

- **Just outside the gates' scan roots.** If a marker check scans `internal/`, `cmd/`, `scripts/`, then
  `docs/` is where the markers are. One repo had 40 stale `TODO` rows in a single file for that reason.
- **References to a system that no longer exists.** A rewrite leaves comments explaining the new code by
  pointing at the old. 280 of them in one repo, to a codebase deleted six releases earlier.
- **Generated artefacts committed without a freshness gate.** Compare the artefact against a fresh
  regeneration; drift in both directions is common and neither is visible from the file.
- **A register of open items whose items are closed.** Spot-check four rows. If all four are done, the
  register is unmaintained and is a phantom backlog, not a to-do list.

## A · Fake-done — looks finished, does nothing  *(tier 2)*
*Dead on arrival · tautological self-proof · constant stub · unfinished-work marker · gated empty seam.*

The refuter list is in `SKILL.md`. Method notes:

- **Strip comments and compile-time satisfaction assertions before counting references.** A doc comment
  names its own symbol, so a `count <= 1` filter finds nothing without the strip. Measured on one repo:
  1 → 1 → 15 dead symbols as each strip was added.
- **Unused-symbol tools assume exported means externally consumed** — false for any private module, so
  the whole exported surface goes unchecked by default.
- **Tautological self-proof** is the subclass no tool can see: a file that declares an interface,
  declares the accessors satisfying it, and asserts its own satisfaction is referenced by everything and
  used by nothing.
- A config key needs a **read**, not a definition. Grep for the read.

## B · False claim — asserts something untrue  *(tier 2)*
*Fabricated claim · inert test · mirrored test · documented-vs-implemented drift · phantom dependency ·
invented metric.*

Bars are in `SKILL.md`. Where to look:

- Every "always / never / regardless / cannot" omits a precondition. Find it.
- Open every test whose **name** makes a claim and confirm it drives the function it names. One tested a
  parallel implementation used only by a benchmark runtime while the path it claimed to cover had none.
- Recompute at least one reported number from raw data. A "43% churn" figure was 15-22%: the query
  counted twelve identical no-argument reads as duplicates of each other.
- Verify doc counts against source — migrations, tables, endpoints, stages. They drift silently and a
  reader has no way to know.

## D · Repetition  *(tier 2)*
*Duplicated implementation · copy-paste family · synonym helper · magic literal.*

A comment explaining why two copies are identical is not a justification — it is the drift warning, and
the comments usually drift before the code. Check whether any test pins the copies together; usually
none does. Layer-crossing copies (a sync worker and a vendor client) drift faster than sibling ones,
because different concerns own them.

## C · Unreachable — cannot run  *(tier 3)*
*Dead guard · sited guard · orphan computation.*

The layer-enumeration bar is in `SKILL.md`. **Sited guard is the most repeated defect shape across every
repo swept**: a protection applied where the author was looking rather than to the class. Whenever you
find one, grep the sink for its siblings before writing anything — the finding is usually "this is fixed
at 6 sites and missing at 24", not "this is broken here".

## E · Inflation — more structure than the problem has  *(tier 3)*
*Single-impl interface · speculative surface · over-defensive boilerplate · helper sprawl · blind
generics · inappropriate concurrency · god function · deep nesting.*

Name the caller that needs the flexibility; "a future one" is speculation. **Exempt consumer-declared
narrow ports** — an interface declared in the module that consumes it, naming only the methods it uses,
is interface segregation done right in any language. Applied bluntly once, this family produced 23 hits
and zero real findings, which is the worst ratio any family has recorded.

## F · Provenance residue — reads as machine-written  *(tier 3, never blocking)*
*Register tells · comment restating code · ceremonial structure · cross-language idiom · placeholder
naming.*

Separate runtime output from comments before reporting any count — the difference between "7,022
em-dashes" and "14 in operator-facing error text" is the entire finding. The em-dash count is nearly
always dominated by comments and docs; the interesting number is the one that reaches a human.

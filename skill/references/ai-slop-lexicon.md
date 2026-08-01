---
type: topic
version: 2026-08-01.1
tags: [ai-slop, code-review, taxonomy, ubiquitous-language, code-quality]
---
# AI-slop lexicon — the ubiquitous language

**This is the definitional registry, not a tag list.** A class earns a place here only when it has a
definition that decides membership, a **discriminator** separating it from its nearest neighbour, and
a detection method someone else can run. A label without those three produces findings nobody can
argue with, which is the same as findings nobody can act on.

Governs the sweep in [[pre-release-slop-audit]]. Counts are never recorded here — they are per-repo
and per-run, and live on the project's sweep page ([[projects/personal/roboticus/slop-sweep-2026-07|roboticus sweep]]).

**Severity is a property of the class, not of the count.** Measured inverse correlation: the two
blocking findings in the 2026-07-23 roboticus sweep sat at 33 and 8 occurrences; the largest class,
at 7,022, was cosmetic. **B** = blocking, **F** = fix-or-file, **N** = note only.

**Adding a class.** New classes arrive from a sweep that found something the registry could not name,
or from the standing web check for current definitions. Give it all three fields, record provenance,
and state the discriminator against the closest existing entry — if you cannot, it is that entry.

**A new class enters with `sev` = `N (draft)` and stays there until a sweep has applied it.** That cell
is the carrier for the draft cap — there is no separate `status` field, and there was no carrier at all
until 2026-08-01: the skill's Step 0.3 required new classes to land as `status: draft`, capped below
blocking, against tables whose only columns are term/definition/discriminator/detection/sev. A rule
wired to a field that does not exist is this registry's own *gated empty seam*. Drop the marker when a
sweep has used the class and the class survived — see *A class is not validated until it has been
applied*, where two of the first batch were wrong on first use.

---

## A. Fake-done — looks finished, is not
*The defining property: it would pass review, and it does nothing.*

| term | definition | discriminator | detection | sev |
|---|---|---|---|---|
| **Dead on arrival** | Built, tested, documented; zero production call sites | vs *Unreachable*: the code is fine, nothing invokes it | call-site grep, then refute the universal negative (see families.md) — mutation-proof only on a copy outside the tree | B |
| **Tautological self-proof** | Declares an abstraction, declares what satisfies it, asserts its own satisfaction | vs *Dead on arrival*: tools report it **live** → [[dead-code-that-proves-itself-alive]] | identifier index with comments **and** satisfaction assertions stripped | B |
| **Constant stub** | Function whose body unconditionally returns a literal/zero value | vs *Dead on arrival*: it **is** called, and answers nothing | AST: single-return bodies with no parameter reference | B |
| **Unfinished-work marker** | TODO/FIXME/HACK/XXX/STUB: a deferral with no owner, date, or issue | vs *Stale doc*: marks work never started, not work since done | regex, ratcheted to zero | F |
| **Gated empty seam** | Config flag, route, or hook wired to an unimplemented path | vs *Speculative surface*: it is **reachable** and returns nothing | grep each new key for a **read**, not a definition | B |
| **Swallowed advisory** | Warning or event raised correctly into a sink that is muted at that point in the lifecycle, so it reaches nobody | vs *Silent failure*: the error IS surfaced — the delivery context defeats it, and the raising code is correct in isolation | for each warn/emit, ask what the sink is doing AT THAT MOMENT (boot suppression, disabled level, nil writer), not what it does at steady state | B |

## B. False claim — asserts something untrue
*The defining property: a reader who trusts it is misled. Load-bearing when an agent reads it back.*

| term | definition | discriminator | detection | sev |
|---|---|---|---|---|
| **Fabricated claim** | Prose describing behaviour the code lacks | vs *Stale reference*: was never true, rather than true once | every "always/never/regardless" omits a precondition — find it | B |
| **Inert test** | Test whose assertions pass both before and after the change | vs *Mirrored test*: cannot fail at all → [[a-green-mutation-proves-nothing]] | mutate the behaviour **on a copy outside the tree**; confirm **that** test goes red for the right reason; if you cannot, the finding is SUSPECTED | B |
| **Mirrored test** | Expectation recomputed by the implementation under test | vs *Inert test*: fails, but only if the code disagrees with itself | `want` and `got` derived from the same function | B |
| **Documented-vs-implemented drift** | A stated count, shape, or flow that the code contradicts | vs *Fabricated claim*: drifted since writing, not invented | recompute every number in the docs from source | F |
| **Phantom dependency** | Import, package, or API that does not exist | vs *Stale dependency*: never existed, rather than deprecated | resolve unfamiliar imports against the registry, not model confidence | B |
| **Invented metric** | Reported figure whose query measures something other than its name | vs *Documented drift*: the number is current and still wrong | recompute one figure from raw rows | B |
| **Rendering-as-fact** | A display sentinel or summary (`unset`, `n/a`, `"3 entries"`, a zero meaning "auto") consumed downstream as an authoritative value | vs *Invented metric*: the datum is real — what is wrong is that a PRESENTATION of it was read as the datum | for every placeholder a generator emits, find each consumer that compares against it rather than against "was anything authored" | B |
| **Wiring-blind test** | Test that exercises a unit directly and still passes when the unit's only call site is deleted | vs *Inert test*: it CAN fail — mutate the unit and it goes red — but it cannot see that nothing invokes the unit | delete the call site, not the logic; if the suite stays green, the wiring is unproven | B |

## C. Unreachable — cannot run
*The defining property: a code path exists whose precondition nothing establishes.*

| term | definition | discriminator | detection | sev |
|---|---|---|---|---|
| **Dead guard** | Protective branch whose condition no code path makes true | vs *Dead on arrival*: it is on a live path and silently never fires | for each `if`, find the writer that sets it | B |
| **Sited guard** | Protection applied at one call site of a sink with several | vs *Dead guard*: it fires — for one sibling → [[enumerate-the-class-not-the-site]] | enumerate all callers of the sink | B |
| **Orphan computation** | Result computed every cycle and discarded | vs *Dead on arrival*: the producer runs; the consumer is missing | trace each producer to a consumer (a `switch` with no case) | B |

## D. Repetition
*The defining property: one rule, more than one home, no gate keeping them equal.*

| term | definition | discriminator | detection | sev |
|---|---|---|---|---|
| **Duplicated implementation** | Same logic in ≥2 packages, no test pinning them together | vs *Copy-paste family*: independent copies, not a template | normalized-body hashing across packages | F |
| **Copy-paste family** | N call sites sharing a verbatim block, grown by copying the last one | vs *Duplicated implementation*: one shape, N instances | sliding-window hash of normalized lines | F |
| **Synonym helper** | Second helper whose name is a synonym of an existing one | vs *Duplicated implementation*: names differ, so grep misses it | cluster utility names by meaning, not string | F |
| **Magic literal** | Value repeated inline where a constant should own it | vs *Duplicated implementation*: data, not logic | grep repeated string/number literals in structured positions | F |
| **Hand-copy of generated truth** | Hand-maintained document restating facts a generator already derives from source, with no gate holding them equal | vs *Duplicated implementation*: facts and prose, not logic — and one copy is authoritative by construction | diff the hand copy against the generated one field by field; every disagreement is the hand copy being wrong | F |

## E. Inflation — more structure than the problem has
*The defining property: name the caller that needs the flexibility. "A future one" is speculation.*

| term | definition | discriminator | detection | sev |
|---|---|---|---|---|
| **Single-impl interface** | **Producer-side** interface with one implementation and no second in prospect | vs *Tautological self-proof*: genuinely used, just needless. **Exempt: a consumer-declared narrow port** (declared in the package/module that consumes it, naming only the methods it uses) — interface segregation done right. Language-independent: the idiom in Go, and the same exemption applies to TypeScript, Kotlin and anywhere else the consumer owns the type | count implementations per interface, then **discard every consumer-declared one before reporting** | F |
| **Speculative surface** | Knob, parameter, or hook no caller varies | vs *Gated empty seam*: it works, nobody needs it | every caller passes the same value | F |
| **Over-defensive boilerplate** | Validation for states the types or upstream guarantees prevent | vs *Sited guard*: guards nothing rather than guarding narrowly | checks duplicating an upstream invariant | F |
| **Helper sprawl** | Logic split into single-use private functions, deepening the stack | vs *Speculative surface*: no flexibility claimed, just fragmented | private functions with exactly one caller | N |
| **Blind generics** | Type parameters that neither improve safety nor remove boilerplate | vs *Single-impl interface*: parametric rather than nominal | generic declarations instantiated at one type | F |
| **Inappropriate concurrency** | Goroutines/channels where synchronous code is faster and debuggable | vs *Speculative surface*: costs correctness, not just clarity | concurrency primitives with no contention or latency motive | F |
| **God function / oversize file** | Unit exceeding the project's stated complexity or line cap | vs *Helper sprawl*: the opposite failure — too little decomposition | line and cyclomatic thresholds | F |
| **Deep nesting** | Control flow nested past the readable depth | vs *God function*: short but unreadable | indentation depth | N |

## F. Provenance residue — reads as machine-written
*The defining property: harmless to execution, corrosive to trust. Never blocking.*

| term | definition | discriminator | detection | sev |
|---|---|---|---|---|
| **Register tells** | Em-dashes and LLM cadence in prose | vs *Comment bloat*: style, not volume | character and phrase frequency; **separate runtime output from comments** | N |
| **Comment restating code** | Comment that loses no information when deleted | vs *Register tells*: content, not tone | does it say *why*, or narrate the line below? | N |
| **Ceremonial structure** | Section banners, headers, and scaffolding with nothing under them | vs *Comment restating code*: decorates rather than narrates | rule-art headers; comment:code ratio > 1 | N |
| **Cross-language idiom** | Another language's patterns transplanted (Java-isms in Go, etc.) | vs *Inflation*: wrong dialect, not wrong amount | naming suffixes, ctor ceremony, wrong-language syntax | F |
| **Lint-escape** | Suppression directive (`//nolint`, `# noqa`, `@SuppressWarnings`, `eslint-disable`) silencing a check rather than fixing it | vs *Ceremonial structure*: hides a signal rather than adding noise. **A directive with a stated reason on a verified-correct construct is NOT this class** — 37 of 43 in one repo were legitimate typed-nil suppressions | count directives; read each one's justification; report only the unjustified | N |
| **Placeholder naming** | `tmp`, `data`, `obj`, `foo` surviving into committed code | vs *Comment restating code*: names, not prose | identifier denylist | N |
| **Live data as fixture** | Real personal or production values (names, addresses, employers, hostnames, account IDs) embedded as test data because they were in the authoring context | vs *Placeholder naming*: the opposite failure — too REAL rather than too generic. Severity rises to **B** in a public repository | grep fixtures for the operator's own identity and for third-party proper nouns; a PII-guard test asserting on the real value is the signature case | F |

## G. Drift & staleness — was true once
*The defining property: correct when written. Distinguishes this family from **B** entirely.*

| term | definition | discriminator | detection | sev |
|---|---|---|---|---|
| **Stale reference** | Points at a system, file, or release that no longer exists | vs *Phantom dependency*: existed once | resolve every cited path against the tree | F |
| **Phantom backlog** | Register of open items whose items are closed | vs *Unfinished-work marker*: has an owner and a home; the **status** is false | spot-check rows against code; ≥1 stale ⇒ unmaintained | F |
| **Stale dependency** | Deprecated or superseded API still in use, no recorded exemption | vs *Phantom dependency*: real, just old | deprecation lints; weight by blast radius, not count | F |
| **Orphaned artifact** | Committed file nothing references | vs *Dead on arrival*: not code — assets, lockfiles, exports | reference-check every binary/asset against tracked text | N |
| **Mandate drift** | A stated architectural rule the code does not follow, with no exemption | vs *Documented drift*: the **rule** is unmet, not a description | enumerate each MUST/SHOULD; find its enforcement | B |
| **Broken gate** | A check the project defines and does not currently pass | vs *Mandate drift*: automated and red, not unenforced | read the declared gates; run only what the operator approved, and rule out a stale local checkout before calling one broken | B |

## H. Latent defect — runs, and is wrong
*Detected by adversarial reading, not by scanning. Included so a sweep records that it looked.*

| term | definition | discriminator | detection | sev |
|---|---|---|---|---|
| **"Almost right" logic** | Reads correctly, fails at boundaries (off-by-one, empty, timezone) | vs *Inert test*: the **code** is wrong, not the test | adversarial reading: empty input, single element, boundary | B |
| **Command-query mixing** | Query-named operation that mutates state | vs *"Almost right" logic*: correct, but misnamed into misuse | `Get`/`Is`/`List` bodies containing writes | F |
| **Silent failure** | Error path that neither surfaces nor records | vs *Over-defensive boilerplate*: too little handling, not too much | discarded errors; **exclude idiomatic cleanup discards** | B |
| **Probe of the wrong subject** | Health, readiness or liveness check that confirms something other than the thing it gates | vs *Dead guard*: it fires and returns TRUE — it is measuring the wrong subject, not failing to fire | for each probe ask what a FOREIGN process, a half-loaded service, or a bound-but-unready port would return | B |
| **Wrapper-only termination** | Stopping a supervising process leaves the real work running, reparented and unowned | vs *Silent failure*: nothing errors — the caller is told it succeeded | after any kill, enumerate surviving PIDs rather than trusting the exit; never match processes by name pattern, which also hits the watcher | F |

---

## Before reporting anything in C or H: check whether the schema already enforces it

**A defect that the database prevents is not a defect, and a sweep that reads only code will report it
as one.** Three candidates in one payments repo's ledger — a posting to a non-existent account, a
fractional minor unit, an unbalanced posting set — all looked real in the code and were all stopped by
Postgres:

| looked like | actually stopped by |
|---|---|
| unchecked `rowCount` on a balance `UPDATE` | `account_id NOT NULL REFERENCES accounts(id)` |
| only a `<= 0` check on an amount | `amount_minor BIGINT CHECK (amount_minor > 0)` |
| float-summed balance assertion | every input already an integer from the money constructor |

Enforcement living in the schema is **better** engineering than enforcement in the service — it holds
against every writer, not just the one you read. So the finding is inverted: an invariant enforced
*only* in application code, where a constraint could hold it, is the thing worth reporting.

Cost of skipping this check: three false findings against correct code, in the one repo where a false
alarm on the ledger is most expensive.

**Detection:** for every guard, cap or invariant in a persistence path, read the DDL before writing it
up. Applies to `Dead guard`, `Silent failure`, `Over-defensive boilerplate` and every entry in **H**.

## A class is not validated until it has been applied

Two of these were wrong on first use, and both corrections came from running them, not from writing
them. Assume a new entry is a draft until a sweep has used it.

- **`Single-impl interface` had the wrong discriminator.** As first written it flagged 23 constructs
  in roboticus, and **every one was idiomatic**: consumer-declared narrow ports, test/fixture seams,
  or genuine boundaries. Zero real findings. The producer/consumer split above is the repair.
- **The detector was wrong before the code was.** A first pass reported 5 interfaces with zero
  implementations; **all 5 were false**, from a receiver regex that required a named receiver and so
  could not see `func (typeName) Method()`. Report the near-misses you caught, as cases not as a rate (a rate over self-caught errors improves as the auditor gets sloppier) — a class that accuses
  working code is worse than a class that misses something → [[a-green-mutation-proves-nothing]].

## Provenance

Families **A-E** and **G** derive from measured roboticus sweeps (2026-07-22 gate, 2026-07-23
whole-repo). **F** and **H** merged in from the 2026-07-23 external check:

- [AI Slop Has a Shape](https://lobsterone.ai/blog/ai-slop-patterns/) — field taxonomy; contributed
  *synonym helper*, *over-defensive boilerplate*, *phantom dependency*, *"almost right" logic*.
- [AI-SLOP-Detector](https://github.com/flamehaven01/ai-slop-detector) — 27-check catalog; contributed
  *constant stub*, *cross-language contamination*, *placeholder naming*, *deep nesting*,
  *lint-escape*.
- [What Is AI Slop — Larridin](https://larridin.com/developer-productivity-hub/what-is-ai-slop-detect-prevent-low-quality-ai-code)
  — the five-signal index (duplication ratio, 30/90-day revert rate, complexity-adjusted analysis,
  architectural coherence, test behaviour coverage). **Revert rate is the one signal here nothing else
  covers** and it is not yet a class — it needs git history, not a scan.
- [AI builds, We Analyze (arXiv 2601.16839)](https://arxiv.org/pdf/2601.16839) — empirical study of
  AI-generated build code; wildcard dependency versions, missing error handling, deprecated and
  outdated dependencies dominate. Sharpened *stale dependency*.

Nine classes came from outside; three measured non-zero on their first run. The sweep can only find
what the lexicon can name, so the lexicon cannot be fed from the sweep — that is the whole argument
for the web half of Step 0.

**2026-07-26, roboticus v1.7.0 release work.** Seven classes added, every one from an instance the
registry could not name at the time it was found. All were found by RUNNING the thing, not by
scanning it, which is why none of them had a name: a scan sees the code, and each of these is a
defect in what the code REACHES.

- **Swallowed advisory** — boot runs inside a logging suppressor, so every advisory raised there was
  discarded, including one warning that an RCE-equivalent tool was ungated. Unit tests stayed green
  asserting the warning "fired", which it did. Recurred a second time in the same session, in a
  keyless-fallback warning that was itself the fix for another issue.
- **Rendering-as-fact** — a field manifest renders "nothing authored" as `unset` / `n/a` and a list
  default as `"3 entries"`. A grader consumed those as authoritative values and manufactured **37
  false failures in a 490-item release gate**, making a 100% bar unreachable regardless of the system
  under test. Two separate fixes were needed because the first was scoped too narrowly.
- **Wiring-blind test** — tests proving a detector worked while its only call site could be deleted
  with the suite still green. Caught by mutating the CALL SITE rather than the logic. This is the
  mechanism by which *Swallowed advisory* hid for so long.
- **Probe of the wrong subject** — a supervisor's port-only health check let a foreign process on the
  port make a dead child read as healthy, resetting the failure counter so backoff never engaged
  (measured: 847 flap lines in 3 minutes). The same fallacy then cost the auditor an hour when a
  model server bound its port before loading its weights.
- **Wrapper-only termination** — killing `go run` left the compiled child running and reparented, so
  two gate sweeps ran concurrently against one model. A `pkill -f` used to clean it up also matched
  the monitoring process, whose command line contained the pattern.
- **Hand-copy of generated truth** — a curated config guide restated defaults a generator derives
  from source. **19 of 90 shared settings disagreed**, including documented spend caps off by two to
  three orders of magnitude. Fixed with a gate that fails the build on disagreement, because
  correcting the 19 values without one just restarts the drift.
- **Live data as fixture** — the operator's real name, town and employer embedded as test fixtures in
  a PUBLIC repository, 34 occurrences of the name across 8 files. The signature case: the test proving
  a PII guard refuses to type the operator's name proved it by embedding the name six times.

The pattern across all seven: **the artifact was correct and its context defeated it.** That is
invisible to any check that reads the artifact alone, which is why the sweep needs a run-it half as
much as it needs a read-it half.

## Related
[[pre-release-slop-audit]] · [[dead-code-that-proves-itself-alive]] ·
[[enumerate-the-class-not-the-site]] · [[a-green-mutation-proves-nothing]] · [[projects/personal/roboticus/slop-sweep-2026-07|roboticus sweep]] · [[projects/work/outrider/sente/slop-sweep-2026-07|sente sweep]] · [[projects/work/outrider/safe/slop-sweep-2026-07|safe sweep]]

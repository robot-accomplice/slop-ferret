# Version history — why the backups exist

**STATUS 2026-08-01 — live is `SKILL.md`, 517 lines, stamped `2026-08-01.3`. Still DEMOTED
(`disable-model-invocation: true`). No full sweep has ever been run under any version after v1.**

This file is inside the digest (`skill_version.py`), unlike the archived cuts beside it. That is
deliberate and it is new: the cuts are history and archiving one must not force a stamp, but this file
is live prose about which version is current, and while it sat outside the digest it drifted for ten
days — claiming *"v2 (401 lines) is live and DEMOTED"* against a 484-line successor that had no row in
its own table. A blanket directory exclusion had turned the one file that records what is live into the
one file nothing checked.

| version | lines | outcome |
|---|---|---|
| `SKILL.v1-214line.md` | 214 | ABORT 2026-07-23: **5/5 NO-GO**. Evidence bar was prose at 92% depth, downstream of the filing instruction. **Every finding in the six-sweep record was produced by this version.** |
| `SKILL.v2-401line.md` | 401 | Rewritten against 14 conditions. Two stations flipped to GO-WITH-CONDITIONS; the adversary station returned **NO-GO** a second time, several findings *created by the previous round's fixes*. |
| `SKILL.v3.1-186line.md` | 186 | The cut, re-split along a gate/elaboration seam. **5/5 NO-GO** (second board), but 9 of 13 blocking findings were structural and present in v2 too. |
| — *(not archived)* | 484 | The merge of v2 and v3.1 that went live after the second board, stamped `2026-08-01.2+bd3bdaf6a853`. **Its content was not archived and no longer exists** — see below. |
| live `SKILL.md` | 517 | `2026-08-01.3`. The 484-line cut plus the 2026-08-01 audit repairs. Demoted. |

**Recorded fault: the 484-line cut was edited in place without archiving,** during the 2026-08-01 audit
remediation. Its digest is known (`bd3bdaf6a853`) and its content is not recoverable — this directory
exists precisely to prevent that, and the discipline failed the first time it was tested against
someone working quickly. Every change made to it is documented in-line in the live file and in the
audit section below, so nothing is unexplained; but the diff cannot be reconstructed. **Archive before
editing, not after.**

## What has and has not been executed

The distinction that mattered most to the second board — *"three rewrites, five stations, zero
sweeps"* — is now partly closed, and it is worth stating exactly how far.

- **Executed 2026-08-01:** the magma → `gate.py` seam, end to end, against real repositories for the
  first time. `magma` at `counterspy@531cc42` (map built in ~3s, `plan` exit 0, fidelity `rta`
  recognised, 12 candidates, 33 H-paths) and at `ghola@4f33b3c`. The 32-test suite. `skill_version.py`
  on its own tree.
- **Still not executed:** a full sweep. Families A-H have not been run under v2, v3.1 or the live file
  by anybody. The two pre-registered controls — `counterspy@531cc42` cold, then a repo with no sweep
  record — remain outstanding, and they remain the only thing that can settle whether the method still
  reaches. Deferred deliberately until the 2026-08-01 repairs landed: `ghola` could not reach a verdict
  at all before them (zero H-paths enumerated), and any COMPLETE from `counterspy` before them was
  uninterpretable, because the gate certified COMPLETE on a discharge that accounted for nothing.

## The 2026-08-01 audit

The live file was swept with its own method. Three defects were reproduced rather than argued, and all
three were in the parts of the skill that certify the *rest* of it:

1. **`verify` returned COMPLETE, exit 0, on a discharge generated mechanically from the plan** — no file
   opened, 12 candidates unexamined. The previous repair's filed-candidate clause only fires when a
   sweep files something, so the clean-sweep path was unguarded. Closed by requiring every candidate to
   be cleared *or* explicitly refuted.
2. **`/slop-ferret:report` was not a registered command**, so Step 5 mandated something that could not
   load — and the report's *do-not-publish* rule lived only in that unloadable file while `Artifact` was
   granted. Closed by registering the command and moving the prohibition into `SKILL.md`, with
   `Artifact` withdrawn on the `Edit` precedent.
3. **Tier 1 was unbounded and `ghola` enumerated zero H-paths.** roboticus required 387 hand reads
   before `verify` would return anything but INCOMPLETE; ghola, an HTTP client, matched no network
   vocabulary at all. Closed by a consequence-tiered worklist with a PARTIAL verdict, and a
   `network/untrusted-io` signal group.

## The open question v3 did not answer

**Is a short version adequate to surface what a long one found?** Still unknown, still not resolvable by
reading either document, and now less likely to be settled by argument than ever: the live file is
longer than every cut that has been reviewed. v2 and v3.1 are retained so the question can be settled
empirically. The resource-auditor station's judgement stands unrefuted and unacted-on — *five rules
would have produced every blocking finding across six repos plus all five near-misses.*

## How to settle it

`counterspy` @ `531cc42`, cold — without the prior sweep record in context. It has one blocking finding
(the `-0` chunk-size evasion, reproduced RED) and a recorded sweep page. Then a repo with no sweep
record at all — `ghola` or `go-facade-template` — because counterspy tests recall of a known answer and
a fresh repo tests whether the method reaches.

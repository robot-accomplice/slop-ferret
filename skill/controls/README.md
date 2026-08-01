# Cold control runs — brief for the executing session

**You are running a pre-registered control. Read this whole file before anything else.**

This skill has been reviewed four times and repaired repeatedly, and **no full sweep has ever been
run under any version after v1**. Every finding in its six-sweep record came from a 214-line version
that failed review 5/5. Two controls were pre-registered to settle whether the method still works.
This is that run. The point is to produce an honest result, **not a good one.**

---

## Do not open these files

Opening any of them invalidates the run. They are withheld deliberately, not by oversight.

- `~/Claude Vault/wiki/projects/personal/counterspy/slop-sweep-2026-07.md` — the prior sweep record
- `~/Claude Vault/wiki/decisions/2026-07-23-slop-ferret-global-promotion-abort.md`
- `~/Claude Vault/wiki/decisions/2026-08-01-slop-ferret-self-audit-remediation.md`
- `~/Claude Vault/wiki/decisions/2026-08-01-slop-ferret-control-preregistration.md` — the pass
  criteria and the operator's answer key

If you open one by accident, **say so on the face of your report and stop treating the run as a
control.** A contaminated run reported as clean is worse than no run, and it burns a control that
cannot be regenerated.

## Recorded deviation — required, state it in your report

`SKILL.md` Step 0.2 mandates reading the target's prior sweep record. **You are directed not to**,
because that record contains the outcome this run exists to test. So this run is non-compliant with
the skill by construction. That is intended, pre-registered, and must appear in your report as a
recorded deviation — not silently.

Nothing else in the skill is waived.

## What is being tested

**Control 1 — counterspy: does the method still reach a real defect?** A repo with a recorded prior
sweep, run without that record in context.

**Control 2 — ghola or go-facade-template: does the method reach at all on fresh ground?** A repo
with no sweep record. Run this second. Control 1 tests recall on ground someone has already walked;
only control 2 tests whether the method finds anything unaided.

You are not told what either sweep is expected to produce. Sweep normally. **Finding nothing is a
permitted and reportable outcome** — the skill says so in its own opening, and a control that
pressures you toward a finding is not measuring the method.

---

## Control 1 — counterspy

```bash
git -C ~/code/counterspy rev-parse --short HEAD     # expect 531cc42, tree clean
```

Pinned SHA `531cc42`. 862 tracked files. Go. If HEAD differs or the tree is dirty, **stop and tell
the operator** — the gate refuses a dirty map by construction and a moved SHA makes this a different
experiment.

Run `/slop-ferret` against `~/code/counterspy`. Follow it as written, with the one deviation above.

The coded gate, using your own session scratchpad as the map root (never the operator's vault):

```bash
SHA=$(git -C ~/code/counterspy rev-parse --short HEAD)
magma --depth 1 ~/code/counterspy cspy-control <scratchpad>/codemap
python3 ~/.claude/skills/slop-ferret/scripts/gate.py plan \
    <scratchpad>/codemap/cspy-control "$SHA" ~/code/counterspy --since v1.6.0 > plan.json
```

Read `plan.h_unmatched_changes` **before** `plan.h_worklist`. The worklist can only tell you what the
enumeration found; the unmatched list is the only output that can tell you it missed something.

Then sweep, write `discharge.json`, and:

```bash
python3 ~/.claude/skills/slop-ferret/scripts/gate.py verify plan.json discharge.json
# exit 0 COMPLETE · 4 PARTIAL · 3 INCOMPLETE
```

## Control 2 — fresh ground

`~/code/ghola` @ `4f33b3c` (69 tracked, Go + some Rust), or `go-facade-template`. Same procedure.
ghola has no `v1.6.0`-equivalent baseline worth diffing, so `--since` is optional there; say which
you did.

---

## What to report

The verdict block from `SKILL.md` Step 4, then `/slop-ferret:report`. Beyond the standard fields:

1. **The recorded deviation**, named as such.
2. **`Families ref: read | NOT READ`** — answer honestly; it is there to be falsifiable.
3. **Every near-miss** — candidates you refuted before filing, and what refuted each. On this
   artifact these have historically been worth more than the findings.
4. **`h_unmatched_changes`**: how many, how many you waived, and — the number that actually matters —
   **how many waivers you had to think about.**
5. **`PARTIAL` is an honest verdict and so is `INCOMPLETE`.** Do not reach for `COMPLETE`. The gate
   was repaired on 2026-08-01 specifically because it used to hand out `COMPLETE` to a sweep that had
   accounted for nothing, so a `COMPLETE` here means more than it used to and should be earned.

**File nothing.** No issues, no tracker, no write-back to the target. This is a measurement run; the
operator decides afterwards whether anything gets filed. `SUSPECTED` findings go to the operator as
leads regardless, per the skill.

## Two caveats you are inheriting

- `H_DEFER_FLOOR = 60` in `gate.py` is a judgement, not a derived number. No sweep has ever been
  completed, so there was no feasibility ceiling to fit it to. If it feels wrong in practice, that
  observation is a finding about the gate and worth reporting.
- The skill was substantially repaired on 2026-08-01 — the coverage gate, the tier split, the
  vocabulary, the report command. **You are the first run under any of it.** Anything that behaves
  oddly is more likely to be a defect in the instrument than in the target. Check the instrument
  before the code; the skill's own Step 3.5 requires it.

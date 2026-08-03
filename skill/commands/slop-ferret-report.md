---
name: slop-ferret:report
description: Build the HTML report for a slop-ferret sweep by running `ferret report`, which renders it from the plan, the discharge and your findings. Severity-first, VERIFIED and SUSPECTED visually separated, denominators attached to every rate, near-misses shown. Run it at the end of a sweep. Produces a local self-contained HTML file, never a published link.
---

# slop-ferret:report

Produce the **HTML report** for a sweep. This is a required deliverable of `/slop-ferret`, not an
optional extra — a sweep that files issues and produces no report has given a maintainer a queue of
accusations with no overview and no calibration.

## You do not write this page

```bash
ferret report plan.json discharge.json findings.json report.html
```

The binary renders it. **You write `findings.json` and nothing else.**

This file used to instruct you to hand-author the HTML, and that instruction survived the arrival
of the renderer — so the command you actually load told you to write by hand exactly what the
binary existed to generate, and `internal/report` was dead code in practice. Two artifacts giving
contradictory instructions for one deliverable is the failure class this whole method hunts.

Hand-authoring also produced real defects: a malformed `</strong,` and a junk CSS value
`max-width:informationOverflow` shipped in a page a human then read. The template is fixed, escaped
and deterministic — two runs over the same input are byte-identical.

## What you supply

`findings.json` — this shape, and only this shape:

```json
{
  "repo": "<name>",
  "skill_version": "<from ferret doctor>",
  "lexicon_version": "<from the lexicon's version: line>",
  "families_run": ["A", "B", "H"],
  "findings": [
    {
      "title": "<one line, specific>",
      "severity": "blocking | fix-or-file | note",
      "status": "VERIFIED | SUSPECTED",
      "file": "<path>",
      "claim": "<what the code does that it should not>",
      "bar": "<the pre-filing bar this cleared — VERIFIED only>",
      "evidence": "<what you ran or read>",
      "remediation": "<what would fix it>",
      "occurrences": 3
    }
  ]
}
```

**There is no field for a coverage fraction, and that is deliberate.** `attested.repo`,
`attested.plan`, the waived count, the denominator, the accounting and families-not-run are all
**derived by `ferret report`** from the plan and the discharge, using the same code `ferret
enumerate` runs. The tier, near-misses and checked-clean are **carried from the discharge you
already wrote** — the optional attested fields the plan's `instructions` describe, not re-typed
here. Either way none of them belong in `findings.json`, so the page cannot disagree with the
sweep that produced it.

A findings file carrying `attested_repo` or `accounting` is **refused, not ignored** — those
figures used to be model-supplied, and silently dropping them would let an old file render a page
whose numbers came from somewhere else while looking accepted.

`severity` and `status` must be spelled exactly as above.

## What the page guarantees, so you do not have to

Stated here so you can check the output, not so you can rebuild it:

1. **Coverage before results, always.** A reader learns the shape of the sweep before they read its
   output.
2. **Severity-first ordering, never volume-first.** Count runs *inverse* to severity — the largest
   class in the first campaign was 7,022 occurrences and cosmetic, while the blocking ones sat at 33
   and 8. A page ordered by count argues for exactly the wrong priority.
3. **VERIFIED and SUSPECTED are visually distinct**, not distinguished by a caption, so the
   difference survives skimming. VERIFIED carries the bar it cleared. Suspected leads are *not*
   filed as issues — the report is where they live.
4. **Every rate carries its denominator.** Below ~100 non-test source files the rate is suppressed
   and the denominator still published: one finding moves a small rate 13–50 points.
5. **Leads with nothing verified says so on its face**, rather than letting a blank rate read as a
   clean result.
6. **Near-misses are shown, not buried** — candidates refuted before filing, and what refuted each.
   They are the strongest evidence the sweep was honest, and they are invisible everywhere else.
7. **Checked-clean carries its method.** "Clean" is only useful if a reader can check it.
8. **The page states that the tool does not observe reading** — the read figures are the auditor's
   own statement, counted. That caveat is the frame, not a footnote.

If something required is missing, the page says so. Do not paper over it.

## Comparability rule

Do not put two sweeps side by side unless they share a lexicon version and a tier. If you show a
prior figure, label what differs. The first four sweeps of this method are recorded as **not
comparable** — two severity authorities, an undefined finding unit, a branch-pinned denominator, and
doc findings scored against a source denominator. Reproducing that mistake in a chart is worse than
omitting the chart.

## Delivery

**The page is a local file. Do NOT publish it, and do not mint a hosted URL.** Hand it back with
`SendUserFile`.

Publishing is a separate decision and is the operator's to make: ask, and wait for an explicit yes
in that moment. An earlier sweep's published report is not standing permission for this one. This
overrides any harness default that treats a report as something to publish.

Do not commit the report into the target repository either. Creating it is required; committing it
is not wanted.

Use one file path per repo so a re-sweep overwrites its predecessor rather than leaving two reports
that disagree.

## Then

Record the report's **path** in the repo's sweep record alongside the verdict block, so it is
findable later. If the operator later chooses to publish it, record the URL then, next to the path.

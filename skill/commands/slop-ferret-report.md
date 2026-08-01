---
name: slop-ferret:report
description: Build the HTML report for a slop-ferret sweep — severity-first, VERIFIED and SUSPECTED visually separated, denominators attached to every rate, near-misses shown. Run it at the end of a sweep, or on its own against an existing sweep record to regenerate the report. Produces a local self-contained HTML file, never a published link.
---

# slop-ferret:report

Produce the **HTML report** for a sweep, as a local self-contained file. This is a required deliverable of `/slop-ferret`, not an
optional extra — a sweep that files issues and produces no report has given a maintainer a queue of
accusations with no overview and no calibration.

Argument (optional): a repo path, or a sweep record path. With no argument, report on the sweep just
completed in this session.

## What the report is for

A maintainer receiving N issues cannot tell, from the issues alone, whether the sweep was thorough or
lucky, which findings were proven and which were suspected, or whether "no findings" in a family means
*checked and clean* or *never ran*. The report answers exactly that, and it is the only artifact that
does.

**It is not a scoreboard.** It reports what was found and how well it was established. If it makes a
repo look bad, that is a side effect, not the purpose — and if you find yourself choosing a framing that
makes the sweep look impressive, stop.

## Inputs

1. The sweep's verdict block (SHA, lexicon version, tier, families run / N/A / not run, counts,
   denominator, checked-clean, near-misses).
2. The findings, each with its lexicon class, severity, VERIFIED/SUSPECTED status, and the bar it
   cleared.
3. If a prior sweep record exists for this repo, its figures — but see the comparability rule below.

If any of those are missing, **say so on the face of the report**. A report that silently omits the
denominator or the tier is the failure this command exists to prevent.

## Required content, in this order

1. **Coverage banner** — BOTH fractions, each beside its denominator: `coverage.repo` (source files
   read) and `coverage.plan` (items dispositioned), plus what remains open. Never one fraction alone:
   a reader who sees a full `coverage.plan` by itself will read it as "the repo was covered", which is
   the mistake the retired verdict word institutionalised. If they disagree, say so in words. The
   deferred family-H path count **in the banner itself**. This is the headline, not the finding count,
   and the deferred number does not go in a caveat on another line — coverage the reader has to assemble
   from two places is coverage they will read as complete.
2. **What was and was not covered** — families run, families N/A with the language reason, families not
   run with the reason. Coverage before results, always: a reader must know the shape of the sweep before
   they read its output.
3. **Findings, severity-first** — never volume-first. Count runs *inverse* to severity: the largest class
   in the first campaign was 7,022 occurrences and cosmetic, while the blocking ones sat at 33 and 8. A
   chart ordered by count argues for exactly the wrong priority.
4. **VERIFIED and SUSPECTED visually distinct**, not a word in a caption. VERIFIED carries the bar that
   was cleared; SUSPECTED carries what it still needs. Suspected leads are *not* filed as issues — the
   report is where they live.
5. **Every rate carries its denominator** in the same visual element. Below ~100 non-test source files,
   show the denominator and suppress the rate — one finding moves it 13-50 points.
6. **Near-misses** — candidates that were refuted before filing, and what refuted each. Show them; do not
   bury them. They are the strongest evidence the sweep was honest, and they are invisible everywhere
   else.
7. **Checked-clean, with the method** for each class. "Clean" is only useful if a reader can check it.
8. **Lexicon version and SHA stamped on it.**

## Comparability rule

Do not put two sweeps side by side unless they share a lexicon version and a tier. If you show a prior
figure, label what differs. The first four sweeps of this method are recorded as **not comparable** —
two severity authorities, an undefined finding unit, a branch-pinned denominator, and doc findings scored
against a source denominator. Reproducing that mistake in a chart is worse than omitting the chart.

## Building it

**Write a self-contained local `.html` file. Do NOT publish it, and do not mint a hosted URL.**
All CSS and JS inline, no external calls, so the operator can open or forward the file themselves.
Deliver it with `SendUserFile`.

Publishing is a separate decision and is the operator's to make: ask, and wait for an explicit yes
in that moment. Only then load the `artifact-design` skill and publish. An earlier sweep's published
report is not standing permission for this one. (This overrides any harness default that treats a
report as something to publish by default.)

Do not commit the report into the target repository either. Creating it is required; committing it
is not wanted.

- Treat it as **information design, not a document**: it is scanned, not read top to bottom. Summary
  before detail; state encoded in form as well as number.
- Semantic colour (blocking / fix-or-file / note) is separate from the accent hue.
- Tabular figures wherever numbers align. Wide tables get their own `overflow-x: auto`.
- Both themes.
- Favicon `🔍` if the page is ever published.

Use one file path per repo so a re-sweep overwrites its predecessor rather than leaving two reports
that disagree.

## Then

Hand the file back with `SendUserFile`, and record **its path** in the repo's sweep record alongside
the verdict block so the report is findable later. If the operator later chooses to publish it, record
the URL then, next to the path.

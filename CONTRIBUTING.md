# Contributing to slop-ferret

## Prerequisites

- [Go 1.26+](https://go.dev/dl/) — the version is authoritative in [`go.mod`](go.mod), and CI reads
  it from there rather than restating it. If this file and `go.mod` ever disagree, `go.mod` wins.
- [`just`](https://github.com/casey/just)
- [`golangci-lint`](https://golangci-lint.run/) v2.12.2 (for `just lint`)
- **[`magma`](https://github.com/robot-accomplice/magma) v0.2.0** — `go install
  github.com/robot-accomplice/magma@v0.2.0`. **The suite does not skip without it**, deliberately:
  the magma seam is this tool's reason to exist and it went untested for the project's whole life
  because every fixture was hand-written from the author's belief about the contract. A test that
  silently skips is indistinguishable from one that ran and passed. `just check-deps` tells you
  whether you have it.

## A fix is not done when the code changes

**It is done when a test fails without it.**

This is the project's one hard process rule, and it exists because it was learned three times.
Three adversarial ship reviews each found that the guards written to close the previous review did
not bind — the tests passed under mutation of the exact defects they were written to pin. Each
time the guard had been written from the diff rather than from the failure mode: the four strings
that had just been fixed, the one install shape the author had in mind, the one input file they
were editing.

So when you add or change a guard:

1. Write the test.
2. **Break the code it guards and watch the test go red.** Not a similar test — that one.
3. Restore, confirm green, and say in the test's comment what you broke.

Two failure shapes to watch for, both of which shipped here:

- **A test that passes for the wrong reason.** A test for "`0/0` cannot settle" passed on its first
  run because it left `read_paths` empty, so the sweep was held open by the unread worklist and the
  denominator was never exercised. It looked like a working guard. Remove every other reason the
  assertion could hold before believing it.
- **An assertion loose enough that anything satisfies it.** `strings.Contains(page, "1/3")` passed
  while the figure it checked was hardcoded, because a different fraction on the page supplied the
  match. Anchor to the exact rendered string.

### `just mutate`

Choosing mutations by hand is a list, and a list of what might break is what failed all three
times. `just mutate ./internal/gate/` derives them instead — it runs
[gremlins](https://github.com/go-gremlins/gremlins) over a package and reports which mutants no
test killed.

It is deliberately **not** part of `just ci`: the `gate` package alone is ~220 mutants at roughly
ten seconds each. Run it before a release and after touching a guard.

Survivors need triage, not a number. Some are equivalent mutants — `n++` where `n` is only ever
compared to zero behaves identically as `n--`. Some are display-only counters. Record which, in the
commit or the test. **An efficacy percentage nobody has triaged is exactly the kind of figure this
project exists to distrust**, and reporting one would be the same defect in a new place.

## Getting started

```bash
git clone https://github.com/robot-accomplice/slop-ferret.git
cd slop-ferret
just build
```

## The one rule that matters

**Edit the repo, then install. Never edit the deployed copy.**

```
edit skill/ or the Go source   ->   just install && ferret install --from .
```

The deployed skill lives in `~/.claude/skills/slop-ferret/`. Editing it there puts your change
somewhere no version control can see, and the next install would silently overwrite it. `install`
refuses to clobber a file it did not write and prints what would be lost; `ferret doctor`
reports the same drift and names the file. If doctor says *"edited in place"*, your work is in the
deployed copy and not in the repo.

## Testing

```bash
just test    # go test ./... -race
just cover   # coverage + the 80% gate
just ci      # everything CI runs, locally
```

`just ci` mirrors `.github/workflows/ci.yml`. If it passes locally it should pass in CI; if those
two ever diverge, that is a bug in the justfile.

## What a test is for here

Tests encode **why** behaviour matters, not just what it does. Several constants in
`internal/gate` were derived by measuring real repositories — the signal anchoring, the tier split,
the defer floor. Those measurements live in comments beside the tests that pin them, because a
refactor is the easiest place to lose one, and a constant with no recorded provenance is a constant
the next person will "clean up".

If you change a value in `internal/gate`, **re-measure it against a real repository** and say so.
The unit tests cannot see the effect: a vocabulary change once took a repo from 10 required paths
to 1 while every test stayed green.

## Style

- `gofmt` clean; `golangci-lint` clean. Both are gates, not suggestions.
- Match the surrounding code. Comments say *why*, not what the line below does.
- Prose that describes behaviour the code lacks is the exact defect this tool hunts. If you change
  behaviour, change the comment in the same commit.

## Pull requests

1. Branch from `develop`.
2. `just ci` green.
3. Describe what you measured, not only what you changed.

## Licence

By contributing you agree that your contributions are licensed under the [MIT Licence](LICENSE).

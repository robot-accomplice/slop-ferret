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

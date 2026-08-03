# slop-ferret developer tasks. Run `just` to list recipes.
# `just ci` mirrors .github/workflows/ci.yml so you can validate locally before pushing.
#
# It checks its own prerequisites first. The integration tests run REAL magma and fail rather than
# skip when it is absent, so without `check-deps` a contributor's first `just ci` is a wall of
# failures that look like their change broke something.

min_coverage := "80"

# Show available recipes
_default:
    @just --list

# Compile everything
build:
    go build ./...

# Install the ferret binary into $GOBIN (~/go/bin by default)
install:
    go install ./cmd/ferret

# Run from source, e.g. `just run doctor`
run *ARGS:
    go run ./cmd/ferret {{ ARGS }}

# Format the tree in place
fmt:
    gofmt -w .

# Fail if anything is not gofmt-clean (CI gate)
fmt-check:
    #!/usr/bin/env bash
    set -euo pipefail
    unformatted="$(gofmt -l .)"
    if [ -n "$unformatted" ]; then
        echo "not gofmt-clean:"; echo "$unformatted"; exit 1
    fi
    echo "gofmt clean"

# go vet
vet:
    go vet ./...

# golangci-lint (uses .golangci.yml)
lint:
    golangci-lint run ./...

# Run tests with the race detector
test:
    go test ./... -race

# Test with coverage and enforce the {{ min_coverage }}% gate
cover:
    #!/usr/bin/env bash
    set -euo pipefail
    go test ./... -covermode=atomic -coverprofile=coverage.out
    pct="$(go tool cover -func=coverage.out | awk '/^total:/ {print $3}' | tr -d '%')"
    echo "total coverage: ${pct}%  (minimum: {{ min_coverage }}%)"
    awk "BEGIN { exit !(${pct} >= {{ min_coverage }}) }" || {
        echo "coverage ${pct}% is below the {{ min_coverage }}% gate"; exit 1; }

# Open the coverage report in a browser
cover-html: cover
    go tool cover -html=coverage.out

# Verify the deployed skill matches this checkout (catches editing the wrong copy)
doctor:
    go run ./cmd/ferret doctor

# Full local CI validation — same gates as GitHub Actions
# MUTATION TESTING. Not in `ci` -- the gate package alone is ~220 mutants at ~10s each, so a full
# run is tens of minutes. Run it before a release, and after any change to a guard.
#
# WHY IT EXISTS. Three review rounds found that guards written to close the previous round did not
# bind: the tests passed under mutation of the exact defects they were written to pin. Each time the
# author had "verified by mutation" -- by choosing ONE mutation per fix, which is a list, and a
# hand-written list of what might break is the thing that failed all three times. An independent
# audit ran 62 mutations against the same tree and 21 survived.
#
# So the mutation set is DERIVED rather than chosen, for the same reason the stale-prose gate parses
# the dispatch and the magma field set comes from reflection.
#
# Triage the survivors: some are equivalent mutants (a counter used only in an `== 0` test) and some
# are display-only. Record which, rather than chasing the number -- an efficacy figure nobody has
# triaged is exactly the kind of number this project exists to distrust.

# Mutation-test a package: break the code and check a test notices. Slow; run before a release.
mutate PKG='./...':
    #!/usr/bin/env bash
    set -euo pipefail
    command -v gremlins >/dev/null || {
      echo "gremlins is not on PATH: go install github.com/go-gremlins/gremlins/cmd/gremlins@latest" >&2
      exit 1
    }
    # The default timeout is derived from a baseline run and is too tight here: the gate package
    # shells out to git and magma, so an honest mutant looks like a hang.
    gremlins unleash --timeout-coefficient 20 {{PKG}}

# Fail early and by name, rather than as an unexplained test failure three minutes in.
check-deps:
    #!/usr/bin/env bash
    set -euo pipefail
    if ! command -v magma >/dev/null; then
      echo "magma is not on PATH." >&2
      echo "  The magma seam is the one this tool exists for, and its tests fail rather than skip:" >&2
      echo "  a test that silently skips is indistinguishable from one that ran and passed." >&2
      echo "  go install github.com/robot-accomplice/magma@v0.2.0" >&2
      exit 1
    fi
    echo "magma: $(magma --version 2>&1 | head -1)"

ci: check-deps fmt-check vet lint build cover
    @echo "✓ local CI validation passed"

# Tidy module dependencies
tidy:
    go mod tidy

# Cross-compile the release archives into dist/ WITHOUT publishing (dry run of release.yml)
release-dry version="v0.0.0-dev":
    #!/usr/bin/env bash
    set -euo pipefail
    rm -rf dist && mkdir -p dist
    for t in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do
        os="${t%/*}"; arch="${t#*/}"
        bin="ferret"; [ "$os" = "windows" ] && bin="ferret.exe"
        echo "==> ${os}/${arch}"
        CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags "-s -w" -o "dist/${bin}" ./cmd/ferret
        base="slop-ferret_{{ version }}_${os}_${arch}"
        if [ "$os" = "windows" ]; then (cd dist && zip -q "${base}.zip" "$bin" && rm "$bin")
        else (cd dist && tar -czf "${base}.tar.gz" "$bin" && rm "$bin"); fi
    done
    (cd dist && shasum -a 256 ./* > checksums.txt)
    ls -la dist

# Remove build and coverage artifacts
clean:
    rm -rf dist coverage.out

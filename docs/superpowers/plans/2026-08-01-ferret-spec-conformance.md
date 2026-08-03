# ferret Spec Conformance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring the built code into conformance with `docs/superpowers/specs/2026-08-01-slop-ferret-design.md` — rename the binary to `ferret`, remove embedded prose, merge `install`/`update`, default to fetching from the repository, add the records store, and split the overloaded exit code.

**Architecture:** The `Source` abstraction in `internal/install` already decouples "where prose comes from" from "how it is deployed", so removing the embed is a deletion plus a new default, not a redesign. `internal/gate` gains a records writer that consumes the existing `Plan`/`Discharge`/`Result` types. `main.go` moves to `cmd/ferret/` unchanged in behaviour.

**Tech Stack:** Go 1.26, stdlib only (`archive/tar`, `compress/gzip`, `net/http`, `os/exec` for git). Test with the stdlib `testing` package plus `testing/fstest` and `net/http/httptest`. Build tasks with `just`.

## Global Constraints

- **Go 1.26** — authoritative in `go.mod`; never restate the version elsewhere.
- **No dependencies outside the standard library.** The module has none today; keep it that way.
- **Coverage ≥80%**, enforced by `just cover` and CI. Every task must leave `just ci` green.
- **`gofmt` clean and `golangci-lint` clean** — both are gates, not suggestions.
- **Binary is `ferret`; project/repo is `slop-ferret`; slash commands stay `/slop-ferret` and `/slop-ferret:report`.** (D2, D9)
- **Skill version is date-style `YYYY-MM-DD.N`, never semver.** (D5)
- **No skill or lexicon prose compiled into the binary.** (D3)
- **Never write into a target repository being swept.** Reads are `git ls-files` and `git diff --name-only` only.
- **Comments say why, not what.** Prose describing behaviour the code lacks is the defect this tool hunts; change behaviour and its comment in the same commit.
- **Measured constants in `internal/gate`** (signal vocabulary, `hTier1`/`hTier2`, `hDeferFloor`, `anchor`) must not be altered by this plan. If a task appears to require changing one, stop and escalate.

---

## File Structure

| File | Responsibility | Status |
|---|---|---|
| `cmd/ferret/main.go` | CLI dispatch only — `run(argv, stdout, stderr) int` | moved from `main.go` |
| `cmd/ferret/main_test.go` | dispatch, exit codes, version format | moved from `main_test.go` |
| `internal/install/source.go` | `Source`, `Fetch`, `DirSource`, `SkillVersion` | modify (drop embed helper, add default-ref resolution) |
| `internal/install/install.go` | `Install`, `Doctor`, manifest, command entries | modify (drop `Embedded`, `EmbeddedSource`) |
| `internal/gate/gate.go` | `BuildPlan`, constants, denominator | modify (exit-code constant only) |
| `internal/gate/verify.go` | `Verify` | modify (record hook) |
| `internal/gate/record.go` | **new** — record shape, repo key, sha resolution, write/list | create |
| `internal/gate/record_test.go` | **new** — record round-trip, key derivation, sha gate | create |
| `skill/` | prose assets, no longer embedded | unchanged |
| `.github/workflows/ci.yml` | replace the embedded-skill job with `install --from .` | modify |
| `.github/workflows/release.yml` | read skill stamp from `skill/VERSION`; publish skill artifact | modify |
| `justfile`, `README.md`, `CHANGELOG.md` | command names, install paths | modify |

---

## Task 1: Split the overloaded exit code

Spec §7. Exit `3` currently means both "items still open" and "the tool refused". A script cannot tell an unfinished sweep from a map of the wrong tree, and those want opposite responses.

**Files:**
- Modify: `internal/gate/gate.go` (the `die` helper and its call sites)
- Modify: `internal/gate/verify.go` (`Verify` return)
- Test: `internal/gate/gate_test.go`, `cmd/ferret/main_test.go` *(still `main_test.go` at this point — Task 2 moves it)*

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: exported constants `gate.ExitOK = 0`, `gate.ExitMisuse = 2`, `gate.ExitItemsOpen = 3`, `gate.ExitRefused = 4`. `gate.Err.Code` continues to carry an int; refusals now carry `ExitRefused`.

- [ ] **Step 1: Write the failing test**

Add to `internal/gate/gate_test.go`:

```go
// A refusal and an unfinished sweep want opposite responses from a script: one means "read the
// work queue", the other means "nothing was measured, your input is wrong". Sharing exit 3 made
// them indistinguishable.
func TestARefusalAndAnUnfinishedSweepUseDifferentExitCodes(t *testing.T) {
	if ExitRefused == ExitItemsOpen {
		t.Fatal("a refusal must not share an exit code with an unfinished sweep")
	}
	m := writeMap(t, "OLDSHA", "codemap-rows/1", "rta", true, nil)
	_, err := BuildPlan(m, "NEWSHA", gitRepo(t, map[string]string{"a.go": "package a\n"}), "")
	if err == nil {
		t.Fatal("want a refusal")
	}
	if got := code(err); got != ExitRefused {
		t.Fatalf("refusal exit = %d, want ExitRefused (%d)", got, ExitRefused)
	}

	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n"})
	pl := planFor(t, repo)
	_, c, err := Verify(writeJSON(t, pl), writeJSON(t, map[string]any{"sha": "abc123"}))
	if err != nil {
		t.Fatal(err)
	}
	if c != ExitItemsOpen {
		t.Fatalf("unfinished sweep exit = %d, want ExitItemsOpen (%d)", c, ExitItemsOpen)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gate/ -run TestARefusalAndAnUnfinishedSweep -v`
Expected: FAIL — `undefined: ExitRefused`.

- [ ] **Step 3: Write minimal implementation**

In `internal/gate/gate.go`, add above `type Err struct`:

```go
// Exit codes. These are a CONTRACT with whatever script wraps this tool, so they are named rather
// than spelled inline. 3 previously meant both "items still open" and "the tool refused"; a caller
// could not tell an unfinished sweep from a map of the wrong tree, and those want opposite
// responses. 4 was free — it was the retired PARTIAL verdict's code.
const (
	ExitOK        = 0 // nothing raised is undispositioned
	ExitMisuse    = 2 // wrong arity, unreadable file
	ExitItemsOpen = 3 // the sweep is not finished; read `remaining`
	ExitRefused   = 4 // the tool declined to run: wrong tree, unknown contract, missing map
)
```

Then change every refusal to carry `ExitRefused`. In `gate.go`, `loadMap` uses `die(3, …)` in five places (map dir missing, required file missing, bad JSON, unsupported contract, sha mismatch) — change each `die(3,` to `die(ExitRefused,`. Leave `die(2, …)` in `gitLines` as `die(ExitMisuse, …)`.

In `internal/gate/verify.go`, change the three `return nil, 2, die(2, …)` to `return nil, ExitMisuse, die(ExitMisuse, …)`, and the final return:

```go
	res.Status = "settled"
	code := ExitOK
	if len(remaining) > 0 {
		res.Status = "open"
		code = ExitItemsOpen
	}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./... -race`
Expected: PASS. `cmd` tests asserting `code != 3` for a missing map dir will now see `4` — update `TestPlanSurfacesTheGatesExitCode` in `main_test.go` to expect `gate.ExitRefused`, importing `"github.com/robot-accomplice/slop-ferret/internal/gate"`.

- [ ] **Step 5: Update the spec's delta table**

In `docs/superpowers/specs/2026-08-01-slop-ferret-design.md` §10, change the exit-code row to **built**, and remove the "proposed, not yet agreed" bullet from §11.

- [ ] **Step 6: Commit**

```bash
just ci
git add internal/gate main_test.go docs/superpowers/specs
git commit -m "Split the overloaded exit code: 3 is items open, 4 is a refusal

A caller could not tell an unfinished sweep from a map of the wrong tree.
Those want opposite responses from a script. 4 was free, being the retired
PARTIAL code."
```

---

## Task 2: Move the binary to `cmd/ferret`

Spec D2. `go install` derives the binary name from the package directory, so `ferret` requires `cmd/ferret/`.

**Files:**
- Create: `cmd/ferret/main.go` (moved), `cmd/ferret/main_test.go` (moved)
- Delete: `main.go`, `main_test.go`
- Modify: `justfile`, `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `README.md`

**Interfaces:**
- Consumes: `gate.ExitRefused` etc. from Task 1.
- Produces: binary named `ferret`; `go install github.com/robot-accomplice/slop-ferret/cmd/ferret@<ref>` is the install path.

- [ ] **Step 1: Move the files with git so history follows**

```bash
mkdir -p cmd/ferret
git mv main.go cmd/ferret/main.go
git mv main_test.go cmd/ferret/main_test.go
```

- [ ] **Step 2: Fix the embed path**

`//go:embed all:skill` resolves relative to the package directory, which is now `cmd/ferret/`. It will fail to build. **Do not add a copy of `skill/` under `cmd/ferret/`** — Task 3 deletes the embed entirely. For this task only, keep it building by temporarily pointing the tests at a synthetic tree: delete the `//go:embed` line and the `skillFS` var, and replace the `install.Embedded = skillFS` line in `main()` with nothing. Add to `cmd/ferret/main_test.go`'s `init()`:

```go
func init() {
	// Task 3 replaces this with the real source model. Until then the CLI tests need *a* source.
	install.Embedded = fstest.MapFS{
		"skill/SKILL.md": {Data: []byte("# skill\n")},
		"skill/VERSION":  {Data: []byte(`{"version":"2026-08-01.8"}`)},
	}
}
```

importing `"testing/fstest"`.

- [ ] **Step 3: Run the build and tests**

Run: `go build ./... && go test ./... -race`
Expected: PASS. Tests asserting the embedded skill carries the lexicon (`TestTheEmbeddedSkillCarriesWhatTheMethodNeeds`) will fail against the synthetic tree — delete that test; Task 3 replaces it with a real source test.

- [ ] **Step 4: Update every reference to the binary path**

In `justfile`, change `build:` to `go build ./...` (unchanged), `install:` to `go install ./cmd/ferret`, `run *ARGS:` to `go run ./cmd/ferret {{ ARGS }}`, `doctor:` to `go run ./cmd/ferret doctor`, and in `release-dry` change `go build … -o "dist/${bin}" .` to `… ./cmd/ferret` and `bin="slop-ferret"` to `bin="ferret"`.

In `.github/workflows/ci.yml`, change `go run . install` / `go run . doctor` to `go run ./cmd/ferret install` / `go run ./cmd/ferret doctor`.

In `.github/workflows/release.yml`, change both `go run . --version` to `go run ./cmd/ferret --version`, and in the cross-compile loop `bin="slop-ferret"` → `bin="ferret"`, `go build … .` → `go build … ./cmd/ferret`.

In `README.md`, change every `slop-ferret <subcommand>` to `ferret <subcommand>` and the install line to `go install github.com/robot-accomplice/slop-ferret/cmd/ferret@v0.1.0`.

- [ ] **Step 5: Verify the binary name**

```bash
just release-dry v0.0.0-dev
tar -tzf dist/ferret_v0.0.0-dev_darwin_arm64.tar.gz
```
Expected: the archive contains `ferret`.

- [ ] **Step 6: Commit**

```bash
just ci
git add -A
git commit -m "Move the binary to cmd/ferret

go install derives the binary name from the package directory. Repo stays
slop-ferret; the command is ferret."
```

---

## Task 3: Remove the embed; the repository becomes the default source

Spec D3, D8, §5. The binary carries no prose. `install` acquires it — from the repo by default, a ref, or a checkout.

**Files:**
- Modify: `internal/install/install.go` (delete `Embedded`, `EmbeddedSource`)
- Modify: `internal/install/source.go` (add `DefaultSource`)
- Modify: `cmd/ferret/main.go` (dispatch, `--version`)
- Modify: `internal/install/install_test.go`, `internal/install/source_test.go`, `cmd/ferret/main_test.go`
- Modify: `.github/workflows/ci.yml`, `.github/workflows/release.yml`

**Interfaces:**
- Consumes: `Source`, `Fetch(ref string) (Source, func(), error)`, `DirSource(dir string) (Source, error)` — all existing.
- Produces: `install.DefaultSource(binVersion string) (Source, func(), error)` — fetches at `v<binVersion>`. `install.Install` and `install.Doctor` keep their signatures. `install.Embedded` and `install.EmbeddedSource` are **removed**.

- [ ] **Step 1: Write the failing test**

Add to `internal/install/source_test.go`:

```go
// D3: the binary carries no prose, so the default source resolves the binary's own version to a
// ref. A 0.3.0 binary installs the v0.3.0 skill -- the prose that version was tested with --
// without the user needing to know a ref exists.
func TestDefaultSourceResolvesTheBinarysOwnVersion(t *testing.T) {
	var asked string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/commits/") {
			asked = r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			_, _ = w.Write([]byte("aaaaaaaaaaaaaaaa"))
			return
		}
		_, _ = w.Write(tarball(t, "aaaaaaaaaaaaaaaa", map[string]string{
			"skill/SKILL.md": "# s\n", "skill/VERSION": `{"version":"2026-08-01.8"}`}))
	}))
	defer srv.Close()
	oldT, oldR := tarballURL, refAPIURL
	tarballURL, refAPIURL = srv.URL+"/tar.gz/", srv.URL+"/commits/"
	defer func() { tarballURL, refAPIURL = oldT, oldR }()

	src, cleanup, err := DefaultSource("0.3.0")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if asked != "v0.3.0" {
		t.Errorf("resolved ref = %q, want v0.3.0", asked)
	}
	if _, err := fs.ReadFile(src.FS, "skill/SKILL.md"); err != nil {
		t.Errorf("skill not fetched: %v", err)
	}
}

// Before the first release the default has nothing to resolve. It must say so and name the two
// working alternatives, not fall back to HEAD -- a silent fallback is the unpinned install the
// default exists to avoid.
func TestDefaultSourceFailsHelpfullyWhenTheTagDoesNotExist(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	oldR := refAPIURL
	refAPIURL = srv.URL + "/commits/"
	defer func() { refAPIURL = oldR }()

	_, _, err := DefaultSource("0.1.0")
	if err == nil {
		t.Fatal("want an error")
	}
	for _, want := range []string{"--from", "--ref"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error must name %s: %v", want, err)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/install/ -run TestDefaultSource -v`
Expected: FAIL — `undefined: DefaultSource`.

- [ ] **Step 3: Write minimal implementation**

In `internal/install/source.go`:

```go
// DefaultSource is what a bare `ferret install` uses: the repository at the ref matching this
// binary's own version. The install is self-pinning by construction — a 0.3.0 binary gets the
// v0.3.0 prose — without the user having to know a ref exists.
//
// Release artifacts are a SUPPORTED source, not a required one (D8): the default does not depend
// on any artifact having been published, only on the tag existing.
func DefaultSource(binVersion string) (Source, func(), error) {
	ref := "v" + binVersion
	src, cleanup, err := Fetch(ref)
	if err != nil {
		return Source{}, func() {}, fmt.Errorf("%w\n  no skill found at %s. Before the first "+
			"release there is no tag to resolve — use `--from <checkout>` for a local tree, or "+
			"`--ref main` to track the branch", err, ref)
	}
	return src, cleanup, nil
}
```

In `internal/install/install.go`, delete the `Embedded` var and the `EmbeddedSource` function, and the `io/fs` import if it becomes unused (it will not — `srcFiles` uses it).

In `cmd/ferret/main.go`: delete the `embed` import, the `//go:embed` directive and `skillFS` (if Task 2 left them), and rewrite dispatch:

```go
	case "install", "update": // D4: synonyms
		return cmdInstall(args, stdout, stderr)
	case "doctor":
		src, cleanup, err := sourceFor(args)
		if err != nil {
			// doctor must still work with no source: it reports the DEPLOYED copy, and "I cannot
			// reach the network" is not a reason to refuse to describe what is on disk.
			return install.Doctor(stdout, install.Source{}, binVersion)
		}
		defer cleanup()
		return install.Doctor(stdout, src, binVersion)
	case "version", "--version", "-v":
		fmt.Fprintf(stdout, "ferret %s\n", binVersion)
		return ExitOK
```

and a shared source resolver:

```go
// sourceFor turns the flags into a Source. Order matters: an explicit --from or --ref beats the
// default, and the default is the repository at this binary's own version (D8).
func sourceFor(args []string) (install.Source, func(), error) {
	if _, dir := flagValue(args, "--from"); dir != "" {
		s, err := install.DirSource(dir)
		return s, func() {}, err
	}
	if _, ref := flagValue(args, "--ref"); ref != "" {
		return install.Fetch(ref)
	}
	return install.DefaultSource(binVersion)
}
```

`cmdInstall` becomes:

```go
func cmdInstall(args []string, stdout, stderr io.Writer) int {
	src, cleanup, err := sourceFor(args)
	if err != nil {
		fmt.Fprintf(stderr, "ferret: %v\n", err)
		fmt.Fprintln(stderr, "  the installed skill is untouched")
		return ExitMisuse
	}
	defer cleanup()
	return install.Install(stdout, src, has(args, "--force"))
}
```

Delete `cmdUpdate`. Add `const ExitOK = 0` / `ExitMisuse = 2` locally, or import them from `gate` — prefer importing to avoid two spellings of one contract.

- [ ] **Step 4: Update the affected tests**

In `cmd/ferret/main_test.go`: delete the `init()` that set `install.Embedded`. `TestInstallThenDoctorRoundTrips` and `TestDoctorReportsNotInstalledOnACleanHome` must now pass `--from .` with a `t.TempDir()` containing a synthetic `skill/`; write a helper:

```go
func fakeCheckout(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for rel, body := range map[string]string{
		"skill/SKILL.md":                       "# skill\n",
		"skill/VERSION":                        `{"version":"2026-08-01.8"}`,
		"skill/commands/slop-ferret-report.md": "# report\n",
		"skill/references/ai-slop-lexicon.md":  "# lexicon\n",
	} {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
```

Update `TestVersionIsParseableByTheReleaseGate` — `--version` no longer prints a skill stamp, so assert only `ferret <binVersion>` and drop the field-6 assertion.

In `internal/install/install_test.go`, change `setup(t)` to return a `Source` built from `fakeSkill()` (it already does).

- [ ] **Step 5: Update CI and release to match**

`.github/workflows/ci.yml` — replace the `skill` job's body:

```yaml
      - name: install from the checkout and doctor it
        run: |
          export HOME="$(mktemp -d)"
          go run ./cmd/ferret install --from .
          go run ./cmd/ferret doctor
```

`.github/workflows/release.yml` — the stamp check can no longer read `--version`. Replace `stamp="$(go run ./cmd/ferret --version | awk '{print $6}')"` with:

```bash
          stamp="$(jq -r .version skill/VERSION)"
```

Also add the skill artifact to the cross-compile step, after the binary loop:

```bash
          tar -czf "dist/slop-ferret-skill_${TAG}.tar.gz" skill
```

- [ ] **Step 6: Run everything**

Run: `just ci`
Expected: PASS, coverage ≥80%.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "Remove the embed; the repository is the default skill source

The binary carries no prose (D3), which makes the two cadences structural
rather than conventional. A bare install fetches the repo at the ref matching
the binary's own version, so it is self-pinning without the user knowing a
ref exists. install and update are synonyms (D4)."
```

---

## Task 4: Records store

Spec §6, D7. The method says read the prior record before starting and write one at the end; neither works, and `SKILL.md` already references `~/.slop-ferret/records/` — prose ahead of code.

**Files:**
- Create: `internal/gate/record.go`, `internal/gate/record_test.go`
- Modify: `internal/gate/verify.go` (accept attested fields from the discharge)
- Modify: `cmd/ferret/main.go` (`--no-record`, `records` subcommand)

**Interfaces:**
- Consumes: `Plan`, `Discharge`, `Result` from `internal/gate`; `Verify(planPath, dischargePath string) (*Result, int, error)`.
- Produces:
  - `type Record struct` with JSON tags — fields listed in Step 3.
  - `func RepoKey(repo string) (string, error)` — normalised origin URL, or `path-<8hex>` fallback.
  - `func WriteRecord(repo string, pl *Plan, dis *Discharge, res *Result) (string, error)` — returns the written path; refuses if the sha does not resolve.
  - `func ListRecords(repo string) ([]Record, error)` — newest first.
  - `Discharge` gains optional attested fields: `Tier string`, `CheckedClean []CheckedClean`, `NearMisses []string`, `FindingsVerified int`, `FindingsSuspected int`, `ReportPath string`; `type CheckedClean struct{ Class, Method string }`.

- [ ] **Step 1: Write the failing test**

Create `internal/gate/record_test.go`:

```go
package gate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A record must carry the sha it swept, and that sha must RESOLVE. Two prior sweeps recorded
// boundaries that no longer exist as objects -- both taken from dirty maps -- which made their
// denominators unreproducible and left the next sweep unable to scope itself.
func TestWriteRecordRefusesAShaThatDoesNotResolve(t *testing.T) {
	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n"})
	t.Setenv("HOME", t.TempDir())
	pl := planFor(t, repo)
	pl.SHA = "0000000000000000000000000000000000000000"
	_, err := WriteRecord(repo, pl, &Discharge{SHA: pl.SHA}, &Result{})
	if err == nil || !strings.Contains(err.Error(), "does not resolve") {
		t.Fatalf("want a refusal naming the unresolvable sha, got %v", err)
	}
}

func TestWriteRecordThenListRoundTrips(t *testing.T) {
	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n"})
	home := t.TempDir()
	t.Setenv("HOME", home)
	sha := headSHA(t, repo)

	pl := planFor(t, repo)
	pl.SHA = sha
	dis := &Discharge{SHA: sha, Tier: "1-2",
		CheckedClean: []CheckedClean{{Class: "phantom dependency", Method: "build+vet, 4 targets"}},
		NearMisses:   []string{"limit-rate starvation — refuted by the chunk clamp"},
		ReportPath:   "/tmp/report.html"}
	res := &Result{Coverage: Coverage{Repo: "17/25", Plan: "25/25"}, Status: "settled"}

	path, err := WriteRecord(repo, pl, dis, res)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(path, filepath.Join(home, ".slop-ferret", "records")) {
		t.Errorf("record written outside the store: %s", path)
	}
	if _, err := os.Stat(filepath.Join(repo, ".slop-ferret")); err == nil {
		t.Error("a record must never be written into the target repo")
	}

	got, err := ListRecords(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("ListRecords = %d records, want 1", len(got))
	}
	r := got[0]
	if r.SHA != sha || r.CoverageRepo != "17/25" || r.Tier != "1-2" {
		t.Errorf("round-trip lost fields: %+v", r)
	}
	// The attested half must survive: it is the half the next sweep reads to avoid re-spending
	// budget on classes already recorded clean, WITH the method used.
	if len(r.CheckedClean) != 1 || r.CheckedClean[0].Method == "" {
		t.Errorf("checked-clean method not recorded: %+v", r.CheckedClean)
	}
}

func TestRepoKeyPrefersTheOriginURL(t *testing.T) {
	repo := gitRepo(t, map[string]string{"a.go": "package a\n"})
	run(t, repo, "remote", "add", "origin", "https://github.com/robot-accomplice/ghola.git")
	key, err := RepoKey(repo)
	if err != nil {
		t.Fatal(err)
	}
	if key != "github.com/robot-accomplice/ghola" {
		t.Errorf("RepoKey = %q", key)
	}
}

func TestRepoKeyFallsBackForARemotelessRepo(t *testing.T) {
	key, err := RepoKey(gitRepo(t, map[string]string{"a.go": "package a\n"}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(key, "path-") {
		t.Errorf("RepoKey = %q, want a path- fallback", key)
	}
}
```

Add these helpers to `internal/gate/gate_test.go`:

```go
func run(t *testing.T, repo string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}

func headSHA(t *testing.T, repo string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gate/ -run 'TestWriteRecord|TestRepoKey' -v`
Expected: FAIL — `undefined: WriteRecord`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/gate/record.go`:

```go
package gate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// A Record is one sweep of one repository at one commit.
//
// TWO KINDS OF FIELD, and the split is the point. The COMPUTED half the tool derives itself. The
// ATTESTED half comes from the discharge, because it is judgement — which classes were checked
// clean and BY WHAT METHOD, what was refuted before filing, where the report went. The tool
// records the second kind; it never invents it.
//
// The next sweep reads this to avoid re-spending budget on classes already recorded clean. That is
// only safe if the method is recorded alongside the class: "clean" with no method is not checkable.
type Record struct {
	SHA          string `json:"sha"`
	Date         string `json:"date"`
	CoverageRepo string `json:"attested_repo"`
	CoveragePlan string `json:"attested_plan"`
	Denominator  int    `json:"denominator"`
	Waived       int    `json:"waived"`
	WorklistSize int    `json:"worklist_size"`
	UnmatchedSize int   `json:"unmatched_size"`
	Status       string `json:"status"`
	SkillVersion string `json:"skill_version,omitempty"`

	Tier              string         `json:"tier,omitempty"`
	FamiliesNotRun    []string       `json:"families_not_run,omitempty"`
	CheckedClean      []CheckedClean `json:"checked_clean,omitempty"`
	NearMisses        []string       `json:"near_misses,omitempty"`
	FindingsVerified  int            `json:"findings_verified,omitempty"`
	FindingsSuspected int            `json:"findings_suspected,omitempty"`
	ReportPath        string         `json:"report_path,omitempty"`
}

// CheckedClean is a class recorded clean together with the method used. Without the method a reader
// cannot check the claim, and an unchecked "clean" is how a later sweep skips ground nobody covered.
type CheckedClean struct {
	Class  string `json:"class"`
	Method string `json:"method"`
}

func recordsRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".slop-ferret", "records"), nil
}

// RepoKey identifies a repository stably across checkouts. The origin URL is preferred because a
// path changes when the tree moves; the hash fallback keeps remoteless repos usable.
func RepoKey(repo string) (string, error) {
	out, err := gitLines(repo, "remote", "get-url", "origin")
	if err == nil && len(out) > 0 {
		u := out[0]
		u = strings.TrimSuffix(u, ".git")
		u = strings.TrimPrefix(u, "https://")
		u = strings.TrimPrefix(u, "http://")
		if i := strings.Index(u, "@"); i >= 0 && strings.HasPrefix(u, "git@") {
			u = strings.Replace(u[i+1:], ":", "/", 1)
		}
		if u != "" {
			return u, nil
		}
	}
	abs, err := filepath.Abs(repo)
	if err != nil {
		return "", err
	}
	s := sha256.Sum256([]byte(abs))
	return "path-" + hex.EncodeToString(s[:])[:8], nil
}

// WriteRecord persists one sweep. It REFUSES a sha that does not resolve in the target: a boundary
// nobody can resolve makes the next sweep unable to scope itself, which is how a whole-repo re-read
// gets spent re-covering ground. Two prior sweeps recorded exactly that, both from dirty maps.
func WriteRecord(repo string, pl *Plan, dis *Discharge, res *Result) (string, error) {
	if _, err := gitLines(repo, "cat-file", "-e", pl.SHA+"^{commit}"); err != nil {
		return "", fmt.Errorf("sha %q does not resolve in %s — refusing to record a boundary "+
			"nobody can re-derive", pl.SHA, repo)
	}
	key, err := RepoKey(repo)
	if err != nil {
		return "", err
	}
	root, err := recordsRoot()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, filepath.FromSlash(key))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	dates, err := gitLines(repo, "show", "-s", "--format=%cs", pl.SHA)
	date := ""
	if err == nil && len(dates) > 0 {
		date = dates[0]
	}
	rec := Record{
		SHA: pl.SHA, Date: date,
		CoverageRepo: res.Coverage.Repo, CoveragePlan: res.Coverage.Plan,
		Denominator: pl.ProductionTotal, Waived: res.Coverage.Waived,
		WorklistSize: len(pl.HWorklist), UnmatchedSize: len(pl.HUnmatched),
		Status: res.Status,
		Tier:   dis.Tier, FamiliesNotRun: dis.FamiliesNotRun,
		CheckedClean: dis.CheckedClean, NearMisses: dis.NearMisses,
		FindingsVerified: dis.FindingsVerified, FindingsSuspected: dis.FindingsSuspected,
		ReportPath: dis.ReportPath,
	}
	b, err := json.MarshalIndent(rec, "", " ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, pl.SHA+".json")
	return path, os.WriteFile(path, b, 0o644)
}

// ListRecords returns prior sweeps of this repository, newest first.
func ListRecords(repo string) ([]Record, error) {
	key, err := RepoKey(repo)
	if err != nil {
		return nil, err
	}
	root, err := recordsRoot()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(key)))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Record
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(key), e.Name()))
		if err != nil {
			continue
		}
		var r Record
		if json.Unmarshal(b, &r) == nil {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date > out[j].Date })
	return out, nil
}
```

In `internal/gate/verify.go`, add the attested fields to `Discharge`:

```go
	Tier              string         `json:"tier"`
	CheckedClean      []CheckedClean `json:"checked_clean"`
	NearMisses        []string       `json:"near_misses"`
	FindingsVerified  int            `json:"findings_verified"`
	FindingsSuspected int            `json:"findings_suspected"`
	ReportPath        string         `json:"report_path"`
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/gate/ -race`
Expected: PASS.

- [ ] **Step 5: Wire the CLI**

In `cmd/ferret/main.go`, `cmdVerify` gains record-writing. `Verify` currently returns only `(*Result, int, error)`; add an exported helper in `verify.go` so the CLI does not re-read the files:

```go
// VerifyAndRecord runs Verify and, unless suppressed, persists a record. A record you must
// remember to request is one that will not exist when the next sweep looks for it, so this is
// always-write with an opt-out.
func VerifyAndRecord(planPath, dischargePath, repo string, record bool) (*Result, string, int, error) {
	res, code, err := Verify(planPath, dischargePath)
	if err != nil || !record || repo == "" {
		return res, "", code, err
	}
	pl, dis, lerr := loadPlanAndDischarge(planPath, dischargePath)
	if lerr != nil {
		return res, "", code, nil
	}
	path, werr := WriteRecord(repo, pl, dis, res)
	if werr != nil {
		return res, "", code, fmt.Errorf("record: %w", werr)
	}
	return res, path, code, nil
}
```

Refactor the file-reading preamble of `Verify` into `loadPlanAndDischarge(planPath, dischargePath string) (*Plan, *Discharge, error)` and call it from both.

Dispatch:

```go
	case "records":
		if len(args) != 1 {
			fmt.Fprintln(stderr, usage)
			return ExitMisuse
		}
		recs, err := gate.ListRecords(args[0])
		if err != nil {
			return fail(err, stderr)
		}
		for _, r := range recs {
			fmt.Fprintf(stdout, "%s  %s  repo %s  plan %s  %s\n",
				r.Date, r.SHA[:min(12, len(r.SHA))], r.CoverageRepo, r.CoveragePlan, r.Status)
		}
		return ExitOK
```

`cmdVerify` takes an optional third positional `<repo>` and `--no-record`; when `<repo>` is absent, no record is written and that is not an error.

- [ ] **Step 6: Remove the prose-ahead-of-code note**

`skill/SKILL.md` Step 7 references `~/.slop-ferret/records/`. That reference is now true. Bump `skill/VERSION` to the next date-style value (`2026-08-01.9`) since `skill/` changed.

- [ ] **Step 7: Commit**

```bash
just ci
git add -A
git commit -m "Add the records store

SKILL.md told sweeps to read a prior record and write one, and there was no
store -- prose ahead of code, in the tool that hunts that. Records carry a
computed half the tool derives and an attested half the discharge supplies,
including checked-clean WITH the method, because clean with no method is not
checkable. Refuses a sha that does not resolve: two prior sweeps recorded
boundaries nobody can re-derive."
```

---

## Task 5: Documentation conformance

**Files:**
- Modify: `README.md`, `CHANGELOG.md`, `docs/architecture/c4-component.md`, `docs/architecture/dataflow.md`, `docs/superpowers/specs/2026-08-01-slop-ferret-design.md`

**Interfaces:**
- Consumes: the finished behaviour of Tasks 1–4.
- Produces: nothing code depends on.

- [ ] **Step 1: Update the README**

Replace the "Two artifacts, two cadences" source table with the three rows from spec §5 (`install` / `--ref` / `--from`), remove every mention of an embedded copy, change all commands to `ferret`, add `records` to the command table, and document exit codes `0/2/3/4`.

- [ ] **Step 2: Update the architecture docs**

`c4-component.md`: delete the `//go:embed skill/` node and its "bootstrap floor" caption; the source model is now fetch/dir only. `dataflow.md`: in the install/update diagram replace the `embedded skill` node with `repo @ v<binVersion>`.

- [ ] **Step 3: Update the CHANGELOG**

Add under Unreleased → Changed: binary renamed to `ferret`; `install`/`update` synonyms; skill no longer embedded; exit code 4 introduced for refusals. Under Added: records store.

- [ ] **Step 4: Update the spec's delta table**

`docs/superpowers/specs/2026-08-01-slop-ferret-design.md` §10 — every "not built" row that this plan completed becomes **built**.

- [ ] **Step 5: Verify no stale claims survive**

```bash
grep -rn "embed\|slop-ferret plan\|slop-ferret install\|slop-ferret doctor" README.md docs/ CHANGELOG.md
```
Expected: no hits outside historical notes that explicitly say "was" or "removed".

- [ ] **Step 6: Commit**

```bash
just ci
git add -A
git commit -m "Bring the docs into conformance with the spec

Prose describing behaviour the code lacks is the defect this tool hunts."
```

---

## Self-Review

**Spec coverage.** D1 needs no work (already one binary). D2 → Task 2. D3 → Task 3. D4 → Task 3. D5 → already correct, guarded by `release.yml`; Task 3 changes where the gate reads the stamp. D6 → already built. D7 → Task 4. D8 → Task 3. D9 → no change by design. §6 records → Task 4. §7 exit codes → Task 1. §10 delta table → Tasks 1 and 5. **No spec section is unimplemented.**

**Placeholders.** None: every code step carries the actual code, every test step the actual test, every run step the exact command and expected result.

**Type consistency.** `Source`, `Fetch`, `DirSource`, `Install`, `Doctor`, `SkillVersion` keep their existing signatures. New: `DefaultSource(string) (Source, func(), error)`, `RepoKey(string) (string, error)`, `WriteRecord(string, *Plan, *Discharge, *Result) (string, error)`, `ListRecords(string) ([]Record, error)`, `VerifyAndRecord(string, string, string, bool) (*Result, string, int, error)`, `loadPlanAndDischarge(string, string) (*Plan, *Discharge, error)`, `CheckedClean{Class, Method}`. `Discharge` gains six optional fields, all with JSON tags matching the names used in Task 4's test. Exit constants are defined once in `internal/gate` and imported by `cmd/ferret` rather than respelled.

**Ordering.** Task 1 is independent. Task 2 must precede Task 3 (the embed path breaks on the move and Task 3 deletes it). Task 4 depends on Task 3 only for the CLI file's location. Task 5 last, because it documents the others.

**Known rough edge, deliberately kept:** Task 2 Step 2 leaves a synthetic-FS shim in the test file for exactly one task, because the alternative — duplicating `skill/` under `cmd/ferret/` so the embed keeps resolving — would create a second copy of the prose, which is the drift class this tool exists to find. A shim deleted one task later is the cheaper wrong.

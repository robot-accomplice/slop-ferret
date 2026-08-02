package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robot-accomplice/slop-ferret/internal/gate"
)

func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestNoArgsPrintsUsage(t *testing.T) {
	code, _, errs := runCLI(t)
	if code != gate.ExitMisuse || !strings.Contains(errs, "ferret") {
		t.Fatalf("code=%d err=%q", code, errs)
	}
}

func TestUnknownSubcommandPrintsUsage(t *testing.T) {
	if code, _, _ := runCLI(t, "sweep-everything"); code != 2 {
		t.Fatalf("code=%d, want 2", code)
	}
}

// release.yml parses `--version` to check the tag against the binary, the way the sibling
// projects' release gate does. If this stops printing a parseable version, releases silently stop
// being verifiable — so the spelling is pinned by a test rather than by convention.
func TestVersionIsParseableByTheReleaseGate(t *testing.T) {
	for _, spelling := range []string{"version", "--version", "-v"} {
		code, out, _ := runCLI(t, spelling)
		if code != 0 {
			t.Fatalf("%s: code=%d", spelling, code)
		}
		fields := strings.Fields(out)
		if len(fields) < 2 || fields[0] != "ferret" {
			t.Fatalf("%s: %q is not `ferret <version>`", spelling, out)
		}
		if fields[1] != binVersion {
			t.Errorf("%s: field 2 = %q, want %q — release.yml reads this field",
				spelling, fields[1], binVersion)
		}
	}
}

func TestPlanRejectsWrongArity(t *testing.T) {
	if code, _, _ := runCLI(t, "plan", "only-one-arg"); code != 2 {
		t.Fatal("plan needs three positional args")
	}
}

func TestVerifyRejectsWrongArity(t *testing.T) {
	if code, _, _ := runCLI(t, "verify", "only-one"); code != 2 {
		t.Fatal("verify needs two positional args")
	}
}

func TestPlanSurfacesTheGatesExitCode(t *testing.T) {
	// A map dir that does not exist is a REFUSAL, not a usage error and not an unfinished sweep.
	// The three are separate exit codes precisely so a wrapping script can tell them apart.
	code, _, errs := runCLI(t, "plan", filepath.Join(t.TempDir(), "nope"), "abc123", t.TempDir())
	if code != gate.ExitRefused {
		t.Fatalf("code=%d, want ExitRefused (%d): %s", code, gate.ExitRefused, errs)
	}
}

func TestInstallFromRejectsADirWithNoSkillTree(t *testing.T) {
	code, _, errs := runCLI(t, "install", "--from", t.TempDir())
	if code != 2 || !strings.Contains(errs, "skill") {
		t.Fatalf("code=%d err=%q", code, errs)
	}
}

func TestDoctorReportsNotInstalledOnACleanHome(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	code, out, _ := runCLI(t, "doctor")
	if code != 1 || !strings.Contains(out, "not installed") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestInstallThenDoctorRoundTrips(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if code, out, errs := runCLI(t, "install", "--from", fakeCheckout(t)); code != 0 {
		t.Fatalf("install code=%d out=%s err=%s", code, out, errs)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "slop-ferret", "SKILL.md")); err != nil {
		t.Fatalf("skill not deployed: %v", err)
	}
	code, out, _ := runCLI(t, "doctor")
	if code != 0 {
		t.Fatalf("doctor after install: code=%d out=%s", code, out)
	}
}

// A checkout stands in for the repo the default would fetch. Synthetic on purpose: these tests are
// about DEPLOYMENT and must not start failing because someone edited the real SKILL.md.
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

// install and update are synonyms (D4): they differ only in whether something is already there,
// which the tool can see for itself.
func TestInstallAndUpdateAreSynonyms(t *testing.T) {
	dir := fakeCheckout(t)
	for _, verb := range []string{"install", "update"} {
		t.Setenv("HOME", t.TempDir())
		if code, out, errs := runCLI(t, verb, "--from", dir); code != 0 {
			t.Fatalf("%s: code=%d out=%s err=%s", verb, code, out, errs)
		}
	}
}

// With no compiled-in copy and no tag to resolve, a bare install must say what to do instead of
// silently falling back to HEAD -- that fallback is the unpinned install the default exists to avoid.
func TestBareInstallBeforeAnyReleaseNamesTheAlternatives(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	code, _, errs := runCLI(t, "install")
	if code == 0 {
		t.Fatal("with no tag to resolve, a bare install must not silently succeed")
	}
	for _, want := range []string{"--from", "--ref"} {
		if !strings.Contains(errs, want) {
			t.Errorf("error must name %s: %q", want, errs)
		}
	}
}

// verify without a repo argument records nothing and is not an error: an absent repo is a narrower
// invocation, not a mistake.
func TestVerifyWithoutARepoStillVerifies(t *testing.T) {
	if code, _, _ := runCLI(t, "verify", "only-one"); code != gate.ExitMisuse {
		t.Fatal("verify still needs at least plan and discharge")
	}
}

func TestRecordsRejectsWrongArity(t *testing.T) {
	if code, _, _ := runCLI(t, "records"); code != gate.ExitMisuse {
		t.Fatal("records needs a repo")
	}
}

func TestRecordsOnAnUnsweptRepoPrintsNothingAndSucceeds(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	code, out, _ := runCLI(t, "records", t.TempDir())
	if code != gate.ExitOK {
		t.Fatalf("code=%d, want 0 — an unswept repo is a normal state", code)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("want no output, got %q", out)
	}
}

// A minimal plan/discharge pair on a real git repo. Written by hand rather than via magma: these
// tests are about the CLI's plumbing around verify, not about planning.
func verifyFixture(t *testing.T) (repo, planPath, dischargePath, sha string) {
	t.Helper()
	repo = t.TempDir()
	src := filepath.Join(repo, "internal", "wallet", "pay.go")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("package wallet\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "x"}} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	out, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	sha = strings.TrimSpace(string(out))

	dir := t.TempDir()
	planPath = filepath.Join(dir, "plan.json")
	dischargePath = filepath.Join(dir, "discharge.json")
	write := func(p string, v any) {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(planPath, map[string]any{
		"contract": "slop-gate/2", "sha": sha,
		"production_files": []string{"internal/wallet/pay.go"},
		"production_total": 1,
		"h_worklist":       []map[string]string{{"path": "internal/wallet/pay.go", "reason": "money/value"}},
		"h_required":       []map[string]string{{"path": "internal/wallet/pay.go", "reason": "money/value"}},
		"h_deferred":       []any{}, "h_unmatched": []string{}, "candidates": []any{},
		"unseeded_families": []string{"D", "E"},
	})
	write(dischargePath, map[string]any{
		"sha": sha, "read_paths": []string{"internal/wallet/pay.go"},
		"families_not_run": []string{"D", "E"}, "tier": "1-2",
		"checked_clean": []map[string]string{{"class": "phantom dependency", "method": "build+vet"}},
	})
	return repo, planPath, dischargePath, sha
}

// Three positionals: verify AND record. The record path goes to stderr so stdout stays clean JSON
// for a pipe.
func TestVerifyWithARepoWritesARecordAndKeepsStdoutParseable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo, pp, dp, sha := verifyFixture(t)

	code, out, errs := runCLI(t, "verify", pp, dp, repo)
	if code != gate.ExitOK {
		t.Fatalf("code=%d err=%s", code, errs)
	}
	var res map[string]any
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("stdout must be parseable JSON for a pipe: %v", err)
	}
	if !strings.Contains(errs, "recorded:") {
		t.Errorf("the record path belongs on stderr: %q", errs)
	}
	if _, err := os.Stat(filepath.Join(home, ".slop-ferret", "records")); err != nil {
		t.Errorf("no record written: %v", err)
	}
	_ = sha
}

// Two positionals: verify, record nothing, and that is not an error. An absent repo is a narrower
// invocation, not a mistake -- there is nothing to key a record by and nothing to resolve the sha
// against.
func TestVerifyWithoutARepoRecordsNothingAndSucceeds(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	_, pp, dp, _ := verifyFixture(t)

	code, _, errs := runCLI(t, "verify", pp, dp)
	if code != gate.ExitOK {
		t.Fatalf("code=%d err=%s", code, errs)
	}
	if strings.Contains(errs, "recorded:") {
		t.Errorf("no repo means no record: %q", errs)
	}
	if _, err := os.Stat(filepath.Join(home, ".slop-ferret", "records")); err == nil {
		t.Error("a record was written with no repo argument")
	}
}

func TestVerifyNoRecordSuppressesTheWrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo, pp, dp, _ := verifyFixture(t)

	code, _, errs := runCLI(t, "verify", pp, dp, repo, "--no-record")
	if code != gate.ExitOK {
		t.Fatalf("code=%d err=%s", code, errs)
	}
	if strings.Contains(errs, "recorded:") {
		t.Errorf("--no-record must suppress the write: %q", errs)
	}
	if _, err := os.Stat(filepath.Join(home, ".slop-ferret", "records")); err == nil {
		t.Error("--no-record still wrote a record")
	}
}

// An unfinished sweep exits 3 and still prints its result: the work queue is the useful output.
func TestVerifyReportsItemsOpenAndStillPrintsTheResult(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo, pp, dp, sha := verifyFixture(t)
	b, _ := json.Marshal(map[string]any{"sha": sha, "families_not_run": []string{"D", "E"}})
	if err := os.WriteFile(dp, b, 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ := runCLI(t, "verify", pp, dp, repo)
	if code != gate.ExitItemsOpen {
		t.Fatalf("code=%d, want ExitItemsOpen", code)
	}
	if !strings.Contains(out, "remaining") {
		t.Errorf("an unfinished sweep must still print its work queue: %s", out)
	}
}

// A malformed plan is MISUSE, not a refusal and not an unfinished sweep -- three distinct codes.
func TestVerifyOnAMalformedPlanIsMisuse(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "plan.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := runCLI(t, "verify", bad, bad); code != gate.ExitMisuse {
		t.Fatalf("code=%d, want ExitMisuse", code)
	}
}

// SUPERSEDED by TestARecordFailureKeepsTheVerifyResultAndTheRealExitCode below. The old test
// asserted that an UNSETTLED sweep surfaces a record failure; after the ABORT I1 fix an
// unsettled sweep no longer attempts a record at all — that is the normal case, not a
// failure — so the premise no longer exists.

// ABORT M2. A record failure must not discard the verify result or masquerade as misuse: it happens
// at the END of a sweep, which is the most expensive moment to lose one.
func TestARecordFailureKeepsTheVerifyResultAndTheRealExitCode(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo, pp, dp, _ := verifyFixture(t)
	for _, f := range []string{pp, dp} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		m["sha"] = "0000000000000000000000000000000000000000"
		nb, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, nb, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	code, out, errs := runCLI(t, "verify", pp, dp, repo)
	if code == gate.ExitMisuse {
		t.Errorf("a record failure is not misuse: code=%d err=%s", code, errs)
	}
	if out == "" {
		t.Error("the verify result must still be printed")
	}
	if !strings.Contains(errs, "warning") {
		t.Errorf("the record failure must be surfaced as a warning: %q", errs)
	}
}

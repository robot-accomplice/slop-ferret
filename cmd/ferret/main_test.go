package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robot-accomplice/slop-ferret/internal/gate"
	"github.com/robot-accomplice/slop-ferret/internal/report"
)

// failingForge points the source resolution at a server that 404s, so DefaultSource's ref
// resolution fails deterministically — the "no release to resolve" path — regardless of whether a
// real tag exists on the live forge. Before this, TestBareInstall* and TestInstallThenDoctor
// resolved the live repo and inverted the moment v0.1.0 was tagged; the release workflow re-runs the
// suite AT the tag, so they broke the first publish. Now they are hermetic.
func failingForge(t *testing.T) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "no such ref", http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("SLOP_FERRET_API_BASE", srv.URL+"/")
}

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
	if code, _, _ := runCLI(t, "enumerate", "only-one"); code != 2 {
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
	failingForge(t) // doctor must not report drift against a live-resolved source it happened to reach
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
		"skill/references/ai-slop-lexicon.md":  "# lexicon\n\n```h-signals\nmoney/value: pay|wallet\n```\n",
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

// With no compiled-in copy and no source to resolve, a bare install must say what to do instead of
// silently falling back to HEAD -- that fallback is the unpinned install the default exists to avoid.
// The no-source condition is forced by pointing resolution at a 404 forge, so the test holds whether
// or not a real release tag exists (it used to depend on v0.1.0 not being tagged yet).
func TestBareInstallWithNoResolvableSourceNamesTheAlternatives(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	failingForge(t)
	code, _, errs := runCLI(t, "install")
	if code == 0 {
		t.Fatal("with no source to resolve, a bare install must not silently succeed")
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
	if code, _, _ := runCLI(t, "enumerate", "only-one"); code != gate.ExitMisuse {
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

	code, out, errs := runCLI(t, "enumerate", pp, dp, repo)
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

	code, _, errs := runCLI(t, "enumerate", pp, dp)
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

	code, _, errs := runCLI(t, "enumerate", pp, dp, repo, "--no-record")
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
	code, out, _ := runCLI(t, "enumerate", pp, dp, repo)
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
	if code, _, _ := runCLI(t, "enumerate", bad, bad); code != gate.ExitMisuse {
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
	code, out, errs := runCLI(t, "enumerate", pp, dp, repo)
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

// THE SEAM THAT HAD NEVER CARRIED A BYTE.
//
// `report` used to take one model-written JSON file carrying every coverage figure, while a
// comment in internal/report claimed those figures "come from `enumerate`". They did not, and the
// field names did not even match — `enumerate` emits a nested `attested: {repo, plan}` against the
// renderer's flat `attested_repo`, so piping one into the other produced a page with blank
// fractions. No test noticed because every renderer test constructed the struct in Go and none
// exercised the JSON path at all.
//
// This runs the whole chain with real magma: plan -> discharge -> report, and asserts the page
// carries figures NOBODY TYPED.
func TestReportDerivesItsFiguresFromTheRealPlanAndDischarge(t *testing.T) {
	magma, err := exec.LookPath("magma")
	if err != nil {
		t.Fatalf("magma is not on PATH; this seam must be exercised end to end: %v", err)
	}
	// Self-provision the skill so `ferret plan` has a vocabulary. Without this the test reads the
	// skill deployed in the developer's ambient ~/.claude — present locally, absent on a fresh CI
	// runner, where `plan` then refuses with an empty H vocabulary (exit 4). Every other e2e here
	// installs into a temp HOME first; this one did not, and only first-push CI surfaced it.
	t.Setenv("HOME", t.TempDir())
	rootB, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	if code, _, errs := runCLI(t, "install", "--from", strings.TrimSpace(string(rootB))); code != gate.ExitOK {
		t.Fatalf("install skill so plan has a vocabulary: code=%d %s", code, errs)
	}
	repo := t.TempDir()
	for rel, body := range map[string]string{
		"go.mod":                 "module example.com/e2e\n\ngo 1.26\n",
		"internal/wallet/pay.go": "package wallet\n\nfunc Pay() {}\n",
		"internal/wallet/fee.go": "package wallet\n\nfunc Fee() {}\n",
		"internal/client/get.go": "package client\n\nfunc Get() {}\n",
	} {
		p := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, a := range [][]string{{"init", "-q"}, {"add", "-A"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "x"}} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, a...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", a, err, out)
		}
	}
	shaB, err := exec.Command("git", "-C", repo, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	sha := strings.TrimSpace(string(shaB))

	maps := t.TempDir()
	if b, err := exec.Command(magma, "--depth", "1", repo, "e2e", maps).CombinedOutput(); err != nil {
		t.Fatalf("magma: %v\n%s", err, b)
	}

	code, planOut, errs := runCLI(t, "plan", filepath.Join(maps, "e2e"), sha, repo)
	if code != gate.ExitOK {
		t.Fatalf("plan: code=%d %s", code, errs)
	}
	dir := t.TempDir()
	planPath := filepath.Join(dir, "plan.json")
	if err := os.WriteFile(planPath, []byte(planOut), 0o644); err != nil {
		t.Fatal(err)
	}
	var pl gate.Plan
	if err := json.Unmarshal([]byte(planOut), &pl); err != nil {
		t.Fatal(err)
	}

	// A PARTIAL sweep: read exactly one production file. The page must show that, and the number
	// must come from here rather than from anything the findings file says.
	if len(pl.ProductionFiles) < 2 {
		t.Fatalf("fixture produced %d production files, need >= 2 for a partial read",
			len(pl.ProductionFiles))
	}
	dis, _ := json.Marshal(map[string]any{
		"sha": sha, "read_paths": pl.ProductionFiles[:1],
		"families_not_run": pl.UnseededFamilies, "tier": "1-2",
	})
	disPath := filepath.Join(dir, "discharge.json")
	if err := os.WriteFile(disPath, dis, 0o644); err != nil {
		t.Fatal(err)
	}

	findings, _ := json.Marshal(map[string]any{
		"repo": "e2e", "skill_version": "x",
		"families_run": []string{"H"},
		"findings": []map[string]any{
			{"title": "a note", "severity": "note", "status": "VERIFIED", "file": "internal/wallet/pay.go"},
		},
	})
	findPath := filepath.Join(dir, "findings.json")
	if err := os.WriteFile(findPath, findings, 0o644); err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(dir, "r.html")
	if code, _, errs := runCLI(t, "report", planPath, disPath, findPath, outPath); code != gate.ExitOK {
		t.Fatalf("report: code=%d %s", code, errs)
	}
	page, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	body := string(page)

	// Anchored to the banner sentence, not a bare substring. A `Contains(body, "1/3")` check passes
	// on any other fraction that happens to render the same way — verified by mutation: hardcoding
	// AttestedRepo to "9/9" left a bare-substring version of this assertion green, because the plan
	// fraction supplied the match.
	want := fmt.Sprintf("The auditor states <strong>1/%d</strong> source files read",
		len(pl.ProductionFiles))
	if !strings.Contains(body, want) {
		t.Errorf("the banner must carry the DERIVED read fraction, computed here from the plan and "+
			"the discharge; nothing in findings.json mentions it.\nwant: %s", want)
	}
	if !strings.Contains(body, "incomplete") {
		t.Errorf("one file of %d read is an incomplete accounting and the page must say so",
			len(pl.ProductionFiles))
	}
	if !strings.Contains(body, "a note") {
		t.Error("the authored finding is missing from the page")
	}
}

// The retired single-file format carried the coverage figures. Accepting it silently would let an
// old input render a page whose numbers came from somewhere else while looking accepted, so the
// unknown fields are a refusal rather than a shrug.
func TestTheOldSingleFileReportFormatIsRefusedNotIgnored(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "old.json")
	if err := os.WriteFile(old, []byte(
		`{"repo":"x","attested_repo":"250/250","accounting":"complete","denominator":250}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := report.ParseAuthored(mustRead(t, old))
	if err == nil {
		t.Fatal("a findings file carrying attested_repo/accounting must be refused: those figures " +
			"are derived, and silently dropping them is how a typed-in fraction reached a page")
	}
	if !strings.Contains(err.Error(), "derived") {
		t.Errorf("the refusal must say where the figures now come from: %v", err)
	}
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

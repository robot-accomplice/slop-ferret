// Ported from python/test_gate.py alongside gate.py itself. Each test names a property whose
// absence was REPRODUCED on a real repository, not imagined — the measurements in the comments
// are the reason the constants are what they are, and a port is the easiest place to lose them.
package gate

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeMap(t *testing.T, sha, contract, fidelity string, computable bool, deadRows []map[string]any) string {
	t.Helper()
	d := filepath.Join(t.TempDir(), "m", ".magma")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	head := map[string]any{"contract_version": contract, "generator": "magma/0.1.0", "sha": sha,
		"fidelity": fidelity, "reachability_computable": computable}
	write := func(name string, extra map[string]any) {
		doc := map[string]any{}
		for k, v := range head {
			doc[k] = v
		}
		for k, v := range extra {
			doc[k] = v
		}
		b, _ := json.Marshal(doc)
		if err := os.WriteFile(filepath.Join(d, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var rows any = deadRows
	if !computable {
		rows = nil
	}
	write("_dead.json", map[string]any{"rows": rows})
	write("_test-only.json", map[string]any{"rows": []any{}})
	return filepath.Dir(d)
}

func gitRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	d := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(d, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "x"}} {
		if out, err := exec.Command("git", append([]string{"-C", d}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	return d
}

func code(err error) int {
	if e, ok := err.(*Err); ok {
		return e.Code
	}
	return 0
}

// ---- plan refusals: a map of the wrong tree or wrong shape must fail loud, not seed stale rows.

func TestRefusesOnWrongTreeSHA(t *testing.T) {
	m := writeMap(t, "OLDSHA", "codemap-rows/1", "rta", true, nil)
	_, err := BuildPlan(m, "NEWSHA", gitRepo(t, map[string]string{"a.go": "package a\n"}), "")
	if err == nil || code(err) != 3 || !strings.Contains(err.Error(), "different tree") {
		t.Fatalf("want refusal on sha mismatch, got %v", err)
	}
}

// A dirty-tree map stamps `<sha>+<diffhash>`, which can never equal a pinned commit. That is the
// point: a dirty map reports in-flight code as dead, and its sha is disproportionately likely to
// evaporate when commits are amended or rebased.
func TestRefusesADirtyMapAndSaysSo(t *testing.T) {
	m := writeMap(t, "abc123+dirty99", "codemap-rows/1", "rta", true, nil)
	_, err := BuildPlan(m, "abc123", gitRepo(t, map[string]string{"a.go": "package a\n"}), "")
	if err == nil || !strings.Contains(err.Error(), "DIRTY-tree map") {
		t.Fatalf("want a dirty-map refusal naming the cause, got %v", err)
	}
}

func TestRefusesOnUnsupportedContract(t *testing.T) {
	m := writeMap(t, "abc123", "codemap-graph/1", "rta", true, nil)
	_, err := BuildPlan(m, "abc123", gitRepo(t, map[string]string{"a.go": "package a\n"}), "")
	if err == nil || !strings.Contains(err.Error(), "NOT interchangeable") {
		t.Fatalf("want a contract refusal that names the three contracts, got %v", err)
	}
}

// An UNRECOGNISED fidelity silently fell through to the heuristic bar, so every candidate from a
// real RTA call graph read as a weak lead. Measured: 628 of 628 mislabelled. It errs safe, which
// is exactly why nobody noticed for the life of the table.
func TestUnrecognisedFidelityAnnouncesItself(t *testing.T) {
	m := writeMap(t, "abc123", "codemap-rows/1", "quantum-vibes", true,
		[]map[string]any{{"symbol": "F", "file": "a.go", "line": 1}})
	p, err := BuildPlan(m, "abc123", gitRepo(t, map[string]string{"a.go": "package a\n"}), "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p.Candidates[0].Bar, "UNRECOGNISED fidelity") {
		t.Errorf("bar must flag the unknown fidelity: %q", p.Candidates[0].Bar)
	}
}

func TestRTAFidelityAddsNoWeakLeadCaveat(t *testing.T) {
	m := writeMap(t, "abc123", "codemap-rows/1", "rta", true,
		[]map[string]any{{"symbol": "F", "file": "a.go", "line": 1}})
	p, _ := BuildPlan(m, "abc123", gitRepo(t, map[string]string{"a.go": "package a\n"}), "")
	if strings.Contains(p.Candidates[0].Bar, "weak lead") {
		t.Errorf("a real call graph must not be labelled a weak lead: %q", p.Candidates[0].Bar)
	}
}

// A family the map could not seed is a family that DID NOT RUN, and must reach the plan as such —
// the alternative is a missing input silently reading as a clean result.
func TestUnseededFamiliesAreReported(t *testing.T) {
	m := writeMap(t, "abc123", "codemap-rows/1", "rta", true, nil)
	p, _ := BuildPlan(m, "abc123", gitRepo(t, map[string]string{"a.go": "package a\n"}), "")
	got := strings.Join(p.UnseededFamilies, ",")
	if got != "D,E" {
		t.Errorf("unseeded_families = %q, want D,E", got)
	}
}

// ---- H enumeration

// ghola @4f33b3c — a pre-registered control repo — enumerated ZERO H-paths and so could never
// reach a verdict, because an HTTP fetch client whose whole surface is parsing untrusted remote
// responses matched none of the vocabulary. network/untrusted-io is the repair.
func TestAnHTTPClientEnumeratesNetworkPaths(t *testing.T) {
	repo := gitRepo(t, map[string]string{
		"internal/client/download.go": "package client\n",
		"internal/client/stream.go":   "package client\n",
		"internal/cookies/jar.go":     "package cookies\n",
		"internal/transport/tls.go":   "package transport\n",
	})
	m := writeMap(t, "abc123", "codemap-rows/1", "rta", true, nil)
	p, _ := BuildPlan(m, "abc123", repo, "")
	if len(p.HWorklist) == 0 {
		t.Fatal("an HTTP client must not enumerate zero H-paths")
	}
}

// The anchor was `(^|/)` alone, so internal/db/user_store.go was MISSED even though `store` is in
// the persistence vocabulary. Relaxing it to allow a word separator adds 81 files to roboticus's
// 285-path worklist.
func TestSignalsMatchAfterAWordSeparatorNotOnlyAtSegmentStart(t *testing.T) {
	repo := gitRepo(t, map[string]string{"internal/db/user_store.go": "package db\n"})
	m := writeMap(t, "abc123", "codemap-rows/1", "rta", true, nil)
	p, _ := BuildPlan(m, "abc123", repo, "")
	if len(p.HWorklist) != 1 {
		t.Fatalf("user_store.go must match `store` mid-filename: %+v", p.HWorklist)
	}
}

// Below the floor the whole worklist is required: deferral exists to make a LARGE worklist
// tractable, and on a repo readable in one pass it buys nothing and costs coverage.
func TestASmallWorklistIsNeverDeferred(t *testing.T) {
	repo := gitRepo(t, map[string]string{
		"internal/wallet/pay.go":      "package w\n",
		"internal/client/download.go": "package c\n",
	})
	m := writeMap(t, "abc123", "codemap-rows/1", "rta", true, nil)
	p, _ := BuildPlan(m, "abc123", repo, "")
	if len(p.HDeferred) != 0 || len(p.HRequired) != len(p.HWorklist) {
		t.Errorf("small worklist must be all-required: req=%d def=%d total=%d",
			len(p.HRequired), len(p.HDeferred), len(p.HWorklist))
	}
}

// THE COMPLEMENT. ghola's internal/bridge/bridge.go matched no signal AND had not changed since
// the baseline, so it appeared in neither h_worklist nor h_unmatched_changes and the gate returned
// COMPLETE without it. A hand read then made it the sweep's worst finding.
func TestAFileNoSignalReachedIsRaisedNotAbsent(t *testing.T) {
	repo := gitRepo(t, map[string]string{
		"internal/wallet/pay.go":    "package w\n",
		"internal/bridge/bridge.go": "package b\n",
		"vendor/x/y/lib.go":         "package y\n",
		"README.md":                 "# x\n",
	})
	m := writeMap(t, "abc123", "codemap-rows/1", "rta", true, nil)
	p, _ := BuildPlan(m, "abc123", repo, "")
	if !contains(p.HUnmatched, "internal/bridge/bridge.go") {
		t.Errorf("a signal miss must RAISE the file: %+v", p.HUnmatched)
	}
	if contains(p.ProductionFiles, "vendor/x/y/lib.go") {
		t.Error("vendored trees must stay out of the denominator")
	}
	if p.ProductionTotal != 2 {
		t.Errorf("production_total = %d, want 2: %v", p.ProductionTotal, p.ProductionFiles)
	}
}

// The denominator is an extension allowlist, so a language it omits must ANNOUNCE itself rather
// than quietly shrinking the total. That is the opposite choice from the H vocabulary, on purpose.
func TestAnUnknownLanguageAnnouncesItself(t *testing.T) {
	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n", "src/t.zig": "// z\n"})
	m := writeMap(t, "abc123", "codemap-rows/1", "rta", true, nil)
	p, _ := BuildPlan(m, "abc123", repo, "")
	if !contains(p.ProductionUnclassified, "src/t.zig") {
		t.Errorf("unclassified = %v", p.ProductionUnclassified)
	}
	if contains(p.ProductionFiles, "src/t.zig") {
		t.Error("an unrecognised extension must not enter the denominator silently")
	}
}

func TestSlopHSignalsExtendsTheVocabulary(t *testing.T) {
	repo := gitRepo(t, map[string]string{
		"internal/widget/shape.go": "package w\n",
		".slop-h-signals":          "domain/widget: (widget)\n",
	})
	m := writeMap(t, "abc123", "codemap-rows/1", "rta", true, nil)
	p, _ := BuildPlan(m, "abc123", repo, "")
	found := false
	for _, w := range p.HWorklist {
		if w.Reason == "domain/widget" {
			found = true
		}
	}
	if !found {
		t.Errorf("a per-repo signal must extend the vocabulary: %+v", p.HWorklist)
	}
}

// ---- verify

func writeJSON(t *testing.T, v any) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "f.json")
	b, _ := json.Marshal(v)
	os.WriteFile(p, b, 0o644)
	return p
}

func planFor(t *testing.T, repo string) *Plan {
	t.Helper()
	m := writeMap(t, "abc123", "codemap-rows/1", "rta", true, nil)
	p, err := BuildPlan(m, "abc123", repo, "")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestUnreadRequiredPathLeavesAnItemOpen(t *testing.T) {
	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n"})
	pl := planFor(t, repo)
	res, c, err := Verify(writeJSON(t, pl), writeJSON(t, map[string]any{"sha": "abc123"}))
	if err != nil {
		t.Fatal(err)
	}
	if c != 3 || res.Status != "open" {
		t.Fatalf("code=%d status=%s, want 3/open", c, res.Status)
	}
}

func TestReadingEverythingSettlesAndFillsBothFractions(t *testing.T) {
	repo := gitRepo(t, map[string]string{
		"internal/wallet/pay.go":    "package w\n",
		"internal/bridge/bridge.go": "package b\n",
	})
	pl := planFor(t, repo)
	res, c, _ := Verify(writeJSON(t, pl),
		writeJSON(t, map[string]any{"sha": "abc123", "read_paths": pl.ProductionFiles,
			"families_not_run": []string{"D", "E"}}))
	if c != 0 || res.Status != "settled" {
		t.Fatalf("code=%d status=%s remaining=%v", c, res.Status, res.Remaining)
	}
	if res.Coverage.Repo != "2/2" {
		t.Errorf("coverage.repo = %s, want 2/2", res.Coverage.Repo)
	}
}

// The whole reason a waiver needs no written justification. Choosing not to read a file is normal
// and should cost nothing to record — but the fraction exists to tell the person running the sweep
// what they actually looked at, so a waived file has to keep reading as unread. Nobody is scored.
func TestAWaiverSettlesTheItemAndDoesNotRaiseRepoCoverage(t *testing.T) {
	repo := gitRepo(t, map[string]string{
		"internal/wallet/pay.go":    "package w\n",
		"internal/bridge/bridge.go": "package b\n",
	})
	pl := planFor(t, repo)
	res, c, _ := Verify(writeJSON(t, pl), writeJSON(t, map[string]any{
		"sha": "abc123", "read_paths": []string{"internal/wallet/pay.go"},
		"coverage_waived":  []any{"internal/bridge/bridge.go"},
		"families_not_run": []string{"D", "E"}}))
	if c != 0 || res.Status != "settled" {
		t.Fatalf("a waiver must settle the accounting: %v", res.Remaining)
	}
	if res.Coverage.Repo != "1/2" {
		t.Errorf("coverage.repo = %s, want 1/2 — a waived file must still count as UNREAD",
			res.Coverage.Repo)
	}
	if res.Coverage.Waived != 1 {
		t.Errorf("waived = %d", res.Coverage.Waived)
	}
}

func TestAWaiverMayCarryAnOptionalReason(t *testing.T) {
	repo := gitRepo(t, map[string]string{
		"internal/wallet/pay.go":    "package w\n",
		"internal/bridge/bridge.go": "package b\n",
	})
	pl := planFor(t, repo)
	res, c, _ := Verify(writeJSON(t, pl), writeJSON(t, map[string]any{
		"sha": "abc123", "read_paths": []string{"internal/wallet/pay.go"},
		"coverage_waived": []any{map[string]string{"path": "internal/bridge/bridge.go",
			"reason": "covered last week"}},
		"families_not_run": []string{"D", "E"}}))
	if c != 0 {
		t.Fatalf("remaining=%v", res.Remaining)
	}
	if res.Coverage.Repo != "1/2" {
		t.Errorf("coverage.repo = %s", res.Coverage.Repo)
	}
}

// ghola's shape: every enumerated item dispositioned, most of the repo unread. One verdict word
// could not say that, and said COMPLETE.
func TestTheTwoFractionsCanDisagreeWhichIsThePoint(t *testing.T) {
	repo := gitRepo(t, map[string]string{
		"internal/wallet/pay.go":    "package w\n",
		"internal/bridge/bridge.go": "package b\n",
	})
	pl := planFor(t, repo)
	res, _, _ := Verify(writeJSON(t, pl), writeJSON(t, map[string]any{
		"sha": "abc123", "read_paths": []string{"internal/wallet/pay.go"},
		"coverage_waived":  []any{"internal/bridge/bridge.go"},
		"families_not_run": []string{"D", "E"}}))
	if res.Coverage.Plan != "2/2" {
		t.Errorf("plan fraction = %s, want fully dispositioned", res.Coverage.Plan)
	}
	if res.Coverage.Repo != "1/2" {
		t.Errorf("repo fraction = %s, want half read", res.Coverage.Repo)
	}
	if strings.Contains(res.Headline, "COMPLETE") {
		t.Error("the verdict triple must not come back")
	}
}

// A discharge from another sweep once satisfied any plan, and stale artifacts demonstrably survive
// across sessions.
func TestADischargeFromAnotherSweepDoesNotSatisfyThisPlan(t *testing.T) {
	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n"})
	pl := planFor(t, repo)
	res, c, _ := Verify(writeJSON(t, pl), writeJSON(t, map[string]any{
		"sha": "OTHER", "read_paths": pl.ProductionFiles, "families_not_run": []string{"D", "E"}}))
	if c != 3 {
		t.Fatal("a foreign discharge must not settle")
	}
	if !strings.Contains(strings.Join(res.Remaining, " "), "different sweep") {
		t.Errorf("remaining=%v", res.Remaining)
	}
}

// "COMPLETE, no findings" was the single most consequential thing this emitted, and the
// clean-sweep path — file nothing, clear nothing, attest the reads — was certified with every
// candidate unexamined. Measured against a real plan: 12 candidates, 0 cleared, COMPLETE, exit 0.
func TestACandidateNeitherClearedNorRefutedKeepsAnItemOpen(t *testing.T) {
	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n"})
	m := writeMap(t, "abc123", "codemap-rows/1", "rta", true,
		[]map[string]any{{"symbol": "Ghost", "file": "internal/wallet/pay.go", "line": 3}})
	pl, _ := BuildPlan(m, "abc123", repo, "")
	res, c, _ := Verify(writeJSON(t, pl), writeJSON(t, map[string]any{
		"sha": "abc123", "read_paths": pl.ProductionFiles, "families_not_run": []string{"D", "E"}}))
	if c != 3 {
		t.Fatal("an unexamined candidate must keep an item open")
	}
	if !strings.Contains(strings.Join(res.Remaining, " "), "neither cleared nor refuted") {
		t.Errorf("remaining=%v", res.Remaining)
	}
}

func TestRefutingACandidateIsEnoughToAccountForIt(t *testing.T) {
	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n"})
	m := writeMap(t, "abc123", "codemap-rows/1", "rta", true,
		[]map[string]any{{"symbol": "Ghost", "file": "internal/wallet/pay.go", "line": 3}})
	pl, _ := BuildPlan(m, "abc123", repo, "")
	_, c, _ := Verify(writeJSON(t, pl), writeJSON(t, map[string]any{
		"sha": "abc123", "read_paths": pl.ProductionFiles, "families_not_run": []string{"D", "E"},
		"candidates_refuted": []map[string]string{{"file": "internal/wallet/pay.go", "symbol": "Ghost"}}}))
	if c != 0 {
		t.Fatal("refuting is a cheap, valid disposition")
	}
}

// A filed candidate that never cleared its bar is an accusation without the evidence its class
// requires. Reproduced: 628 candidates, 0 cleared, COMPLETE, exit 0.
func TestAFiledCandidateThatNeverClearedItsBarKeepsAnItemOpen(t *testing.T) {
	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n"})
	m := writeMap(t, "abc123", "codemap-rows/1", "rta", true,
		[]map[string]any{{"symbol": "Ghost", "file": "internal/wallet/pay.go", "line": 3}})
	pl, _ := BuildPlan(m, "abc123", repo, "")
	res, c, _ := Verify(writeJSON(t, pl), writeJSON(t, map[string]any{
		"sha": "abc123", "read_paths": pl.ProductionFiles, "families_not_run": []string{"D", "E"},
		"candidates_filed":   []map[string]string{{"file": "internal/wallet/pay.go", "symbol": "Ghost"}},
		"candidates_refuted": []map[string]string{{"file": "internal/wallet/pay.go", "symbol": "Ghost"}}}))
	if c != 3 || !strings.Contains(strings.Join(res.Remaining, " "), "did not clear their bar") {
		t.Fatalf("code=%d remaining=%v", c, res.Remaining)
	}
}

// An unseeded family must be ACKNOWLEDGED, not merely printed at. Prose is not a gate.
func TestUnacknowledgedUnseededFamiliesKeepAnItemOpen(t *testing.T) {
	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n"})
	pl := planFor(t, repo)
	res, c, _ := Verify(writeJSON(t, pl), writeJSON(t, map[string]any{
		"sha": "abc123", "read_paths": pl.ProductionFiles}))
	if c != 3 || !strings.Contains(strings.Join(res.Remaining, " "), "families_not_run") {
		t.Fatalf("code=%d remaining=%v", c, res.Remaining)
	}
}

// A worklist that enumerated NOTHING cannot certify anything: "nothing to read" must never read as
// "everything was read".
func TestAnEmptyWorklistCannotSettle(t *testing.T) {
	repo := gitRepo(t, map[string]string{"zzz/qqq.go": "package q\n"})
	pl := planFor(t, repo)
	if len(pl.HWorklist) != 0 {
		t.Skip("fixture matched a signal; not the case under test")
	}
	res, c, _ := Verify(writeJSON(t, pl), writeJSON(t, map[string]any{
		"sha": "abc123", "read_paths": pl.ProductionFiles, "families_not_run": []string{"D", "E"}}))
	if c != 3 || !strings.Contains(strings.Join(res.Remaining, " "), "h_worklist is EMPTY") {
		t.Fatalf("code=%d remaining=%v", c, res.Remaining)
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

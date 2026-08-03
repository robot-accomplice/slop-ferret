// Ported from python/test_gate.py alongside gate.py itself. Each test names a property whose
// absence was REPRODUCED on a real repository, not imagined — the measurements in the comments
// are the reason the constants are what they are, and a port is the easiest place to lose them.
package gate

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func writeMap(t *testing.T, sha, contract, fidelity string, computable bool, deadRows []map[string]any) string {
	t.Helper()
	return writeMapTree(t, sha, sha, contract, fidelity, computable, deadRows)
}

func writeMapTree(t *testing.T, sha, tree, contract, fidelity string, computable bool, deadRows []map[string]any) string {
	t.Helper()
	d := filepath.Join(t.TempDir(), "m", ".magma")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	head := map[string]any{"contract_version": contract, "generator": "magma/0.1.0", "sha": sha,
		"fidelity": fidelity, "reachability_computable": computable}
	if tree != "" {
		head["tree"] = tree
	}
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
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
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
	if err == nil || code(err) != ExitRefused || !strings.Contains(err.Error(), "different tree") {
		t.Fatalf("want refusal on sha mismatch, got %v", err)
	}
}

// THIS TEST USED TO ASSERT A FICTION, and that is why the bug survived.
//
// It fed sha="abc123+dirty99" — a shape real magma never produces — and asserted the refusal named
// it as dirty. It passed for its whole life while the real dirty-map path went unguarded, because
// the fixture was written from the same wrong belief as the code it tested. A test agreeing with a
// bug is worse than no test: it certifies the belief.
//
// What is actually true: a sha that differs from the pinned one is refused as a MISMATCH (any
// cause), and dirtiness is detected via `tree` — see TestADirtyMapIsRefusedViaTheTreeField.
func TestAShaMismatchIsRefusedWhateverItLooksLike(t *testing.T) {
	for _, sha := range []string{"abc123+dirty99", "OTHERSHA"} {
		m := writeMap(t, sha, "codemap-rows/1", "rta", true, nil)
		_, err := BuildPlan(m, "abc123", gitRepo(t, map[string]string{"a.go": "package a\n"}), "")
		if err == nil || code(err) != ExitRefused {
			t.Fatalf("sha=%q: want a refusal, got %v", sha, err)
		}
		if !strings.Contains(err.Error(), "different tree") {
			t.Errorf("sha=%q: %v", sha, err)
		}
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
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
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
	res, c, err := Enumerate(writeJSON(t, pl), writeJSON(t, map[string]any{"sha": "abc123"}))
	if err != nil {
		t.Fatal(err)
	}
	if c != 3 || res.Accounting != "incomplete" {
		t.Fatalf("code=%d status=%s, want 3/open", c, res.Accounting)
	}
}

func TestReadingEverythingSettlesAndFillsBothFractions(t *testing.T) {
	repo := gitRepo(t, map[string]string{
		"internal/wallet/pay.go":    "package w\n",
		"internal/bridge/bridge.go": "package b\n",
	})
	pl := planFor(t, repo)
	res, c, _ := Enumerate(writeJSON(t, pl),
		writeJSON(t, map[string]any{"sha": "abc123", "read_paths": pl.ProductionFiles,
			"families_not_run": []string{"D", "E"}}))
	if c != 0 || res.Accounting != "complete" {
		t.Fatalf("code=%d status=%s remaining=%v", c, res.Accounting, res.Remaining)
	}
	if res.Attested.Repo != "2/2" {
		t.Errorf("attested.repo = %s, want 2/2", res.Attested.Repo)
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
	res, c, _ := Enumerate(writeJSON(t, pl), writeJSON(t, map[string]any{
		"sha": "abc123", "read_paths": []string{"internal/wallet/pay.go"},
		"coverage_waived":  []any{"internal/bridge/bridge.go"},
		"families_not_run": []string{"D", "E"}}))
	if c != 0 || res.Accounting != "complete" {
		t.Fatalf("a waiver must settle the accounting: %v", res.Remaining)
	}
	if res.Attested.Repo != "1/2" {
		t.Errorf("attested.repo = %s, want 1/2 — a waived file must still count as UNREAD",
			res.Attested.Repo)
	}
	if res.Attested.Waived != 1 {
		t.Errorf("waived = %d", res.Attested.Waived)
	}
}

func TestAWaiverMayCarryAnOptionalReason(t *testing.T) {
	repo := gitRepo(t, map[string]string{
		"internal/wallet/pay.go":    "package w\n",
		"internal/bridge/bridge.go": "package b\n",
	})
	pl := planFor(t, repo)
	res, c, _ := Enumerate(writeJSON(t, pl), writeJSON(t, map[string]any{
		"sha": "abc123", "read_paths": []string{"internal/wallet/pay.go"},
		"coverage_waived": []any{map[string]string{"path": "internal/bridge/bridge.go",
			"reason": "covered last week"}},
		"families_not_run": []string{"D", "E"}}))
	if c != 0 {
		t.Fatalf("remaining=%v", res.Remaining)
	}
	if res.Attested.Repo != "1/2" {
		t.Errorf("attested.repo = %s", res.Attested.Repo)
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
	res, _, _ := Enumerate(writeJSON(t, pl), writeJSON(t, map[string]any{
		"sha": "abc123", "read_paths": []string{"internal/wallet/pay.go"},
		"coverage_waived":  []any{"internal/bridge/bridge.go"},
		"families_not_run": []string{"D", "E"}}))
	if res.Attested.Plan != "2/2" {
		t.Errorf("plan fraction = %s, want fully dispositioned", res.Attested.Plan)
	}
	if res.Attested.Repo != "1/2" {
		t.Errorf("repo fraction = %s, want half read", res.Attested.Repo)
	}
	if strings.Contains(res.Headline, "COMPLETE") {
		t.Error("the verdict triple must not come back")
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
	res, c, _ := Enumerate(writeJSON(t, pl), writeJSON(t, map[string]any{
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
	_, c, _ := Enumerate(writeJSON(t, pl), writeJSON(t, map[string]any{
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
	res, c, _ := Enumerate(writeJSON(t, pl), writeJSON(t, map[string]any{
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
	res, c, _ := Enumerate(writeJSON(t, pl), writeJSON(t, map[string]any{
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
	res, c, _ := Enumerate(writeJSON(t, pl), writeJSON(t, map[string]any{
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

// Above the floor the worklist splits by CONSEQUENCE, not by a top-N cap: a numeric cap is a magic
// number nobody can defend and would silently drop whatever sorted last. roboticus @443681b9
// enumerated 387 paths with every one required, so no honest sweep there could reach a verdict.
func TestALargeWorklistSplitsByConsequence(t *testing.T) {
	files := map[string]string{}
	for i := 0; i < 40; i++ {
		files[fmt.Sprintf("internal/wallet/pay%02d.go", i)] = "package w\n"  // tier 1: money
		files[fmt.Sprintf("internal/client/http%02d.go", i)] = "package c\n" // tier 2: network
	}
	repo := gitRepo(t, files)
	p := planFor(t, repo)
	if len(p.HWorklist) <= hDeferFloor {
		t.Fatalf("fixture too small to exercise the split: %d", len(p.HWorklist))
	}
	if len(p.HDeferred) == 0 {
		t.Fatal("a large worklist must defer the volume tier")
	}
	for _, w := range p.HRequired {
		if hTier2[w.Reason] {
			t.Errorf("tier-2 reason %q must not be in the required tier", w.Reason)
		}
	}
	req := map[string]bool{}
	for _, w := range p.HRequired {
		req[w.Path] = true
	}
	for _, w := range p.HDeferred {
		if req[w.Path] {
			t.Errorf("%s is in both tiers", w.Path)
		}
	}
}

// --since compares the enumeration against a set already known to matter — what actually changed.
// Measured on roboticus: 6 of 6 production files changed in the last 12 release commits were on
// neither the 387-path worklist nor the 129-path required tier.
func TestSinceReportsChangedFilesNoSignalReached(t *testing.T) {
	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n"})
	writeAndCommit(t, repo, "internal/widget/shape.go", "package widget\n")
	m := writeMap(t, "abc123", "codemap-rows/1", "rta", true, nil)
	p, err := BuildPlan(m, "abc123", repo, "HEAD~1")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, u := range p.HUnmatchedChanges {
		got = append(got, u.Path)
	}
	if !contains(got, "internal/widget/shape.go") {
		t.Errorf("h_unmatched_changes = %v, want the unenumerated changed file", got)
	}
	if p.ChangeBaseline != "HEAD~1" {
		t.Errorf("change_baseline = %q", p.ChangeBaseline)
	}
}

func TestWithoutSinceThereIsNoChangeCrossCheck(t *testing.T) {
	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n"})
	writeAndCommit(t, repo, "internal/widget/shape.go", "package widget\n")
	p := planFor(t, repo)
	if len(p.HUnmatchedChanges) != 0 {
		t.Errorf("a whole-repo sweep has no change set to compare against: %v", p.HUnmatchedChanges)
	}
}

func TestAnUnreadableRepoFailsLoud(t *testing.T) {
	m := writeMap(t, "abc123", "codemap-rows/1", "rta", true, nil)
	_, err := BuildPlan(m, "abc123", filepath.Join(t.TempDir(), "not-a-git-repo"), "")
	if err == nil {
		t.Fatal("a non-repo must fail loud, not enumerate nothing and look clean")
	}
}

func writeAndCommit(t *testing.T, repo, rel, body string) {
	t.Helper()
	p := filepath.Join(repo, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-A"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "second"}} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
}

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
	_, c, err := Enumerate(writeJSON(t, pl), writeJSON(t, map[string]any{"sha": "abc123"}))
	if err != nil {
		t.Fatal(err)
	}
	if c != ExitItemsOpen {
		t.Fatalf("unfinished sweep exit = %d, want ExitItemsOpen (%d)", c, ExitItemsOpen)
	}
}

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

// THE REFUSAL THAT NEVER FIRED. SKILL.md and this package both claimed a dirty map "refuses by
// construction" because magma stamps a composite sha that can never equal a pinned commit.
// Reproduced against real magma 2026-08-02: it stamps `sha` with the CLEAN head sha and puts the
// marker in `tree` ("4f33b3c-dirty"). This gate compared `sha`, so it accepted a dirty map and
// exited 0 — the guarantee was prose only.
//
// A dirty map reports in-flight, not-yet-wired code as dead, and its boundary is disproportionately
// likely to evaporate when the commit is amended or rebased away. Two prior sweeps pinned exactly
// such a boundary and neither resolves today.
func TestADirtyMapIsRefusedViaTheTreeField(t *testing.T) {
	for _, tree := range []string{"abc123-dirty", "abc123+deadbeef"} {
		m := writeMapTree(t, "abc123", tree, "codemap-rows/1", "rta", true, nil)
		_, err := BuildPlan(m, "abc123", gitRepo(t, map[string]string{"a.go": "package a\n"}), "")
		if err == nil {
			t.Fatalf("tree=%q: a dirty map must be refused", tree)
		}
		if code(err) != ExitRefused {
			t.Errorf("tree=%q: exit %d, want ExitRefused", tree, code(err))
		}
		if !strings.Contains(err.Error(), "DIRTY") {
			t.Errorf("tree=%q: the refusal must name the cause: %v", tree, err)
		}
	}
}

func TestACleanMapWhoseTreeEqualsItsShaIsAccepted(t *testing.T) {
	m := writeMapTree(t, "abc123", "abc123", "codemap-rows/1", "rta", true, nil)
	if _, err := BuildPlan(m, "abc123", gitRepo(t, map[string]string{"a.go": "package a\n"}), ""); err != nil {
		t.Fatalf("a clean map must be accepted: %v", err)
	}
}

// An older map with no `tree` field at all must still work: absence is not evidence of dirtiness.
func TestAMapWithNoTreeFieldIsAccepted(t *testing.T) {
	m := writeMapTree(t, "abc123", "", "codemap-rows/1", "rta", true, nil)
	if _, err := BuildPlan(m, "abc123", gitRepo(t, map[string]string{"a.go": "package a\n"}), ""); err != nil {
		t.Fatalf("a map without a tree field must be accepted: %v", err)
	}
}

// A REFUSED MAP MUST NOT READ AS A CLEAN ONE.
//
// magma distinguishes rows:null (the analysis could not run) from rows:[] (it ran and found
// nothing) and its contract calls that distinction load-bearing: "a refusal must never be mistaken
// for 'found nothing'". This gate discarded it. Reproduced 2026-08-02 against a real refused map
// (magma 0.1.0 has no Rust parser): the plan came back with 0 candidates, no reason, and family A
// absent from unseeded_families — so a sweep could legitimately report family A checked-clean over
// an analysis that never ran.
func TestARefusedMapMarksFamilyANotSeededAndCarriesTheReason(t *testing.T) {
	m := writeRefusedMap(t, "abc123", `language "rust" detected but its parser is not built yet`)
	p, err := BuildPlan(m, "abc123", gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n"}), "")
	if err != nil {
		t.Fatalf("a refusal is not a hard error — the H read still applies: %v", err)
	}
	if !contains(p.UnseededFamilies, "A") {
		t.Errorf("family A was NOT computed, so it must be declared unseeded: %v", p.UnseededFamilies)
	}
	if p.NotComputableReason == "" {
		t.Error("the reason magma refused must reach the plan; dropping it loses the WHY")
	}
	if !strings.Contains(strings.Join(p.UnseededDetailValues(), " "), "rust") {
		t.Errorf("the unseeded detail for A must name the reason: %v", p.UnseededDetail)
	}
}

// And the discharge must then be forced to acknowledge it, exactly as for D and E.
func TestASweepOverARefusedMapCannotSettleWithoutDeclaringA(t *testing.T) {
	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n"})
	m := writeRefusedMap(t, "abc123", "no parser")
	p, _ := BuildPlan(m, "abc123", repo, "")
	res, code, _ := Enumerate(writeJSON(t, p), writeJSON(t, map[string]any{
		"sha": "abc123", "read_paths": p.ProductionFiles,
		"families_not_run": []string{"D", "E"}})) // A omitted on purpose
	if code != ExitItemsOpen {
		t.Fatalf("code=%d: omitting A must leave an item open", code)
	}
	if !strings.Contains(strings.Join(res.Remaining, " "), "A") {
		t.Errorf("remaining must name family A: %v", res.Remaining)
	}
}

func writeRefusedMap(t *testing.T, sha, reason string) string {
	t.Helper()
	d := filepath.Join(t.TempDir(), "m", ".magma")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"_dead.json", "_test-only.json"} {
		body := map[string]any{"contract_version": "codemap-rows/1", "generator": "magma/0.1.0",
			"sha": sha, "tree": sha, "fidelity": "", "reachability_computable": false,
			"not_computable_reason": reason, "rows": nil}
		b, _ := json.Marshal(body)
		if err := os.WriteFile(filepath.Join(d, n), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Dir(d)
}

// The vocabulary lives in the deployed skill, not in this binary, so it can be iterated from usage
// feedback without a binary release. These tests pin that: the file is the source, and the binary
// carries no fallback table that could silently disagree with it.
func TestTheVocabularyComesFromTheLexiconNotTheBinary(t *testing.T) {
	dir := t.TempDir()
	sig := filepath.Join(dir, "lexicon.md")
	if err := os.WriteFile(sig, []byte("# lexicon\n\nprose about classes\n\n"+
		"```h-signals\n# a comment\n\ndomain/widget: (widget|sprocket)\n```\n"+
		"more prose, ignored\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := LexiconPath
	LexiconPath = func() string { return sig }
	defer func() { LexiconPath = old }()

	repo := gitRepo(t, map[string]string{
		"internal/widget/shape.go": "package w\n",
		"internal/wallet/pay.go":   "package p\n", // built-in money word — must NOT match now
	})
	p := planFor(t, repo)
	if len(p.HWorklist) != 1 || p.HWorklist[0].Reason != "domain/widget" {
		t.Fatalf("the file is the only source of signals: %+v", p.HWorklist)
	}
	if !contains(p.HUnmatched, "internal/wallet/pay.go") {
		t.Error("with no built-in table, a money-shaped path must be unmatched, not silently matched")
	}
}

// REVERSED 2026-08-02. This test used to be named ...YieldsAnEmptyWorklistNotAFailure and asserted
// that `plan` exits 0 with no vocabulary, on the stated ground that "the empty-worklist stop in
// enumerate says what to do about it far better than a parse error would."
//
// That reasoning was wrong on three facts, all reproduced:
//
//  1. The enumerate stop names the WRONG remedy — "extend the signals via `.slop-h-signals`" — which
//     sends the operator to write regexes into the TARGET repo when the cause is that the skill was
//     never installed. `ferret install` is not mentioned.
//  2. It arrives two commands later. In between, plan.json says nothing about a lexicon at all; a
//     grep of its 56 lines for "lexicon", "skill" or "signal" matches only file paths.
//  3. A PARTIAL load defeats it entirely. One unbalanced paren in the lexicon drops a single signal
//     class while leaving others, so the worklist is non-empty, the stop never fires, and the sweep
//     records family H checked clean having quietly demoted `internal/auth/session.go` to the
//     cheap-to-waive complement.
//
// This is the default state of a `go install`ed binary before `ferret install` has ever run, so the
// silent path was the common path.
func TestAnEmptyVocabularyIsRefusedAtPlanTime(t *testing.T) {
	old := LexiconPath
	LexiconPath = func() string { return filepath.Join(t.TempDir(), "absent") }
	defer func() { LexiconPath = old }()

	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package p\n"})
	m := writeMap(t, "abc123", "codemap-rows/1", "rta", true, nil)
	_, err := BuildPlan(m, "abc123", repo, "")
	if err == nil {
		t.Fatal("a sweep with no vocabulary enumerates nothing and reports the same as a clean " +
			"repo; plan must refuse rather than hand back an empty worklist")
	}
	if c := code(err); c != ExitRefused {
		t.Errorf("exit = %d, want %d (refused): nothing was measured, which is not the same as "+
			"items being open", c, ExitRefused)
	}
	if !strings.Contains(err.Error(), "ferret install") {
		t.Errorf("the refusal must name the actual remedy — the skill is not deployed — rather "+
			"than sending the operator to write regexes into the target repo: %v", err)
	}
}

// A signal that will not compile is refused, not skipped. Skipping silently demotes every path it
// would have matched from the blast-radius tier into the cheap-to-waive complement, while the sweep
// still reports family H covered: measured, one unbalanced paren moved internal/auth/session.go out
// of h_required with exit 0 and no warning.
func TestAnUncompilableSignalIsRefusedNotSkipped(t *testing.T) {
	old := LexiconPath
	dir := t.TempDir()
	lex := filepath.Join(dir, "lex.md")
	if err := os.WriteFile(lex, []byte("```h-signals\nauth/session: (auth|session\n```\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	LexiconPath = func() string { return lex }
	defer func() { LexiconPath = old }()

	repo := gitRepo(t, map[string]string{"internal/auth/session.go": "package a\n"})
	m := writeMap(t, "abc123", "codemap-rows/1", "rta", true, nil)
	_, err := BuildPlan(m, "abc123", repo, "")
	if err == nil {
		t.Fatal("an uncompilable signal must be refused: skipping it silently shrinks the " +
			"blast-radius tier while the sweep still reports that tier covered")
	}
	if !strings.Contains(err.Error(), "does not compile") {
		t.Errorf("the refusal must name the broken pattern: %v", err)
	}
}

// The vocabulary now lives outside the binary, on a cadence the binary does not control, so the
// plan records what actually loaded. Without this a sweep against a half-loaded lexicon leaves a
// plan byte-identical to one over a repo whose files genuinely matched nothing.
func TestThePlanRecordsWhereItsVocabularyCameFrom(t *testing.T) {
	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package p\n"})
	p := planFor(t, repo)
	for _, k := range []string{"lexicon", "lexicon_version", "signals_total"} {
		if p.VocabProvenance[k] == "" {
			t.Errorf("vocab_provenance is missing %q: %v", k, p.VocabProvenance)
		}
	}
	if p.VocabProvenance["signals_total"] == "0" {
		t.Error("a plan that loaded no signals should not have been produced at all")
	}
}

// The shipped vocabulary must parse and every entry must compile — a typo in a regex would silently
// drop a whole signal class.
func TestTheShippedLexiconSignalsParseAndCompile(t *testing.T) {
	got := parseLexiconSignals(filepath.Join(repoRoot(t), "skill", "references", "ai-slop-lexicon.md"))
	if len(got) < 5 {
		t.Fatalf("shipped vocabulary looks empty: %d entries", len(got))
	}
	for _, pair := range got {
		if _, err := regexp.Compile(`(?i)` + anchor + `(` + pair[1] + `)`); err != nil {
			t.Errorf("signal %q does not compile: %v", pair[0], err)
		}
	}
}

// CONDITION 8. The tier split's behaviour is now RE-DERIVABLE by someone who was not there.
//
// gate.go instructed "re-measure required/deferred on real repos after any edit here" and cited
// repositories that are private, unpinned, and in one case a sha that no longer resolves — an
// unfollowable instruction. Meanwhile README and the C4 doc both claimed "the measurements live in
// comments beside the tests that pin them", and no test pinned hDeferFloor at all: lowering it kept
// CI green.
//
// This pins the three constants that decide the split, against a committed fixture.
func TestTheTierSplitIsReDerivableFromAFixture(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "tier-split-fixture.txt"))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			files[l] = "package x\n"
		}
	}
	if len(files) != 70 {
		t.Fatalf("fixture has %d paths, expected 70", len(files))
	}
	p := planFor(t, gitRepo(t, files))

	if len(p.HWorklist) <= hDeferFloor {
		t.Fatalf("fixture (%d matched) must exceed hDeferFloor (%d) or the split never engages",
			len(p.HWorklist), hDeferFloor)
	}
	if len(p.HRequired) != 40 || len(p.HDeferred) != 30 {
		t.Errorf("split = %d required / %d deferred, want 40/30. If you changed hSignalSrc, "+
			"hTier1, hTier2, hDeferFloor or anchor, this is the measurement that moved — "+
			"re-derive it deliberately rather than updating the number to match",
			len(p.HRequired), len(p.HDeferred))
	}
	for _, w := range p.HRequired {
		if hTier2[w.Reason] {
			t.Errorf("tier-2 reason %q leaked into the required tier", w.Reason)
		}
	}
}

// hDeferFloor is a judgement, but its EFFECT is testable, and this test PINS IT FROM BELOW.
//
// The previous version built 10 files and called `t.Skip("fixture unexpectedly large")` whenever
// the worklist exceeded the floor — so lowering the floor made the test skip itself out of
// existence rather than fail. Verified by mutation on 2026-08-02: hDeferFloor 60 -> 5 left the
// whole suite green, which is the exact defect the fixture's own header claims to have closed.
// A test that opts out precisely when its constant moves is indistinguishable from no test.
//
// 50 paths, deliberately mixed 30 tier-1 / 20 tier-2 so that a split WOULD be visible if one
// engaged. At the real floor nothing defers. Lower the floor under 50 and the 20 tier-2 paths
// move to deferred and this fails. TestTheTierSplitIsReDerivableFromAFixture pins the other end
// at 70, so the floor is bracketed rather than bounded on one side only.
func TestLoweringTheDeferFloorIsCaught(t *testing.T) {
	const below = 50
	files := map[string]string{}
	for i := 0; i < 30; i++ {
		files[fmt.Sprintf("internal/wallet/pay%02d.go", i)] = "package x\n"
	}
	for i := 0; i < 20; i++ {
		files[fmt.Sprintf("internal/client/download%02d.go", i)] = "package x\n"
	}
	p := planFor(t, gitRepo(t, files))
	if len(p.HWorklist) != below {
		t.Fatalf("fixture matched %d paths, expected %d — the signals moved, so this test no "+
			"longer measures the floor", len(p.HWorklist), below)
	}
	if below > hDeferFloor {
		t.Fatalf("hDeferFloor is %d, at or below this fixture's %d paths: the floor has been "+
			"lowered far enough that a 50-path repo now defers work. The floor exists so that a "+
			"worklist small enough to read in full IS read in full — re-derive it deliberately "+
			"rather than lowering it to make a sweep finish sooner", hDeferFloor, below)
	}
	if len(p.HDeferred) != 0 || len(p.HRequired) != len(p.HWorklist) {
		t.Errorf("at or below the floor everything is required: %d required / %d deferred",
			len(p.HRequired), len(p.HDeferred))
	}
}

// Tests must not depend on what happens to be installed on the machine running them. LexiconPath
// defaults to the DEPLOYED skill, which is correct at runtime and non-hermetic in a test: a
// developer with no skill installed, or an older one, would get different results from CI.
//
// So the suite pins the vocabulary to the copy shipped in THIS repo. That also makes every
// tier-split assertion below a statement about the shipped vocabulary rather than about a machine.
func TestMain(m *testing.M) {
	// Pin the vocabulary to the SHIPPED lexicon so these tests do not depend on what happens to be
	// installed on the machine running them. Falling back to the deployed copy would make the suite
	// pass or fail on a developer's ~/.claude — and this used to fall back SILENTLY, which is the
	// same "absence renders as a value" defect the tests below exist to catch.
	root, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot locate the repo root to pin the lexicon: %v\n", err)
		os.Exit(1)
	}
	shipped := filepath.Join(strings.TrimSpace(string(root)), "skill", "references", "ai-slop-lexicon.md")
	if _, err := os.Stat(shipped); err != nil {
		fmt.Fprintf(os.Stderr, "the shipped lexicon is missing (%v).\nThese tests must run against "+
			"it, not against whatever is deployed in ~/.claude — a silent fallback would make the "+
			"whole H-enumeration suite measure the developer's machine.\n", err)
		os.Exit(1)
	}
	LexiconPath = func() string { return shipped }
	os.Exit(m.Run())
}

// Prose outside the fence must never be read as a signal — the lexicon is mostly prose, and a
// stray `word: definition` line in it would otherwise become a live matching rule.
func TestOnlyTheFencedBlockIsRead(t *testing.T) {
	f := filepath.Join(t.TempDir(), "lex.md")
	if err := os.WriteFile(f, []byte(
		"| **Dead on arrival** | Built, tested, documented; zero production call sites | ... |\n"+
			"note: this line is prose and must be ignored\n"+
			"```h-signals\nmoney/value: (wallet)\n```\n"+
			"trailing: also prose, also ignored\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := parseLexiconSignals(f)
	if len(got) != 1 || got[0][0] != "money/value" {
		t.Fatalf("only the fenced block is a signal source: %+v", got)
	}
}

// `.slop-h-signals` comes from the repository being audited, and matching is O(files x signals), so
// its size is a cost the TARGET controls. Measured: 2,000 signals over 2,000 paths took 59.9s; a
// committed 100k-line file is hours. Refuse loudly rather than truncate — silently reading the
// first N would make the worklist depend on line order.
func TestAnOversizedSignalFileIsRefused(t *testing.T) {
	for _, c := range []struct{ name, body, want string }{
		{"too many signals", strings.Repeat("money/value: pay\n", maxSignalLines+1), "over the cap"},
		{"too many bytes", "money/value: " + strings.Repeat("a|", maxSignalFileBytes/2+1) + "b\n",
			"over the"},
	} {
		t.Run(c.name, func(t *testing.T) {
			repo := gitRepo(t, map[string]string{
				"internal/wallet/pay.go": "package p\n",
				".slop-h-signals":        c.body,
			})
			m := writeMap(t, "abc123", "codemap-rows/1", "rta", true, nil)
			_, err := BuildPlan(m, "abc123", repo, "")
			if err == nil {
				t.Fatal("an unbounded signal file from the audited repo must be refused")
			}
			if code(err) != ExitRefused || !strings.Contains(err.Error(), c.want) {
				t.Errorf("code=%d err=%v", code(err), err)
			}
		})
	}
}

// The cap must not break ordinary use: a handful of project-specific signals is the feature.
func TestAModestSignalFileStillWorks(t *testing.T) {
	repo := gitRepo(t, map[string]string{
		"internal/widget/shape.go": "package w\n",
		".slop-h-signals":          "domain/widget: widget\n",
	})
	p := planFor(t, repo)
	if len(p.HWorklist) == 0 {
		t.Fatal("a repo-supplied signal must still enumerate")
	}
}

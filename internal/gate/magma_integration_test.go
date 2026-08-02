package gate

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ABORT CONDITION 2. The magma seam is this tool's entire reason to exist, and until 2026-08-02 it
// had NEVER been executed against real magma in any test. Every fixture was hand-written from the
// author's belief about the contract, so the tests agreed with the bugs: the dirty-map guard read
// the wrong field, the refusal signal was discarded, `semantic` was absent from the fidelity table
// and `limitations` was parsed away. Four of the ten findings in the review campaign lived in this
// seam, and no unit test could see any of them.
//
// This test runs REAL magma. It does not skip when magma is absent — a test that silently skips is
// indistinguishable from a test that ran and passed, which is the same defect class the tool hunts.
// CI installs magma explicitly; if it is missing, this fails and says so.
func TestRealMagmaEnvelopeIsFullyConsumed(t *testing.T) {
	magma, err := exec.LookPath("magma")
	if err != nil {
		t.Fatalf("magma is not on PATH. This test must not skip: an integration test that " +
			"silently skips proves nothing and is how this seam went untested. " +
			"Install it: go install github.com/robot-accomplice/magma@latest")
	}

	repo := gitRepo(t, map[string]string{
		"go.mod":                 "module example.com/fixture\n\ngo 1.26\n",
		"main.go":                "package main\n\nfunc main() { used() }\n\nfunc used() {}\n\nfunc orphan() {}\n",
		"internal/wallet/pay.go": "package wallet\n\nfunc Pay() {}\n",
	})
	sha := strings.TrimSpace(runOut(t, repo, "rev-parse", "--short", "HEAD"))

	out := t.TempDir()
	if b, err := exec.Command(magma, "--depth", "1", repo, "fixture", out).CombinedOutput(); err != nil {
		t.Fatalf("magma failed: %v\n%s", err, b)
	}
	mapDir := filepath.Join(out, "fixture")

	// 1. Every field real magma emits must be consumed, not silently dropped.
	raw, err := os.ReadFile(filepath.Join(mapDir, ".magma", "_dead.json"))
	if err != nil {
		t.Fatal(err)
	}
	var emitted map[string]any
	if err := json.Unmarshal(raw, &emitted); err != nil {
		t.Fatal(err)
	}
	var parsed rowDoc
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{"contract_version": true, "generator": true, "sha": true,
		"tree": true, "fidelity": true, "reachability_computable": true,
		"not_computable_reason": true, "rows": true, "limitations": true}
	for k := range emitted {
		if !known[k] {
			t.Errorf("real magma emits %q and this gate does not know it exists — the last four "+
				"times that happened it was a live defect", k)
		}
	}

	// 2. THE FIDELITY TABLE IS THE PRODUCER'S VOCABULARY. A value magma really emits must have a
	//    bar, or every candidate from it is mislabelled as the weakest evidence.
	if parsed.Fidelity == "" {
		t.Fatal("magma emitted no fidelity")
	}
	if _, ok := fidelityBar[parsed.Fidelity]; !ok {
		t.Errorf("real magma emitted fidelity %q which is not in fidelityBar %v — candidates from "+
			"it will be labelled UNRECOGNISED and treated as the weakest evidence",
			parsed.Fidelity, keysOf(fidelityBar))
	}

	// 3. The plan must build from a real map and carry its provenance.
	p, err := BuildPlan(mapDir, sha, repo, "")
	if err != nil {
		t.Fatalf("BuildPlan over a real magma map: %v", err)
	}
	if p.MapProvenance["generator"] == "" || !strings.HasPrefix(p.MapProvenance["generator"], "magma/") {
		t.Errorf("provenance lost: %v", p.MapProvenance)
	}
	if p.Contract != planContract {
		t.Errorf("plan contract = %q", p.Contract)
	}

	// 4. Round-trip through verify, which must accept a plan it produced itself.
	disp := make([]map[string]string, 0, len(p.Candidates))
	for _, c := range p.Candidates {
		disp = append(disp, map[string]string{"file": c.File, "symbol": c.Symbol})
	}
	pp, dp := writeJSON(t, p), writeJSON(t, map[string]any{
		"sha": sha, "read_paths": p.ProductionFiles,
		"families_not_run": p.UnseededFamilies, "candidates_refuted": disp})
	res, code, err := Enumerate(pp, dp)
	if err != nil || code != ExitOK {
		t.Errorf("enumerate over a real-magma plan: code=%d err=%v remaining=%v", code, err, res.Remaining)
	}
}

// If magma ever declares limitations (v0.3.0+ adds them), they must reach the candidate bars rather
// than being parsed away — magma's contract names this gate as the consumer they exist for.
func TestDeclaredLimitationsReachTheCandidateBar(t *testing.T) {
	d := filepath.Join(t.TempDir(), "m", ".magma")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"contract_version": "codemap-rows/1", "generator": "magma/0.3.0",
		"sha": "abc123", "tree": "abc123", "fidelity": "rta", "reachability_computable": true,
		"limitations": []map[string]string{{"id": "go-closure-edges", "effect": "may-omit-edges",
			"description": "a function called only through a closure can appear unreachable"}},
		"rows": []map[string]any{{"symbol": "Ghost", "file": "internal/wallet/pay.go", "line": 3}}}
	b, _ := json.Marshal(body)
	for _, n := range []string{"_dead.json", "_test-only.json"} {
		if n == "_test-only.json" {
			body["rows"] = []map[string]any{}
			b2, _ := json.Marshal(body)
			if err := os.WriteFile(filepath.Join(d, n), b2, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.WriteFile(filepath.Join(d, n), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := BuildPlan(filepath.Dir(d), "abc123",
		gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n"}), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Candidates) == 0 {
		t.Fatal("expected a candidate")
	}
	if !strings.Contains(p.Candidates[0].Bar, "go-closure-edges") {
		t.Errorf("the producer's declared limitation must be on the bar the sweeper clears: %q",
			p.Candidates[0].Bar)
	}
	if len(p.MapLimitations) != 1 {
		t.Errorf("map_limitations = %v", p.MapLimitations)
	}
}

// A plan from a different ferret must be refused, not silently degraded to zeros.
func TestAForeignPlanContractIsRefused(t *testing.T) {
	pp := writeJSON(t, map[string]any{"contract": "slop-gate/1", "sha": "x",
		"h_worklist": []map[string]string{{"path": "a.go", "reason": "money/value"}}})
	dp := writeJSON(t, map[string]any{"sha": "x"})
	_, code, err := Enumerate(pp, dp)
	if code != ExitRefused || err == nil {
		t.Fatalf("code=%d err=%v — a foreign plan contract must refuse, not settle", code, err)
	}
}

func runOut(t *testing.T, repo string, args ...string) string {
	t.Helper()
	b, err := exec.Command("git", append([]string{"-C", repo}, args...)...).Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

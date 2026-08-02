package gate

import (
	"encoding/json"
	"strings"
	"testing"
)

// The skeleton must start WRONG. A template that defaults to "everything done" is a template that
// gets submitted unread — which is the failure this whole tool exists because of.
func TestSkeletonStartsUndispositionedSoAnUneditedOneIsIncomplete(t *testing.T) {
	repo := gitRepo(t, map[string]string{
		"internal/wallet/pay.go":    "package w\n",
		"internal/bridge/bridge.go": "package b\n",
	})
	m := writeMap(t, "abc123", "codemap-rows/1", "rta", true,
		[]map[string]any{{"symbol": "Ghost", "file": "internal/wallet/pay.go", "line": 3}})
	pl, err := BuildPlan(m, "abc123", repo, "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Skeleton(pl)
	if err != nil {
		t.Fatal(err)
	}
	var d map[string]any
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatalf("the skeleton must be valid JSON: %v", err)
	}
	if d["sha"] != pl.SHA {
		t.Errorf("sha not bound: %v", d["sha"])
	}
	if len(d["read_paths"].([]any)) != 0 {
		t.Error("read_paths must start empty")
	}
	if got := len(d["_unread"].([]any)); got != pl.ProductionTotal {
		t.Errorf("_unread = %d, want every production file (%d)", got, pl.ProductionTotal)
	}
	if len(d["_candidates_undispositioned"].([]any)) != len(pl.Candidates) {
		t.Error("every candidate must be listed for disposition")
	}

	// An unedited skeleton must enumerate as INCOMPLETE.
	res, code, err := Enumerate(writeJSON(t, pl), writeJSONRaw(t, b))
	if err != nil {
		t.Fatal(err)
	}
	if code != ExitItemsOpen || res.Accounting != "incomplete" {
		t.Fatalf("an unedited skeleton must be incomplete: code=%d accounting=%s", code, res.Accounting)
	}
}

// The scaffolding fields must not be mistaken for claims.
func TestSkeletonScaffoldingIsIgnoredByEnumerate(t *testing.T) {
	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n"})
	pl := planFor(t, repo)
	b, err := Skeleton(pl)
	if err != nil {
		t.Fatal(err)
	}
	var d map[string]any
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	// Fill it in as an agent would.
	d["read_paths"] = pl.ProductionFiles
	nb, err := json.MarshalIndent(d, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	res, code, err := Enumerate(writeJSON(t, pl), writeJSONRaw(t, nb))
	if err != nil {
		t.Fatal(err)
	}
	if code != ExitOK {
		t.Fatalf("a filled skeleton must complete: code=%d remaining=%v", code, res.Remaining)
	}
	if !strings.HasPrefix(res.Attested.Repo, "1/") {
		t.Errorf("attested.repo = %s", res.Attested.Repo)
	}
}

// The unseeded families are a FACT the plan already states; making the agent retype them only
// creates a way to retype them wrong.
func TestSkeletonPrefillsWhatThePlanAlreadyKnows(t *testing.T) {
	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n"})
	pl := planFor(t, repo)
	b, err := Skeleton(pl)
	if err != nil {
		t.Fatal(err)
	}
	var d map[string]any
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	got := d["families_not_run"].([]any)
	if len(got) != len(pl.UnseededFamilies) {
		t.Errorf("families_not_run = %v, want the plan's unseeded set %v", got, pl.UnseededFamilies)
	}
}

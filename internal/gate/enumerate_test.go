package gate

import (
	"strings"
	"testing"
)

// THE COMPLEMENT CLAUSE IS THE ENFORCEMENT HALF OF THIS TOOL'S FOUNDING DEFECT, and until now
// nothing tested it.
//
// ghola @4f33b3c: `internal/bridge/bridge.go` — an unauthenticated localhost HTTP server making
// arbitrary outbound fetches — matched no H signal and had not changed since the baseline, so it
// appeared in no worklist, and the gate certified the sweep COMPLETE without it. A hand read later
// turned it into the sweep's worst finding. `plan` raising it in `h_unmatched` is only half the
// repair; `Enumerate` HOLDING THE SWEEP OPEN over it is the half that does the work.
//
// Review III proved the second half was unguarded: replacing `range pl.HUnmatched` with an empty
// range left the entire suite green, and a sweep with unread signal-unmatched production files
// settled `complete` at exit 0 — then wrote a durable record that SKILL.md Step 0.2 tells the next
// sweep not to re-spend budget on. The one existing test asserted that BuildPlan POPULATES
// HUnmatched, which is the input to this clause rather than the clause itself.
func TestUnreadComplementFilesHoldTheSweepOpen(t *testing.T) {
	repo := gitRepo(t, map[string]string{
		"internal/wallet/pay.go":  "package wallet\n", // matches a money signal
		"internal/misc/notes.go":  "package misc\n",   // matches nothing
		"internal/misc/notes2.go": "package misc\n",   // matches nothing
	})
	p := planFor(t, repo)
	if len(p.HUnmatched) == 0 {
		t.Fatal("fixture produced no unmatched files, so this test cannot measure the clause")
	}

	// Read ONLY the signal-matched path, leaving the complement neither read nor waived — exactly
	// ghola's shape.
	read := make([]string, 0, len(p.HWorklist))
	for _, w := range p.HWorklist {
		read = append(read, w.Path)
	}
	res, code, err := Enumerate(writeJSON(t, p), writeJSON(t, map[string]any{
		"sha": p.SHA, "read_paths": read, "families_not_run": p.UnseededFamilies}))
	if err != nil {
		t.Fatal(err)
	}
	if code != ExitItemsOpen || res.Accounting == "complete" {
		t.Fatalf("a sweep leaving %d signal-unmatched production files unread must NOT settle: "+
			"code=%d accounting=%q. A signal miss is not a clearance",
			len(p.HUnmatched), code, res.Accounting)
	}
	named := false
	for _, r := range res.Remaining {
		if strings.Contains(r, "no H signal reached") && strings.Contains(r, "unread") {
			named = true
		}
	}
	if !named {
		t.Errorf("the open item must NAME the complement, or a reader cannot act on it: %v",
			res.Remaining)
	}

	// The counterfactual, which is what makes the assertion above about the CLAUSE rather than
	// about the sweep being unfinished for some other reason: dispositioning the same files
	// settles it. Waived still counts as unread in attested.repo — the point is that somebody
	// decided, not that the files were never raised.
	waived := make([]map[string]string, 0, len(p.HUnmatched))
	for _, u := range p.HUnmatched {
		waived = append(waived, map[string]string{"path": u, "reason": "not in scope"})
	}
	res2, code2, err := Enumerate(writeJSON(t, p), writeJSON(t, map[string]any{
		"sha": p.SHA, "read_paths": read, "coverage_waived": waived,
		"families_not_run": p.UnseededFamilies}))
	if err != nil {
		t.Fatal(err)
	}
	if code2 != ExitOK || res2.Accounting != "complete" {
		t.Errorf("dispositioning every complement file must settle the sweep: code=%d %q remaining=%v",
			code2, res2.Accounting, res2.Remaining)
	}
}

// `0/0` MUST NOT SETTLE. ABORT I condition 4 required this verbatim and it was never implemented:
// a plan with an empty `production_files` list but one worklist entry reported `attested.repo`
// "0/0", `accounting: complete`, exit 0, and wrote a record. Nothing was measured, and "nothing was
// measured" reading as "clean" is the single failure this tool exists to prevent.
// The FIRST version of this test passed immediately, and passed for the wrong reason: it left
// `read_paths` empty, so the sweep was held open by the unread WORKLIST and never exercised the
// denominator at all. Removing that confound showed the truth — `code=0 accounting="complete"
// attested="0/0" remaining=[]`. Every path here is now dispositioned, so the ONLY thing left that
// can refuse is the zero denominator.
func TestASweepOverNoProductionFilesCannotSettle(t *testing.T) {
	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package wallet\n"})
	p := planFor(t, repo)
	p.ProductionFiles = nil
	p.ProductionTotal = 0

	// Disposition everything else, so nothing but the denominator can hold this open.
	read := make([]string, 0, len(p.HWorklist))
	for _, w := range p.HWorklist {
		read = append(read, w.Path)
	}
	res, code, err := Enumerate(writeJSON(t, p), writeJSON(t, map[string]any{
		"sha": p.SHA, "read_paths": read, "families_not_run": p.UnseededFamilies}))
	if err == nil {
		t.Fatalf("a denominator of zero settled: code=%d accounting=%q attested=%q remaining=%v. "+
			"There is no repository this describes — it is an instrument reading of nothing",
			code, res.Accounting, res.Attested.Repo, res.Remaining)
	}
	if code != ExitRefused {
		t.Errorf("exit = %d, want %d (refused): nothing was measured, which is not the same as "+
			"items being open", code, ExitRefused)
	}
}

// The plan contract is MANDATORY. It was checked as `pl.Contract != "" && ...`, so a plan that
// omitted the field entirely was accepted — and omitting a field is exactly what a hand-written
// plan does. `report` and `enumerate` now derive every published figure from the plan and the
// discharge, which makes an unvalidated plan the whole attack surface rather than a detail.
func TestAPlanWithNoContractIsRefused(t *testing.T) {
	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package wallet\n"})
	p := planFor(t, repo)
	for _, contract := range []string{"", "slop-gate/1", "anything"} {
		p.Contract = contract
		_, code, err := Enumerate(writeJSON(t, p), writeJSON(t, map[string]any{
			"sha": p.SHA, "read_paths": p.ProductionFiles, "families_not_run": p.UnseededFamilies}))
		if err == nil || code != ExitRefused {
			t.Errorf("contract %q was accepted (code=%d): a plan must come from `ferret plan`",
				contract, code)
		}
	}
}

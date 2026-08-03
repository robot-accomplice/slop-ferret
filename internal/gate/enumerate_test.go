package gate

import (
	"fmt"
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

// pct FLOORS. Rounding let 1999/2000 render as 100.0% — a partial read shown as complete, in the
// one number a reader scans first. That defect is named in the function's own comment and in the
// CHANGELOG as fixed, and mechanical mutation showed nothing tested it: floor could be swapped back
// to round, and every arithmetic operator in the expression could be changed, with the suite green.
func TestPctFloorsAndNeverReadsAsCompleteWhenItIsNot(t *testing.T) {
	for _, c := range []struct {
		done, total int
		want        float64
	}{
		{1999, 2000, 99.9}, // the defect: rounding renders this 100.0
		{2000, 2000, 100},
		{1, 3, 33.3},
		{2, 3, 66.6}, // floors: rounding gives 66.7
		{1, 1000, 0.1},
		{0, 25, 0},
		{17, 25, 68},
	} {
		got := pct(c.done, c.total)
		if got == nil {
			t.Fatalf("pct(%d,%d) = nil", c.done, c.total)
		}
		if *got != c.want {
			t.Errorf("pct(%d,%d) = %v, want %v", c.done, c.total, *got, c.want)
		}
	}
	// A partial read must NEVER produce 100.
	for _, c := range [][2]int{{1999, 2000}, {9999, 10000}, {24, 25}} {
		if v := pct(c[0], c[1]); v != nil && *v >= 100 {
			t.Errorf("pct(%d,%d) = %v — a partial read rendered as complete is the whole defect",
				c[0], c[1], *v)
		}
	}
	// A zero denominator has no percentage. nil is not 0%: "nothing to measure" and "measured
	// nothing" are different claims, and the caller suppresses the figure rather than printing 0.
	if pct(0, 0) != nil {
		t.Error("pct(0,0) must be nil, not 0 — there is no fraction to report")
	}
}

// `attested.plan` is one of the tool's two headline fractions and its denominator is a sum of three
// lists. Mutation showed every operator in both expressions could be flipped with the suite green,
// so the published fraction was unpinned in both numerator and denominator.
func TestTheAttestedPlanFractionCountsEveryRaisedItemAndEveryOpenOne(t *testing.T) {
	repo := gitRepo(t, map[string]string{
		"internal/wallet/pay.go":  "package w\n", // tier-1 signal -> required
		"internal/client/get.go":  "package c\n", // tier-2 signal
		"internal/misc/notes.go":  "package m\n", // no signal -> complement
		"internal/misc/notes2.go": "package m\n", // no signal -> complement
		"internal/misc/notes3.go": "package m\n", // no signal -> complement
	})
	p := planFor(t, repo)
	raised := len(p.HRequired) + len(p.HDeferred) + len(p.HUnmatched)
	if raised < 4 {
		t.Fatalf("fixture raised only %d items; this test cannot measure the fraction", raised)
	}

	// Disposition exactly one item and nothing else.
	one := p.HUnmatched[0]
	res, _, err := Enumerate(writeJSON(t, p), writeJSON(t, map[string]any{
		"sha": p.SHA, "read_paths": []string{one}, "families_not_run": p.UnseededFamilies}))
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("1/%d", raised)
	if res.Attested.Plan != want {
		t.Errorf("attested.plan = %q, want %q. The denominator is every item the plan RAISED "+
			"(required + deferred + complement) and the numerator is those dispositioned; a plan "+
			"fraction that omits a list reports a narrower sweep as a fuller one",
			res.Attested.Plan, want)
	}

	// And all of them.
	all := append([]string{}, p.ProductionFiles...)
	res2, _, err := Enumerate(writeJSON(t, p), writeJSON(t, map[string]any{
		"sha": p.SHA, "read_paths": all, "families_not_run": p.UnseededFamilies}))
	if err != nil {
		t.Fatal(err)
	}
	if res2.Attested.Plan != fmt.Sprintf("%d/%d", raised, raised) {
		t.Errorf("attested.plan = %q, want everything dispositioned", res2.Attested.Plan)
	}
}

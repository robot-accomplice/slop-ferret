package gate

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

type Discharge struct {
	SHA             string            `json:"sha"`
	ReadPaths       []string          `json:"read_paths"`
	CoverageWaived  []json.RawMessage `json:"coverage_waived"`
	FamiliesNotRun  []string          `json:"families_not_run"`
	CandidatesFiled []struct {
		File   string `json:"file"`
		Symbol string `json:"symbol"`
	} `json:"candidates_filed"`
	CandidatesCleared []struct {
		File   string `json:"file"`
		Symbol string `json:"symbol"`
	} `json:"candidates_cleared"`
	CandidatesRefuted []struct {
		File   string `json:"file"`
		Symbol string `json:"symbol"`
	} `json:"candidates_refuted"`

	// The ATTESTED half of a record (§6). All optional: a sweep that supplies none of it still
	// verifies, it just records less for the next one to lean on.
	Tier              string         `json:"tier"`
	CheckedClean      []CheckedClean `json:"checked_clean"`
	NearMisses        []string       `json:"near_misses"`
	FindingsVerified  int            `json:"findings_verified"`
	FindingsSuspected int            `json:"findings_suspected"`
	ReportPath        string         `json:"report_path"`
}

// Attested is what the AUDITOR STATED, counted. It is not a measurement and this tool does not
// observe reading: `read_paths` comes from the sweeping agent's own discharge. The field names say
// "attested" rather than "coverage" for that reason — the previous naming read as verification of
// the audit, which is a claim the arithmetic cannot support and which five independent reviewers
// called out as the tool's central overclaim.
//
// This is an audit and reporting tool. It looks at the thing, evaluates what it can, and reports.
// What the auditor did is the auditor's statement, reported as such.
type Attested struct {
	Repo         string   `json:"repo"`
	RepoPct      *float64 `json:"repo_pct"`
	RepoNote     string   `json:"repo_note"`
	Plan         string   `json:"plan"`
	PlanPct      *float64 `json:"plan_pct"`
	PlanNote     string   `json:"plan_note"`
	Waived       int      `json:"waived"`
	Unclassified int      `json:"unclassified"`
}

type Result struct {
	PlanSHA                string     `json:"plan_sha"`
	Attested               Attested   `json:"attested"`
	HWorklistTotal         int        `json:"h_worklist_total"`
	HRequiredTotal         int        `json:"h_required_total"`
	HPathsAttested         int        `json:"h_paths_attested"`
	HRequiredUnattested    []WorkItem `json:"h_required_unattested"`
	HDeferredUnattested    int        `json:"h_deferred_unattested"`
	ChangeBaseline         string     `json:"change_baseline"`
	UnmatchedChangesTotal  int        `json:"unmatched_changes_total"`
	UnmatchedChangesOpen   []string   `json:"unmatched_changes_open"`
	UnreadUnmatched        []string   `json:"unread_unmatched"`
	UnreadUnmatchedTotal   int        `json:"unread_unmatched_total"`
	CandidatesTotal        int        `json:"candidates_total"`
	CandidatesCleared      int        `json:"candidates_cleared"`
	CandidatesRefuted      int        `json:"candidates_refuted"`
	CandidatesFiled        int        `json:"candidates_filed"`
	FiledWithoutBar        []string   `json:"filed_without_bar"`
	CandidatesUnaccounted  []string   `json:"candidates_unaccounted"`
	UnseededFamilies       []string   `json:"unseeded_families"`
	FamiliesDeclaredNotRun []string   `json:"families_declared_not_run"`
	Remaining              []string   `json:"remaining"`
	Accounting             string     `json:"accounting"`
	Headline               string     `json:"headline"`
}

type key struct{ file, symbol string }

// pct FLOORS rather than rounds. Rounding let 1999/2000 render as 100.0%, so a partial read as
// complete — in the one number a reader scans first.
func pct(done, total int) *float64 {
	if total == 0 {
		return nil
	}
	v := float64(int(1000*float64(done)/float64(total))) / 10
	return &v
}

// Verify reports two fractions and a work queue. Every clause below was absent at some point and
// is here because its absence was reproduced, not imagined.
// LoadSweep parses a plan and a discharge and enumerates them, returning all three. It exists so
// that `report` derives its coverage figures from THE SAME parse and THE SAME arithmetic that
// `enumerate` runs, rather than from numbers an auditor typed into a findings file.
//
// The renderer used to take every figure as model-supplied JSON while a comment claimed they
// "come from enumerate so the report cannot disagree" — and the field names did not even match, so
// the seam had never carried a byte. Routing both commands through one function is what makes that
// claim structurally true instead of aspirational: there is no longer a parameter to disagree with.
func LoadSweep(planPath, dischargePath string) (*Plan, *Discharge, *Result, int, error) {
	pl, dis, err := loadPlanAndDischarge(planPath, dischargePath)
	if err != nil {
		return nil, nil, nil, ExitMisuse, err
	}
	res, code, err := Enumerate(planPath, dischargePath)
	return pl, dis, res, code, err
}

func loadPlanAndDischarge(planPath, dischargePath string) (*Plan, *Discharge, error) {
	pb, err := os.ReadFile(planPath)
	if err != nil {
		return nil, nil, die(ExitMisuse, "reading plan: %v", err)
	}
	db, err := os.ReadFile(dischargePath)
	if err != nil {
		return nil, nil, die(ExitMisuse, "reading discharge: %v", err)
	}
	var pl Plan
	if err := json.Unmarshal(pb, &pl); err != nil {
		return nil, nil, die(ExitMisuse, "plan is not valid JSON: %v", err)
	}
	var dis Discharge
	if err := json.Unmarshal(db, &dis); err != nil {
		return nil, nil, die(ExitMisuse, "discharge is not valid JSON: %v", err)
	}
	return &pl, &dis, nil
}

// VerifyAndRecord runs Verify and, unless suppressed, persists a record.
//
// Always-write with an opt-out, deliberately: a record you must remember to request is one that
// will not exist when the next sweep looks for it, and the whole point of the store is that Step
// 0.2 finds something.
func EnumerateAndRecord(planPath, dischargePath, repo string, record bool) (*Result, string, int, error) {
	res, code, err := Enumerate(planPath, dischargePath)
	if err != nil || !record || repo == "" {
		return res, "", code, err
	}
	// An unfinished sweep simply does not produce a record. That is the normal case, not a failure:
	// records exist to be trusted by a LATER sweep, and only a settled one has established anything.
	// Returning an error here would make an ordinary in-progress sweep look broken.
	if res.Accounting != "complete" {
		return res, "", code, nil
	}
	pl, dis, lerr := loadPlanAndDischarge(planPath, dischargePath)
	if lerr != nil {
		return res, "", code, nil
	}
	path, werr := WriteRecord(repo, pl, dis, res)
	if werr != nil {
		// A record that cannot be written must not silently vanish, but it also must not discard a
		// verify result the operator already earned. Surface it and keep the result.
		return res, "", code, fmt.Errorf("record: %w", werr)
	}
	return res, path, code, nil
}

func Enumerate(planPath, dischargePath string) (*Result, int, error) {
	plp, disp, err := loadPlanAndDischarge(planPath, dischargePath)
	if err != nil {
		return nil, ExitMisuse, err
	}
	pl, dis := *plp, *disp

	// The plan's own contract was written at BuildPlan and read nowhere, while loadMap refuses an
	// unknown MAP contract with exit 4. The asymmetry meant any future field rename would degrade
	// to zeros and SETTLE rather than refuse — guarding the input we did not design and not the one
	// we did.
	// The contract is MANDATORY, not opt-in. It used to read `pl.Contract != "" && ...`, so a plan
	// that simply omitted the field was accepted — which is the shape a hand-written plan takes,
	// and hand-written plans are how a sweep that never ran produces a settled record.
	if pl.Contract != planContract {
		return nil, ExitRefused, die(ExitRefused, "plan contract %q is not %q — a plan must come "+
			"from `ferret plan`, which stamps this. An absent contract used to be accepted, which "+
			"made a hand-written plan indistinguishable from a real one", pl.Contract, planContract)
	}

	// A DENOMINATOR OF ZERO IS NOT A CLEAN SWEEP. ABORT I condition 4 required this verbatim
	// ("`0/0` cannot settle") and it was never implemented: a plan with an empty `production_files`
	// reported attested.repo "0/0", accounting complete, exit 0, empty remaining, and wrote a
	// durable record. There is no repository that describes — it is an instrument reading of
	// nothing, and this tool exists to stop nothing from reading as clean.
	if pl.ProductionTotal == 0 || len(pl.ProductionFiles) == 0 {
		return nil, ExitRefused, die(ExitRefused, "the plan names no production files, so there is "+
			"no denominator and nothing to be complete ABOUT. `0/0` is not coverage. If the "+
			"repository really has no source files this tool has nothing to say about it; if it "+
			"has some, the plan is wrong — re-run `ferret plan`")
	}

	var remaining []string

	// 1. The discharge must belong to THIS plan. verify once referenced neither sha nor contract,
	//    so a discharge from any other sweep satisfied any plan — and stale artifacts demonstrably
	//    survive across sessions.
	switch {
	case dis.SHA == "":
		remaining = append(remaining, fmt.Sprintf("discharge has no `sha`; it cannot be shown to "+
			"belong to this plan (plan sha %q)", pl.SHA))
	case dis.SHA != pl.SHA:
		remaining = append(remaining, fmt.Sprintf("discharge sha %q != plan sha %q: this discharge "+
			"belongs to a different sweep", dis.SHA, pl.SHA))
	}

	// 2. A worklist that enumerated NOTHING cannot certify anything. H enumeration is
	//    vocabulary-bound and under-enumerates silently on an unfamiliar domain, so an empty
	//    worklist is the one case where "nothing to read" must never read as "everything was read".
	if len(pl.HWorklist) == 0 {
		remaining = append(remaining, "h_worklist is EMPTY: the plan enumerated no family-H path, "+
			"so this run proves nothing. Extend the signals via `.slop-h-signals` and re-plan; do "+
			"not accept a zero worklist as coverage")
	}

	read := map[string]bool{}
	for _, p := range dis.ReadPaths {
		read[p] = true
	}
	// A waiver may be a bare path or {path, reason}; the reason is optional by policy. Waiving is
	// deliberately cheap — what it buys is the ACCOUNTING, never the coverage number.
	waived := map[string]bool{}
	for _, raw := range dis.CoverageWaived {
		var s string
		if json.Unmarshal(raw, &s) == nil && s != "" {
			waived[s] = true
			continue
		}
		var o struct {
			Path string `json:"path"`
		}
		if json.Unmarshal(raw, &o) == nil && o.Path != "" {
			waived[o.Path] = true
		}
	}

	required := pl.HRequired
	if len(required) == 0 && len(pl.HWorklist) > 0 && pl.HRequired == nil {
		required = pl.HWorklist // a plan written before the split: everything in it is required
	}
	var unread []WorkItem
	for _, w := range required {
		if !read[w.Path] {
			unread = append(unread, w)
		}
	}
	deferredUnattested := 0
	for _, w := range pl.HDeferred {
		if !read[w.Path] {
			deferredUnattested++
		}
	}

	// A changed file no signal reached is the enumeration's own blind spot, so it cannot be
	// deferred on a consequence argument — nobody assessed its consequence.
	var holes []string
	for _, u := range pl.HUnmatchedChanges {
		if !read[u.Path] && !waived[u.Path] {
			holes = append(holes, u.Path)
		}
	}
	if len(holes) > 0 {
		remaining = append(remaining, fmt.Sprintf("%d changed file(s) that no H signal reached went "+
			"neither attested nor waived. The enumeration never saw them, so their absence from the "+
			"worklist says nothing about them. Read each, or list it in `coverage_waived`", len(holes)))
	}

	// THE COMPLEMENT CLAUSE. Same argument, minus the baseline restriction that made it a subset.
	// Before this, matching no signal meant a file was never raised at all, so the gate could not
	// tell "cleared" from "never existed". ghola's internal/bridge/bridge.go sat in exactly that
	// gap and the run still certified COMPLETE.
	var undispositioned []string
	for _, p := range pl.HUnmatched {
		if !read[p] && !waived[p] {
			undispositioned = append(undispositioned, p)
		}
	}
	if len(undispositioned) > 0 {
		remaining = append(remaining, fmt.Sprintf("%d production file(s) no H signal reached are "+
			"still unread. A signal miss is not a clearance — nothing has looked at them yet, so "+
			"they are the natural next place to spend time. Read what is worth reading and waive "+
			"the rest in `coverage_waived` (a reason is optional; waived counts as unread in "+
			"`attested.repo`, which is the point)", len(undispositioned)))
	}
	if len(unread) > 0 {
		remaining = append(remaining, fmt.Sprintf("%d REQUIRED family-H path(s) unattested: the "+
			"sweep did not look at a blast-radius path", len(unread)))
	}

	// A family the map could not seed must be ACKNOWLEDGED as not-run, not merely printed at.
	// Prose is not a gate; that is the same defect this function was once repaired for.
	declared := map[string]bool{}
	for _, f := range dis.FamiliesNotRun {
		declared[f] = true
	}
	var unack []string
	for _, f := range pl.UnseededFamilies {
		if !declared[f] {
			unack = append(unack, f)
		}
	}
	sort.Strings(unack)
	if len(unack) > 0 {
		remaining = append(remaining, fmt.Sprintf("families %v had no map seed and the discharge "+
			"does not list them in `families_not_run`. They were not run; say so, or run them by "+
			"hand and drop them from the plan's unseeded set. They may not read as checked-clean", unack))
	}

	// 3. Every candidate the sweep FILED must have cleared its bar. This is the filed set, not
	//    every candidate: most candidates are correctly discarded (21 of 23 dead rows in one real
	//    map were test mocks), so requiring all of them would make every sweep open and the tool
	//    would be ignored.
	cleared := map[key]bool{}
	for _, c := range dis.CandidatesCleared {
		cleared[key{c.File, c.Symbol}] = true
	}
	var filedUnbarred []string
	for _, c := range dis.CandidatesFiled {
		if !cleared[key{c.File, c.Symbol}] {
			filedUnbarred = append(filedUnbarred, c.Symbol)
		}
	}
	if len(filedUnbarred) > 0 {
		remaining = append(remaining, fmt.Sprintf("%d FILED candidate(s) did not clear their bar: "+
			"an accusation was made without the evidence its class requires", len(filedUnbarred)))
	}

	// 4. Every candidate the plan raised must be ACCOUNTED FOR — cleared or explicitly refuted.
	//    Clause 3 only bites when the sweep FILES something, so the clean-sweep path (file
	//    nothing, clear nothing, attest the reads) was once certified COMPLETE with every
	//    candidate unexamined. What stops being free is discarding a candidate SILENTLY.
	refuted := map[key]bool{}
	for _, c := range dis.CandidatesRefuted {
		refuted[key{c.File, c.Symbol}] = true
	}
	var unaccounted []string
	for _, c := range pl.Candidates {
		k := key{c.File, c.Symbol}
		if !cleared[k] && !refuted[k] {
			unaccounted = append(unaccounted, c.Class+" "+c.Symbol)
		}
	}
	if len(unaccounted) > 0 {
		remaining = append(remaining, fmt.Sprintf("%d candidate(s) were neither cleared nor "+
			"refuted: the plan raised them and the sweep never says what became of them. Clear "+
			"each one's bar, or list it in `candidates_refuted` to record that you looked and "+
			"discarded it. A sweep that files nothing is not clean by default", len(unaccounted)))
	}

	// TWO FRACTIONS, NO VERDICT WORD. This is an INSTRUMENT READING, not a score. It exists so the
	// person doing the sweep can see where they actually are — nobody is being graded, and there
	// is no adversary to design against. Waived files count as UNREAD for that reason and no
	// other: choosing not to read a file is a normal, correct move, and a fraction that quietly
	// counted it as covered would be lying to the only person who reads it.
	readProd := 0
	for _, p := range pl.ProductionFiles {
		if read[p] {
			readProd++
		}
	}
	enumItems := len(required) + len(pl.HDeferred) + len(pl.HUnmatched)
	enumOpen := len(unread) + deferredUnattested + len(undispositioned)

	if undispositioned == nil {
		undispositioned = []string{}
	}
	shown := undispositioned
	if len(shown) > 50 {
		shown = shown[:50]
	}

	res := &Result{
		PlanSHA: pl.SHA,
		Attested: Attested{
			Repo:    fmt.Sprintf("%d/%d", readProd, len(pl.ProductionFiles)),
			RepoPct: pct(readProd, len(pl.ProductionFiles)),
			RepoNote: "files the auditor STATES they read, over production source files found. " +
				"This tool does not observe reading — it reports the auditor's own statement. " +
				"Waived files count as unread.",
			Plan:    fmt.Sprintf("%d/%d", enumItems-enumOpen, enumItems),
			PlanPct: pct(enumItems-enumOpen, enumItems),
			PlanNote: "items the plan raised for which the discharge states a disposition. High " +
				"here and low in `repo` means the enumeration was narrow, not that the repo is clean.",
			Waived:       len(waived),
			Unclassified: len(pl.ProductionUnclassified),
		},
		HWorklistTotal: len(pl.HWorklist), HRequiredTotal: len(required),
		HPathsAttested:      len(pl.HWorklist) - len(unread) - deferredUnattested,
		HRequiredUnattested: nonNilW(unread), HDeferredUnattested: deferredUnattested,
		ChangeBaseline: pl.ChangeBaseline, UnmatchedChangesTotal: len(pl.HUnmatchedChanges),
		UnmatchedChangesOpen: nonNil(holes), UnreadUnmatched: shown,
		UnreadUnmatchedTotal: len(undispositioned),
		CandidatesTotal:      len(pl.Candidates), CandidatesCleared: len(cleared),
		CandidatesRefuted: len(refuted), CandidatesFiled: len(dis.CandidatesFiled),
		FiledWithoutBar: nonNil(filedUnbarred), CandidatesUnaccounted: nonNil(unaccounted),
		UnseededFamilies:       nonNil(pl.UnseededFamilies),
		FamiliesDeclaredNotRun: nonNil(sortedKeys(declared)),
		Remaining:              nonNil(remaining),
	}

	// ONE BINARY MACHINE SIGNAL, about bookkeeping only — which is all an exit code can carry
	// honestly. It means "there are still items on the list", the way a test runner means "there
	// are still failures": useful to a script, not a judgement about the person running it.
	// The accounting is complete or it is not. This says nothing about whether the audit was good
	// — only whether every item the plan raised has a stated disposition.
	res.Accounting = "complete"
	code := ExitOK
	if len(remaining) > 0 {
		res.Accounting = "incomplete"
		code = ExitItemsOpen
	}
	pctStr := ""
	if res.Attested.RepoPct != nil {
		pctStr = fmt.Sprintf(" (%.1f%%)", *res.Attested.RepoPct)
	}
	res.Headline = fmt.Sprintf("auditor states %s source files read%s · %s of the plan dispositioned",
		res.Attested.Repo, pctStr, res.Attested.Plan)
	if len(waived) > 0 {
		res.Headline += fmt.Sprintf(" · %d waived (count as unread)", len(waived))
	}
	if len(remaining) > 0 {
		res.Headline += fmt.Sprintf(" · %d item(s) still open", len(remaining))
	} else {
		res.Headline += " · nothing left open"
	}
	return res, code, nil
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

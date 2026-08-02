// Package report renders a sweep report to a self-contained HTML file.
//
// WHY THIS IS IN THE BINARY. Layout, ordering, table construction, denominators and HTML escaping
// are deterministic transforms. Having a model do them costs tokens, varies between runs, and
// produced two defects in a single hand-written report on 2026-08-01 — a malformed `</strong,` tag
// and a junk CSS value — caught only because the author happened to grep their own output.
//
// What stays with the model is the half only it can do: which findings exist, how severe they are,
// what refuted the near-misses, and the prose. Those arrive as JSON. This file turns them into a
// page, the same way every time.
//
// Ordering is SEVERITY-FIRST and never volume-first: count runs inverse to severity — the largest
// class in the first campaign was 7,022 occurrences and cosmetic, while the blocking ones sat at 33
// and 8. A page ordered by count argues for exactly the wrong priority.
package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"sort"
	"strings"

	"github.com/robot-accomplice/slop-ferret/internal/gate"
)

// Finding is the judgement half: supplied by whoever ran the sweep, never inferred here.
type Finding struct {
	Title       string `json:"title"`
	File        string `json:"file"`
	Class       string `json:"class"`
	Severity    string `json:"severity"`   // blocking | fix-or-file | note
	Status      string `json:"status"`     // VERIFIED | SUSPECTED
	Claim       string `json:"claim"`      // the falsifiable sentence
	Refutation  string `json:"refutation"` // what would disprove it, and where you looked
	Bar         string `json:"bar"`        // the pre-filing bar cleared
	Evidence    string `json:"evidence"`
	Remediation string `json:"remediation"`
	Occurrences int    `json:"occurrences"`
}

// Input is everything the page needs. IT IS NOT A FILE FORMAT — build it with FromSweep, which
// fills every coverage figure from the plan and the enumeration. The findings half comes from the
// auditor; the arithmetic half cannot.
//
// The previous version took ALL of this as one model-written JSON file while a comment here
// claimed the figures "come from `enumerate` so the report cannot disagree with the tool that
// produced them". That was false twice: `report` read one file and never opened the plan, the
// discharge or the enumerate result; and the field names never matched — `enumerate` emits a
// nested `attested: {repo, plan}` against this struct's flat `attested_repo`. The seam had never
// carried a byte, which no test could notice because every renderer test built this struct in Go
// and none exercised the JSON path at all.
type Input struct {
	Repo         string    `json:"repo"`
	SHA          string    `json:"sha"`
	SkillVersion string    `json:"skill_version"`
	LexiconVer   string    `json:"lexicon_version"`
	Tier         string    `json:"tier"`
	AttestedRepo string    `json:"attested_repo"`
	AttestedPlan string    `json:"attested_plan"`
	Waived       int       `json:"waived"`
	Accounting   string    `json:"accounting"`
	Remaining    []string  `json:"remaining"`
	Denominator  int       `json:"denominator"`
	FamiliesRun  []string  `json:"families_run"`
	FamiliesNot  []string  `json:"families_not_run"`
	Findings     []Finding `json:"findings"`
	NearMisses   []string  `json:"near_misses"`
	CheckedClean []struct {
		Class  string `json:"class"`
		Method string `json:"method"`
	} `json:"checked_clean"`
	MapLimitations []string `json:"map_limitations"`
}

var sevRank = map[string]int{"blocking": 0, "fix-or-file": 1, "note": 2}

// Render writes the page. It never invents a number: every figure comes from Input.
func Render(w io.Writer, in Input) error {
	f := append([]Finding(nil), in.Findings...)
	// Severity first, then VERIFIED before SUSPECTED, then title — deterministic, so two runs over
	// the same input produce byte-identical pages.
	sort.SliceStable(f, func(i, j int) bool {
		ri, rj := sevRank[strings.ToLower(f[i].Severity)], sevRank[strings.ToLower(f[j].Severity)]
		if ri != rj {
			return ri < rj
		}
		if f[i].Status != f[j].Status {
			return f[i].Status == "VERIFIED"
		}
		return f[i].Title < f[j].Title
	})
	in.Findings = f

	verified, suspected := 0, 0
	for _, x := range f {
		if x.Status == "VERIFIED" {
			verified++
		} else {
			suspected++
		}
	}

	// THE RATE IS SUPPRESSED BELOW ~100 NON-TEST SOURCE FILES, and the denominator is published
	// either way. One finding moves a small rate by 13-50 points; a number is harder to retract
	// than a blank.
	rate := "n/a"
	if in.Denominator >= 100 && verified > 0 {
		rate = fmt.Sprintf("%.1f per 1,000", float64(verified)*1000/float64(in.Denominator))
	} else if in.Denominator < 100 {
		rate = fmt.Sprintf("n/a (denominator %d < 100)", in.Denominator)
	}

	// A sweep with SUSPECTED findings and zero VERIFIED has verified nothing. Say so on the face of
	// the page rather than letting a blank rate read as a clean result.
	tell := ""
	if verified == 0 && suspected > 0 {
		tell = fmt.Sprintf("%d lead(s), none verified. This is not a clean result — nothing here "+
			"cleared its pre-filing bar.", suspected)
	}

	return page.Execute(w, map[string]any{
		"In": in, "Verified": verified, "Suspected": suspected, "Rate": rate, "Tell": tell,
	})
}

// ParseInput reads the judgement half from JSON.
// Authored is the ONLY part of the page an auditor writes: the findings and the provenance labels.
// Every coverage figure is absent from it by design — there is no field here to type a fraction
// into, which is what stops a hand-written file from rendering a page that reads as a completed
// sweep.
type Authored struct {
	Repo         string    `json:"repo"`
	SkillVersion string    `json:"skill_version"`
	LexiconVer   string    `json:"lexicon_version"`
	FamiliesRun  []string  `json:"families_run"`
	Findings     []Finding `json:"findings"`
}

// ParseAuthored reads the auditor's half. Unknown fields are REFUSED rather than ignored: the
// retired single-file format carried `attested_repo`, `accounting` and `denominator`, and silently
// dropping them would let an old input render a page whose figures came from somewhere else
// entirely while looking like it had been accepted.
func ParseAuthored(b []byte) (Authored, error) {
	var a Authored
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&a); err != nil {
		return a, fmt.Errorf("%w\n\nCoverage figures are no longer supplied here — they are "+
			"derived from the plan and the enumeration. This file carries the findings and the "+
			"provenance labels only: {repo, skill_version, lexicon_version, families_run, findings}",
			err)
	}
	// SEVERITY AND STATUS ARE ENUMS, and both used to fail open toward "looks fine": an
	// unrecognised severity got rank 0 from a bare map lookup — the same rank as `blocking` — so a
	// trailing-whitespace finding labelled "catastrophic" sorted ABOVE an auth bypass, while
	// sevclass defaulted it to the green `note` chip. An unknown status rendered a card with
	// neither the VERIFIED border nor the SUSPECTED hatching while being counted as suspected.
	// Refusing is the only option that cannot mislabel: there is no safe default for "I do not
	// know how bad this is".
	for i, f := range a.Findings {
		if _, ok := sevRank[strings.ToLower(f.Severity)]; !ok {
			return a, fmt.Errorf("findings[%d] (%q): severity %q is not one of %s. It is an enum, "+
				"not free text — an unrecognised value used to sort as the most severe while "+
				"rendering as the least", i, f.Title, f.Severity, strings.Join(severities(), ", "))
		}
		if f.Status != "VERIFIED" && f.Status != "SUSPECTED" {
			return a, fmt.Errorf("findings[%d] (%q): status %q is not VERIFIED or SUSPECTED. The "+
				"page distinguishes the two visually rather than by caption, and anything else "+
				"renders as neither while still being counted", i, f.Title, f.Status)
		}
	}
	return a, nil
}

func severities() []string {
	out := make([]string, 0, len(sevRank))
	for s := range sevRank {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// FromSweep assembles the page. The coverage half comes from the plan and the enumeration; the
// findings half from the auditor. Keeping the assembly in one function is the binding: there is no
// path that reaches Render with a typed-in fraction.
func FromSweep(pl *gate.Plan, dis *gate.Discharge, res *gate.Result, a Authored) Input {
	in := Input{
		Repo: a.Repo, SkillVersion: a.SkillVersion, LexiconVer: a.LexiconVer,
		FamiliesRun: a.FamiliesRun, Findings: a.Findings,

		SHA:          pl.SHA,
		Denominator:  pl.ProductionTotal,
		AttestedRepo: res.Attested.Repo,
		AttestedPlan: res.Attested.Plan,
		Waived:       res.Attested.Waived,
		Accounting:   res.Accounting,
		Remaining:    res.Remaining,
		FamiliesNot:  res.FamiliesDeclaredNotRun,

		Tier:           dis.Tier,
		NearMisses:     dis.NearMisses,
		MapLimitations: pl.MapLimitations,
	}
	for _, c := range dis.CheckedClean {
		in.CheckedClean = append(in.CheckedClean, struct {
			Class  string `json:"class"`
			Method string `json:"method"`
		}{Class: c.Class, Method: c.Method})
	}
	return in
}

var page = template.Must(template.New("p").Funcs(template.FuncMap{
	"sevclass": func(s string) string {
		switch strings.ToLower(s) {
		case "blocking":
			return "block"
		case "fix-or-file":
			return "fix"
		}
		return "note"
	},
}).Parse(htmlTemplate))

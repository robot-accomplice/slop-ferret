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
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"sort"
	"strings"
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

// Input is everything the page needs. Attested/accounting figures come from `enumerate` so the
// report cannot disagree with the tool that produced them.
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
func ParseInput(b []byte) (Input, error) {
	var in Input
	err := json.Unmarshal(b, &in)
	return in, err
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

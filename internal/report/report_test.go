package report

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

func render(t *testing.T, in Input) string {
	t.Helper()
	var b bytes.Buffer
	if err := Render(&b, in); err != nil {
		t.Fatal(err)
	}
	return out
}

func sample() Input {
	return Input{
		Repo: "ghola", SHA: "4f33b3c", SkillVersion: "2026-08-02.2", LexiconVer: "2026-08-01.1",
		Tier: "1-2", AttestedRepo: "23/25", AttestedPlan: "25/25", Waived: 2,
		Accounting: "complete", Denominator: 25,
		FamiliesRun: []string{"H", "G"}, FamiliesNot: []string{"C", "D"},
		Findings: []Finding{
			{Title: "note thing", Severity: "note", Status: "VERIFIED", File: "a.go"},
			{Title: "suspected blocker", Severity: "blocking", Status: "SUSPECTED", File: "b.rs"},
			{Title: "verified blocker", Severity: "blocking", Status: "VERIFIED", File: "c.go",
				Claim: "x is never called", Occurrences: 3},
			{Title: "fixable", Severity: "fix-or-file", Status: "VERIFIED", File: "d.go"},
		},
		NearMisses: []string{"thing — refuted by the clamp"},
	}
}

// Severity first, never volume first: count runs INVERSE to severity, so a page ordered by count
// argues for exactly the wrong priority.
func TestFindingsAreOrderedSeverityFirstThenVerifiedFirst(t *testing.T) {
	var b bytes.Buffer
	if err := Render(&b, sample()); err != nil {
		t.Fatal(err)
	}
	out := out
	order := []string{"verified blocker", "suspected blocker", "fixable", "note thing"}
	prev := -1
	for _, want := range order {
		i := strings.Index(out, want)
		if i < 0 {
			t.Fatalf("%q missing", want)
		}
		if i < prev {
			t.Errorf("%q out of order", want)
		}
		prev = i
	}
}

// Deterministic: the same input renders byte-identically. That is the whole reason this is not
// hand-written.
func TestRenderIsDeterministic(t *testing.T) {
	var a, b bytes.Buffer
	if err := Render(&a, sample()); err != nil {
		t.Fatal(err)
	}
	if err := Render(&b, sample()); err != nil {
		t.Fatal(err)
	}
	if a.String() != out {
		t.Fatal("two renders of the same input differ")
	}
}

// Self-contained by construction: no external stylesheet, script, font or image. A report that
// phones out is not a local file.
func TestPageMakesNoExternalRequests(t *testing.T) {
	out := render(t, sample())
	for _, bad := range []string{"<script", "<link", "@import", "src=", "http://", "https://"} {
		if strings.Contains(out, bad) {
			t.Errorf("page contains %q — it must be self-contained", bad)
		}
	}
}

// The rate is suppressed below ~100 non-test source files and the denominator is published either
// way: one finding moves a small rate by 13-50 points, and a number is harder to retract than a blank.
func TestRateIsSuppressedBelowTheFloorAndTheDenominatorIsAlwaysShown(t *testing.T) {
	in := sample()
	out := render(t, in)
	if !strings.Contains(out, "n/a (denominator 25 &lt; 100)") &&
		!strings.Contains(out, "n/a (denominator 25 < 100)") {
		t.Errorf("small denominator must suppress the rate and say why")
	}
	if !strings.Contains(out, "denominator 25") {
		t.Error("the denominator must be published regardless")
	}
	in.Denominator = 200
	b.Reset()
	Render(&b, in)
	if !strings.Contains(out, "per 1,000") {
		t.Error("above the floor the rate should render")
	}
}

// A sweep with SUSPECTED findings and zero VERIFIED has verified nothing. Say so on the face of the
// page rather than letting a blank rate read as a clean result.
func TestZeroVerifiedWithSuspectedSaysSoOnItsFace(t *testing.T) {
	in := sample()
	in.Findings = []Finding{{Title: "lead", Severity: "blocking", Status: "SUSPECTED", File: "x.rs"}}
	out := render(t, in)
	if !strings.Contains(out, "none verified") {
		t.Error("must name the tell: leads with nothing verified is not a clean result")
	}
}

// The self-report caveat is the frame, not a footnote.
func TestThePageStatesThatItDoesNotObserveReading(t *testing.T) {
	out := render(t, sample())
	if !strings.Contains(out, "does not observe reading") {
		t.Error("the page must state that the read figures are the auditor's statement")
	}
}

// Prose from the sweep is untrusted input to the renderer: a finding title containing markup must
// not become markup.
func TestFindingProseIsEscaped(t *testing.T) {
	in := sample()
	in.Findings = []Finding{{Title: `<img src=x onerror=alert(1)>`, Severity: "note",
		Status: "VERIFIED", File: "a.go"}}
	out := render(t, in)
	if strings.Contains(out, "<img src=x") {
		t.Error("finding prose must be escaped, not injected")
	}
}

// Coverage before results: a reader must know the shape of the sweep before reading its output.
func TestCoverageSectionPrecedesFindings(t *testing.T) {
	out := render(t, sample())
	if strings.Index(out, "what was and was not covered") > strings.Index(out, "Findings — severity first") {
		t.Error("coverage must come before results")
	}
}

func TestTemplateHasNoUnrenderedActions(t *testing.T) {
	out := render(t, sample())
	// `}}` alone is not a marker — CSS closes nested @media blocks with it. A template ACTION
	// always opens with `{{`, and the stylesheet never contains that sequence.
	if regexp.MustCompile(`\{\{`).MatchString(out) {
		t.Error("unrendered template actions in the output")
	}
}

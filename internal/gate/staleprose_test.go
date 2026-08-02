package gate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// PROSE THAT DESCRIBES BEHAVIOUR THE CODE LACKS IS THIS TOOL'S OWN SUBJECT, and it has now shipped
// here across two review campaigns.
//
// The first version of this test was a ten-string denylist. It was written from the four fixes
// that had just been made rather than from the failure mode, so it could only ever catch those
// four, and CI stayed green over a rename that never reached the deployed SKILL.md, the README
// command table, the runtime `instructions` string, or the CHANGELOG — 25+ live references. A
// denylist of what you already fixed is not a guard; it is a receipt.
//
// So the command half is DERIVED. The accepted verbs are parsed out of cmd/ferret's dispatch, and
// any `ferret <verb>` in any tracked file whose verb is not one of them fails. Adding a command
// passes automatically; REMOVING or renaming one turns every stale mention red without anyone
// having to remember to update a list here.
func TestNoStaleCommandNamesOrRemovedFeatureClaims(t *testing.T) {
	root := repoRoot(t)
	out, err := exec.Command("git", "-C", root, "ls-files").Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}

	live := liveCommands(t, root)

	// A line that must name a retired identifier — a migration fixture, a changelog entry recording
	// the rename — marks itself. Deliberately narrow: one line, visible in the diff, and it says
	// what it is. The alternative that failed twice was exempting whole files.
	const allowMarker = "staleprose:allow"

	// Retired IDENTIFIERS cannot be derived — they are gone from the code by definition, which is
	// the whole problem. This list is the one hand-maintained part, and it is append-only: a name
	// goes in when it is retired and never comes out.
	retired := map[string]string{
		"coverage.repo": "renamed to `attested.repo`; enumerate emits no such field",
		"coverage.plan": "renamed to `attested.plan`; enumerate emits no such field",
		"coverage_repo": "renamed to `attested_repo` in the record schema",
		"coverage_plan": "renamed to `attested_plan` in the record schema",
		"status: settled": "the verdict word was removed; the field is `accounting` and its " +
			"values are complete/incomplete",
		"EMBEDDED in the binary": "the embed was removed; the binary carries no prose",
	}

	// Only genuine history is exempt. The previous exempt list included CHANGELOG.md and the spec
	// — the two documents a reader is most likely to believe — so the exemption sheltered live
	// wrong prose instead of history. If a file needs to quote a retired name, it needs to be a
	// record OF the retirement.
	exempt := map[string]bool{
		"internal/gate/staleprose_test.go": true,
		"docs/releases/v0.1.0-abort.md":    true,
	}

	// Only `ferret <verb>` in COMMAND POSITION counts — after a backtick, a shell prompt, or at the
	// start of a line. "the ferret method" and "ferrets AI slop out" are English, and a matcher
	// that flags them trains the reader to skim the failure list, which is how a real hit gets
	// waved through.
	cmdRe := regexp.MustCompile("(?m)(?:`|\\$ |^\\s*)ferret ([a-z][a-z0-9-]*)")

	var problems []string
	for _, rel := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if rel == "" || exempt[rel] || strings.HasPrefix(rel, "skill/versions/") ||
			strings.HasSuffix(rel, ".png") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			continue
		}
		body := string(b)

		for _, m := range cmdRe.FindAllStringSubmatch(body, -1) {
			verb := m[1]
			if live[verb] {
				continue
			}
			problems = append(problems, fmt.Sprintf(
				"%s: %q — cmd/ferret accepts no such command (live: %s)",
				rel, m[0], strings.Join(sortedKeys(live), " ")))
		}
		// Line by line, so a single legitimate mention can be marked without exempting a whole
		// file. Whole-file exemption is what let the CHANGELOG document a nonexistent command for
		// two review cycles: the carve-out meant for history sheltered live wrong prose.
		for n, line := range strings.Split(body, "\n") {
			if strings.Contains(line, allowMarker) {
				continue
			}
			for bad, why := range retired {
				if strings.Contains(line, bad) {
					problems = append(problems, fmt.Sprintf("%s:%d: contains %q — %s",
						rel, n+1, bad, why))
				}
			}
		}
	}
	sort.Strings(problems)
	if len(problems) > 0 {
		t.Errorf("prose describing behaviour the code lacks (%d):\n  %s",
			len(problems), strings.Join(problems, "\n  "))
	}
}

// liveCommands parses the verbs cmd/ferret actually dispatches on. Reading the dispatch source is
// what makes this test self-maintaining: a hand-kept copy of the command list here would drift
// from the switch exactly the way the prose drifted from the code.
func liveCommands(t *testing.T, root string) map[string]bool {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, "cmd", "ferret", "main.go"))
	if err != nil {
		t.Fatalf("cannot read the dispatch to derive the command set: %v", err)
	}
	live := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^\tcase (".+"):`).FindAllStringSubmatch(string(b), -1) {
		for _, lit := range strings.Split(m[1], ",") {
			v := strings.Trim(strings.TrimSpace(lit), `"`)
			if v != "" && !strings.HasPrefix(v, "-") {
				live[v] = true
			}
		}
	}
	if len(live) < 5 {
		t.Fatalf("derived only %d commands from the dispatch (%v) — the parse broke, and a "+
			"silently-empty command set would make this whole test pass vacuously",
			len(live), sortedKeys(live))
	}
	return live
}

func repoRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatalf("not in a git repo: %v", err)
	}
	return strings.TrimSpace(string(out))
}

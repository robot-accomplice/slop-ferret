package gate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// PROSE THAT DESCRIBES BEHAVIOUR THE CODE LACKS IS THIS TOOL'S OWN SUBJECT, and it shipped here
// four separate times in one day: a skill telling agents to run `slop-ferret plan` after the binary
// became `ferret`; package docs describing an embed that had been removed; a runtime `instructions`
// string naming a nonexistent command; a `doctor` message pointing at one.
//
// Each was fixed by hand and each recurred somewhere the previous fix had not looked — the
// remediation grep covered only `README.md docs/ CHANGELOG.md`, so `internal/` and `cmd/` were
// declared clean having never been checked.
//
// This is the class fix. It runs on every CI, over every tracked file, so a rename cannot again be
// "finished" while the strings that reach a user still say otherwise.
func TestNoStaleCommandNamesOrRemovedFeatureClaims(t *testing.T) {
	root := repoRoot(t)
	out, err := exec.Command("git", "-C", root, "ls-files").Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}

	// Each entry: the forbidden string, and why it is forbidden.
	forbidden := map[string]string{
		"slop-ferret plan":       "the binary is `ferret`; `slop-ferret` is the project",
		"slop-ferret enumerate":  "the binary is `ferret`",
		"slop-ferret verify":     "`verify` was renamed to `enumerate`",
		"slop-ferret install":    "the binary is `ferret`",
		"slop-ferret update":     "the binary is `ferret`",
		"slop-ferret doctor":     "the binary is `ferret`",
		"slop-ferret records":    "the binary is `ferret`",
		"slop-ferret report":     "the binary is `ferret`",
		"ferret verify":          "`verify` was renamed to `enumerate`",
		"EMBEDDED in the binary": "the embed was removed; the binary carries no prose",
	}
	// Files that legitimately discuss the history of these strings.
	exempt := map[string]bool{
		"internal/gate/staleprose_test.go":                             true,
		"docs/releases/v0.1.0-abort.md":                                true,
		"CHANGELOG.md":                                                 true,
		"docs/superpowers/specs/2026-08-01-slop-ferret-design.md":      true,
		"docs/superpowers/plans/2026-08-01-ferret-spec-conformance.md": true,
	}

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
		for bad, why := range forbidden {
			if strings.Contains(body, bad) {
				problems = append(problems, rel+": contains "+bad+" — "+why)
			}
		}
	}
	if len(problems) > 0 {
		t.Errorf("stale prose describing behaviour the code lacks (%d):\n  %s",
			len(problems), strings.Join(problems, "\n  "))
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatalf("not in a git repo: %v", err)
	}
	return strings.TrimSpace(string(out))
}

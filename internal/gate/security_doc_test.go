package gate

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// SECURITY.md enumerates the git subcommands the tool runs against a target repo as a load-bearing
// claim: `git -C <repo>` honours the target's .git/config, so every subcommand is a config-driven
// execution surface a reader must be told about. That list shipped incomplete twice. The omitted
// `remote get-url` was a real path-traversal defect. Then the very sentence warning that an
// incomplete list is dangerous was itself short of `rev-list` and `rev-parse` — the failure class
// this tool hunts, sitting in the security doc.
//
// Derive the set from the gitLines call sites rather than trusting the prose: a hand-kept list in a
// test would drift exactly the way the doc drifted. Break it: add a gitLines(repo, "reflog", ...)
// call, or delete a subcommand's name from SECURITY.md, and this goes red.
func TestSecurityMdNamesEveryGitSubcommandRunAgainstATarget(t *testing.T) {
	root := repoRoot(t)

	// Every `gitLines(repo, "<sub>", ...)` is a subcommand run against the target repo.
	callRe := regexp.MustCompile(`gitLines\(repo, "([a-z][a-z-]*)"`)
	subs := map[string]string{} // subcommand -> the source file it was found in
	entries, err := os.ReadDir(filepath.Join(root, "internal", "gate"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, "internal", "gate", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range callRe.FindAllStringSubmatch(string(b), -1) {
			subs[m[1]] = e.Name()
		}
	}
	if len(subs) < 5 {
		t.Fatalf("found only %d gitLines subcommands (%v) — the match broke, and an empty set "+
			"would pass this test vacuously", len(subs), subs)
	}

	secBytes, err := os.ReadFile(filepath.Join(root, "SECURITY.md"))
	if err != nil {
		t.Fatal(err)
	}
	sec := string(secBytes)
	for sub, file := range subs {
		// Named in command position, e.g. `ls-files`, `diff --name-only`, `rev-list ...`.
		if !strings.Contains(sec, "`"+sub) {
			t.Errorf("the code runs `git %s` against a target (%s) but SECURITY.md's git-subcommand "+
				"list does not name it — the same incomplete-list defect this tool hunts, in the "+
				"security doc", sub, file)
		}
	}
}

package gate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A record must carry the sha it swept, and that sha must RESOLVE. Two prior sweeps recorded
// boundaries that no longer exist as objects -- both taken from dirty maps -- which made their
// denominators unreproducible and left the next sweep unable to scope itself.
func TestWriteRecordRefusesAShaThatDoesNotResolve(t *testing.T) {
	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n"})
	t.Setenv("HOME", t.TempDir())
	pl := planFor(t, repo)
	pl.SHA = "0000000000000000000000000000000000000000"
	// Status must be settled so this reaches the sha check rather than the settle gate — the
	// precedence is deliberate: a sweep that did not settle is refused before anything else.
	_, err := WriteRecord(repo, pl, &Discharge{SHA: pl.SHA}, &Result{Status: "settled"})
	if err == nil || !strings.Contains(err.Error(), "does not resolve") {
		t.Fatalf("want a refusal naming the unresolvable sha, got %v", err)
	}
}

func TestWriteRecordThenListRoundTrips(t *testing.T) {
	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n"})
	home := t.TempDir()
	t.Setenv("HOME", home)
	sha := headSHA(t, repo)

	pl := planFor(t, repo)
	pl.SHA = sha
	dis := &Discharge{SHA: sha, Tier: "1-2",
		CheckedClean: []CheckedClean{{Class: "phantom dependency", Method: "build+vet, 4 targets"}},
		NearMisses:   []string{"limit-rate starvation — refuted by the chunk clamp"},
		ReportPath:   "/tmp/report.html"}
	res := &Result{Coverage: Coverage{Repo: "17/25", Plan: "25/25"}, Status: "settled"}

	path, err := WriteRecord(repo, pl, dis, res)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(path, filepath.Join(home, ".slop-ferret", "records")) {
		t.Errorf("record written outside the store: %s", path)
	}
	if _, err := os.Stat(filepath.Join(repo, ".slop-ferret")); err == nil {
		t.Error("a record must never be written into the target repo")
	}

	got, err := ListRecords(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("ListRecords = %d records, want 1", len(got))
	}
	r := got[0]
	if r.SHA != sha || r.CoverageRepo != "17/25" || r.Tier != "1-2" {
		t.Errorf("round-trip lost fields: %+v", r)
	}
	// The attested half must survive: it is the half the next sweep reads to avoid re-spending
	// budget on classes already recorded clean, WITH the method used. "Clean" with no method is
	// not checkable, and an unchecked clean is how a later sweep skips ground nobody covered.
	if len(r.CheckedClean) != 1 || r.CheckedClean[0].Method == "" {
		t.Errorf("checked-clean method not recorded: %+v", r.CheckedClean)
	}
}

func TestRepoKeyPrefersTheOriginURL(t *testing.T) {
	repo := gitRepo(t, map[string]string{"a.go": "package a\n"})
	run(t, repo, "remote", "add", "origin", "https://github.com/robot-accomplice/ghola.git")
	key, err := RepoKey(repo)
	if err != nil {
		t.Fatal(err)
	}
	if key != "github.com/robot-accomplice/ghola" {
		t.Errorf("RepoKey = %q", key)
	}
}

func TestRepoKeyFallsBackForARemotelessRepo(t *testing.T) {
	key, err := RepoKey(gitRepo(t, map[string]string{"a.go": "package a\n"}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(key, "path-") {
		t.Errorf("RepoKey = %q, want a path- fallback", key)
	}
}

func TestListRecordsIsEmptyNotAnErrorForAnUnsweptRepo(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	got, err := ListRecords(gitRepo(t, map[string]string{"a.go": "package a\n"}))
	if err != nil {
		t.Fatalf("an unswept repo is a normal state, not an error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d records", len(got))
	}
}

// End-to-end: verify writes a record by default and --no-record suppresses it. Always-write is the
// point -- a record you must remember to request is one that will not exist when the next sweep
// looks for it.
func TestVerifyAndRecordWritesByDefaultAndCanBeSuppressed(t *testing.T) {
	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n"})
	t.Setenv("HOME", t.TempDir())
	pl := planFor(t, repo)
	pl.SHA = headSHA(t, repo)
	pp := writeJSON(t, pl)
	dp := writeJSON(t, map[string]any{
		"sha": pl.SHA, "read_paths": pl.ProductionFiles,
		"families_not_run": []string{"D", "E"}, "tier": "1-2"})

	_, path, code, err := VerifyAndRecord(pp, dp, repo, true)
	if err != nil {
		t.Fatal(err)
	}
	if code != ExitOK {
		t.Fatalf("code=%d", code)
	}
	if path == "" {
		t.Fatal("a record should have been written")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("recorded path does not exist: %v", err)
	}

	_, path2, _, err := VerifyAndRecord(pp, dp, repo, false)
	if err != nil {
		t.Fatal(err)
	}
	if path2 != "" {
		t.Errorf("--no-record must suppress the write, got %q", path2)
	}
}

// A record that cannot be written must surface, but must not discard a verify result the operator
// already earned.
func TestVerifyAndRecordSurfacesAWriteFailureWithoutLosingTheResult(t *testing.T) {
	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n"})
	t.Setenv("HOME", t.TempDir())
	pl := planFor(t, repo)
	pl.SHA = "0000000000000000000000000000000000000000" // will not resolve
	pp := writeJSON(t, pl)
	dp := writeJSON(t, map[string]any{"sha": pl.SHA, "read_paths": pl.ProductionFiles,
		"families_not_run": []string{"D", "E"}})

	res, _, _, err := VerifyAndRecord(pp, dp, repo, true)
	if err == nil || !strings.Contains(err.Error(), "record") {
		t.Fatalf("want the record failure surfaced, got %v", err)
	}
	if res == nil {
		t.Fatal("the verify result must survive a failed record")
	}
}

// CONDITION 1 (ABORT S1). The audited repository's `origin` URL was used as a path key with no
// containment check, and filepath.Join CLEANS AFTER JOINING, so `..` walked straight out of the
// records root. Reproduced during the review: a repo whose origin was
// `https://../../.claude/skills/slop-ferret/PWNED` created directories inside ~/.claude and wrote
// there; a branch named `settings` then let it overwrite ~/.claude/settings.json.
//
// This tool exists to be pointed at repositories you have reason to distrust. Handing them a write
// primitive into the operator's home is disqualifying, so containment is asserted twice: the key is
// reduced to safe segments, AND the resolved directory is re-checked against the root before any
// MkdirAll.
func TestRepoKeyCannotEscapeTheRecordsRoot(t *testing.T) {
	hostile := []string{
		"https://../../.claude/skills/slop-ferret/PWNED",
		"https://evil/../../../../../../tmp/pwned/x",
		"/absolute/path",
		"..",
		"../..",
		`https://evil/..\..\windows`,
	}
	for _, url := range hostile {
		repo := gitRepo(t, map[string]string{"a.go": "package a\n"})
		run(t, repo, "remote", "add", "origin", url)
		key, err := RepoKey(repo)
		if err != nil {
			continue // refusing outright is also acceptable
		}
		// The property is CONTAINMENT, not the absence of a substring: a segment literally named
		// `..foo` is a harmless directory name. Join it under a root and check it stays there.
		root := "/records"
		joined := filepath.Join(root, filepath.FromSlash(key))
		if rel, err := filepath.Rel(root, joined); err != nil || rel == ".." ||
			strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Errorf("origin %q produced an escaping key %q (resolves to %s)", url, key, joined)
		}
	}
}

func TestWriteRecordRefusesToEscapeEvenIfTheKeyIsHostile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n"})
	run(t, repo, "remote", "add", "origin", "https://../../ESCAPED/marker")
	sha := headSHA(t, repo)
	pl := planFor(t, repo)
	pl.SHA = sha

	path, err := WriteRecord(repo, pl, &Discharge{SHA: sha}, &Result{Status: "settled"})
	if err == nil {
		root := filepath.Join(home, ".slop-ferret", "records")
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil || strings.HasPrefix(rel, "..") {
			t.Fatalf("record escaped the store: %s (root %s)", path, root)
		}
	}
	// Nothing may exist outside the store, whether the write succeeded or was refused. The segment
	// name may legitimately survive INSIDE the store — it is a name, not a traversal — so the test
	// is containment, not absence of the string.
	root := filepath.Join(home, ".slop-ferret", "records")
	var escaped []string
	_ = filepath.Walk(home, func(p string, fi os.FileInfo, e error) error {
		if e != nil || fi.IsDir() {
			return nil
		}
		if rel, rerr := filepath.Rel(root, p); rerr != nil || strings.HasPrefix(rel, "..") {
			escaped = append(escaped, p)
		}
		return nil
	})
	if len(escaped) > 0 {
		t.Fatalf("wrote outside the records root: %v", escaped)
	}
}

// The SHA reaches a filename. It must look like an object id, not like a branch name someone chose
// — a branch called `settings` was how the review turned traversal into overwriting settings.json.
func TestWriteRecordRefusesANonHexSHA(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n"})
	run(t, repo, "branch", "settings")
	pl := planFor(t, repo)
	pl.SHA = "settings"
	if _, err := WriteRecord(repo, pl, &Discharge{SHA: "settings"}, &Result{Status: "settled"}); err == nil {
		t.Fatal("a non-hex sha must not reach a filename")
	}
}

// CONDITION 3 (ABORT I1) — THE ONE FIX. A record is durable input to a FUTURE sweep: SKILL.md tells
// the next run to read `checked_clean` and not re-spend budget there. The review reproduced a run
// that read zero files, left 100 items open, exited 3, and still wrote a record asserting two
// classes clean with an EMPTY method. That is the persistence layer converting an unperformed audit
// into a completed-looking one — the exact invariant this tool exists to defend.
func TestWriteRecordRefusesAnUnsettledSweep(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n"})
	pl := planFor(t, repo)
	pl.SHA = headSHA(t, repo)
	dis := &Discharge{SHA: pl.SHA, CheckedClean: []CheckedClean{{Class: "dead-on-arrival", Method: "x"}}}
	_, err := WriteRecord(repo, pl, dis, &Result{Status: "open"})
	if err == nil || !strings.Contains(err.Error(), "did not settle") {
		t.Fatalf("an unsettled sweep must not become a durable record: %v", err)
	}
}

func TestWriteRecordDropsACheckedCleanWithNoMethod(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n"})
	pl := planFor(t, repo)
	pl.SHA = headSHA(t, repo)
	dis := &Discharge{SHA: pl.SHA, CheckedClean: []CheckedClean{
		{Class: "dead-on-arrival", Method: ""},
		{Class: "phantom dependency", Method: "build+vet on 4 targets"},
	}}
	if _, err := WriteRecord(repo, pl, dis, &Result{Status: "settled"}); err != nil {
		t.Fatal(err)
	}
	got, err := ListRecords(repo)
	if err != nil || len(got) != 1 {
		t.Fatalf("%v %d", err, len(got))
	}
	if len(got[0].CheckedClean) != 1 || got[0].CheckedClean[0].Class != "phantom dependency" {
		t.Errorf("a clean with no method is not checkable and must be dropped: %+v", got[0].CheckedClean)
	}
}

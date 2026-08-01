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
	_, err := WriteRecord(repo, pl, &Discharge{SHA: pl.SHA}, &Result{})
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

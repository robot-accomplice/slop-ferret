package gate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
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
	_, err := WriteRecord(repo, pl, &Discharge{SHA: pl.SHA}, &Result{Accounting: "complete"})
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
	res := &Result{Attested: Attested{Repo: "17/25", Plan: "25/25"}, Accounting: "complete"}

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
	if r.SHA != sha || r.AttestedRepo != "17/25" || r.Tier != "1-2" {
		t.Errorf("round-trip lost fields: %+v", r)
	}
	// The attested half must survive: it is the half the next sweep reads to avoid re-spending
	// budget on classes already recorded clean, WITH the method used. "Clean" with no method is
	// not checkable, and an unchecked clean is how a later sweep skips ground nobody covered.
	if len(r.CheckedClean) != 1 || r.CheckedClean[0].Method == "" {
		t.Errorf("checked-clean method not recorded: %+v", r.CheckedClean)
	}
}

// REPLACED 2026-08-03. These two tests used to assert that RepoKey PREFERRED the origin URL and
// fell back to a path hash. Both asserted the defect (ABORT II, A2): `origin` is configuration, so
// it is neither stable across checkouts nor beyond the audited repository's control.
//
// The store on the operator's disk proved the first half — it held one repository under two keys,
// `Users/jmachen/code/slop-ferret/` and `github.com/robot-accomplice/slop-ferret/`, with disjoint
// histories, so a second sweep could not see the first.
func TestRepoKeyIsStableWhenTheOriginChanges(t *testing.T) {
	repo := gitRepo(t, map[string]string{"a.go": "package a\n"})
	before, err := RepoKey(repo)
	if err != nil {
		t.Fatal(err)
	}
	run(t, repo, "remote", "add", "origin", "https://github.com/robot-accomplice/ghola.git")
	after, err := RepoKey(repo)
	if err != nil {
		t.Fatal(err)
	}
	run(t, repo, "remote", "set-url", "origin", "git@github.com:someone/else.git")
	moved, err := RepoKey(repo)
	if err != nil {
		t.Fatal(err)
	}
	if before != after || after != moved {
		t.Errorf("the key moved when the remote did (%q -> %q -> %q). A repository that changes "+
			"host, or is cloned twice, must not lose its own sweep history", before, after, moved)
	}
}

// The forgery half. A repository's `origin` is an unauthenticated string IT controls, and this tool
// is pointed at repositories you have reason to distrust. Claiming a victim's URL must not place a
// record in the victim's directory, because SKILL.md Step 0.2 tells the next sweep to read
// `checked_clean` there and not re-spend budget on it.
func TestAHostileOriginCannotReachAnotherRepositorysRecords(t *testing.T) {
	victim := gitRepo(t, map[string]string{"real.go": "package v\n"})
	run(t, victim, "remote", "add", "origin", "https://github.com/robot-accomplice/counterspy.git")
	victimKey, err := RepoKey(victim)
	if err != nil {
		t.Fatal(err)
	}

	hostile := gitRepo(t, map[string]string{"evil.go": "package h\n"})
	run(t, hostile, "remote", "add", "origin", "https://github.com/robot-accomplice/counterspy.git")
	hostileKey, err := RepoKey(hostile)
	if err != nil {
		t.Fatal(err)
	}

	if hostileKey == victimKey {
		t.Fatalf("a repository that merely CLAIMS another's origin got its key (%q). Records are "+
			"durable input to a later sweep: this is a write primitive into another project's "+
			"audit history", hostileKey)
	}
}

// The fallback is for a repository with no commits to key on. It must still be usable rather than
// unrecordable, and must announce which method produced it -- the fallback is genuinely weaker.
func TestAnUnbornRepoFallsBackToThePathAndSaysSo(t *testing.T) {
	d := t.TempDir()
	run(t, d, "init", "-q")
	key, method, err := repoIdentity(d)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(key, "path-") || method != "absolute-path" {
		t.Errorf("key=%q method=%q, want a path- fallback naming itself", key, method)
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
func TestEnumerateAndRecordWritesByDefaultAndCanBeSuppressed(t *testing.T) {
	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n"})
	t.Setenv("HOME", t.TempDir())
	pl := planFor(t, repo)
	pl.SHA = headSHA(t, repo)
	pp := writeJSON(t, pl)
	dp := writeJSON(t, map[string]any{
		"sha": pl.SHA, "read_paths": pl.ProductionFiles,
		"families_not_run": []string{"D", "E"}, "tier": "1-2"})

	_, path, code, err := EnumerateAndRecord(pp, dp, repo, true)
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

	_, path2, _, err := EnumerateAndRecord(pp, dp, repo, false)
	if err != nil {
		t.Fatal(err)
	}
	if path2 != "" {
		t.Errorf("--no-record must suppress the write, got %q", path2)
	}
}

// A record that cannot be written must surface, but must not discard a verify result the operator
// already earned.
func TestEnumerateAndRecordSurfacesAWriteFailureWithoutLosingTheResult(t *testing.T) {
	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n"})
	t.Setenv("HOME", t.TempDir())
	pl := planFor(t, repo)
	pl.SHA = "0000000000000000000000000000000000000000" // will not resolve
	pp := writeJSON(t, pl)
	dp := writeJSON(t, map[string]any{"sha": pl.SHA, "read_paths": pl.ProductionFiles,
		"families_not_run": []string{"D", "E"}})

	res, _, _, err := EnumerateAndRecord(pp, dp, repo, true)
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

	path, err := WriteRecord(repo, pl, &Discharge{SHA: sha}, &Result{Accounting: "complete"})
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
	if _, err := WriteRecord(repo, pl, &Discharge{SHA: "settings"}, &Result{Accounting: "complete"}); err == nil {
		t.Fatal("a non-hex sha must not reach a filename")
	}
}

// CONDITION 3 (ABORT I1) — THE ONE FIX. A record is durable input to a FUTURE sweep: SKILL.md tells
// the next run to read `checked_clean` and not re-spend budget there. The review reproduced a run
// that read zero files, left 100 items open, exited 3, and still wrote a record asserting two
// classes clean with an EMPTY method. That is the persistence layer converting an unperformed audit
// into a completed-looking one — the exact invariant this tool exists to defend.
func TestWriteRecordRefusesAnIncompleteAccounting(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n"})
	pl := planFor(t, repo)
	pl.SHA = headSHA(t, repo)
	dis := &Discharge{SHA: pl.SHA, CheckedClean: []CheckedClean{{Class: "dead-on-arrival", Method: "x"}}}
	_, err := WriteRecord(repo, pl, dis, &Result{Accounting: "incomplete"})
	if err == nil || !strings.Contains(err.Error(), "incomplete accounting") {
		t.Fatalf("an incomplete accounting must not become a durable record: %v", err)
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
	if _, err := WriteRecord(repo, pl, dis, &Result{Accounting: "complete"}); err != nil {
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

// A RENAME ONCE DESTROYED THE STORE IN SILENCE. `coverage_repo`/`status` became  staleprose:allow
// `attested_repo`/`accounting` with no schema field and no migration, so four real campaign
// records unmarshalled to zero values and `ferret records` printed them as
// `stated-read <blank>  plan <blank>` with no accounting — the `open` marker that says DO NOT
// TRUST THIS evaporated in the change meant to make claims more honest.
//
// Absence must never render as a value. A record this binary cannot read is reported as
// unreadable, not as a record with nothing in it.
func TestALegacyRecordIsReportedNotRenderedAsBlanks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := gitRepo(t, map[string]string{"a.go": "package a\n"})
	key, err := RepoKey(repo)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, ".slop-ferret", "records", filepath.FromSlash(key))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// The exact shape on the operator's disk today. Assembled from parts because it must contain
	// the retired field names verbatim — this fixture IS the old schema, which is the whole point —
	// and a JSON string literal has nowhere to put a per-line marker.
	legacy := `{"sha":"4f33b3c","date":"2026-07-01",` + // staleprose:allow
		`"` + "coverage" + `_repo":"23/25",` +
		`"` + "coverage" + `_plan":"25/25","denominator":25,"status":"settled"}`
	if err := os.WriteFile(filepath.Join(dir, "4f33b3c.json"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	recs, err := ListRecords(repo)
	if err == nil {
		t.Fatal("a record predating the schema must be reported, not silently listed as empty")
	}
	if !strings.Contains(err.Error(), "NOT blank") {
		t.Errorf("the error must distinguish unreadable from empty: %v", err)
	}
	for _, r := range recs {
		if r.AttestedRepo == "" {
			t.Error("a record with blank figures reached the caller — that is the defect itself")
		}
	}
}

// The other half: a record this binary wrote must read back, or the guard above is just a
// permanently broken command.
func TestARecordWrittenNowReadsBack(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := gitRepo(t, map[string]string{"a.go": "package a\n"})
	sha := strings.TrimSpace(runOut(t, repo, "rev-parse", "--short", "HEAD"))
	pl := &Plan{SHA: sha, ProductionTotal: 1, ProductionFiles: []string{"a.go"}}
	dis := &Discharge{SHA: sha, ReadPaths: []string{"a.go"}}
	res := &Result{Accounting: "complete", Attested: Attested{Repo: "1/1", Plan: "1/1"}}
	if _, err := WriteRecord(repo, pl, dis, res); err != nil {
		t.Fatal(err)
	}
	recs, err := ListRecords(repo)
	if err != nil {
		t.Fatalf("a record this binary just wrote must read back: %v", err)
	}
	if len(recs) != 1 || recs[0].AttestedRepo != "1/1" || recs[0].Schema != RecordSchema {
		t.Fatalf("round-trip lost data: %+v", recs)
	}
}

// The key moved from origin-derived to root-commit-derived, which ORPHANED every record an older
// binary wrote. `ferret records` then printed nothing at all against a store holding five of them
// — the same "absence rendered as a value" defect the schema field was added to fix, reintroduced
// by the fix for the key. Silence is the one answer a records listing must never give when there
// are records.
func TestRecordsFromTheOldOriginKeyAreReportedNotSilentlyOrphaned(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := gitRepo(t, map[string]string{"a.go": "package a\n"})
	run(t, repo, "remote", "add", "origin", "https://github.com/robot-accomplice/ghola.git")

	// Exactly where the previous binary would have put it.
	old := filepath.Join(home, ".slop-ferret", "records", "github.com", "robot-accomplice", "ghola")
	must(t, os.MkdirAll(old, 0o755))
	must(t, os.WriteFile(filepath.Join(old, "4f33b3c.json"), []byte(`{"sha":"4f33b3c"}`), 0o644))

	recs, err := ListRecords(repo)
	if len(recs) != 0 {
		t.Fatalf("an origin-keyed record must not be read as current: %+v", recs)
	}
	if err == nil {
		t.Fatal("a store holding records must not list as empty — that is indistinguishable " +
			"from a repository nobody has ever swept")
	}
	if !strings.Contains(err.Error(), old) {
		t.Errorf("the report must name where they actually are: %v", err)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// A CLASS CANNOT BE BOTH CHECKED CLEAN AND NOT RUN. Three reviews found records asserting both —
// `families_not_run: ["A"..."H"]` beside `checked_clean: dead-on-arrival`, which is family A. It is
// not untidiness: SKILL.md Step 0.2 tells the next sweep to trust `checked_clean` and skip that
// ground, so the contradiction retires a family nobody ran.
func TestARecordCannotClaimAFamilyBothCleanAndNotRun(t *testing.T) {
	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n"})
	t.Setenv("HOME", t.TempDir())
	pl := planFor(t, repo)
	pl.SHA = headSHA(t, repo)

	dis := &Discharge{SHA: pl.SHA, FamiliesNotRun: []string{"D", "E"},
		CheckedClean: []CheckedClean{{Class: "E · single-impl interface", Method: "read them all"}}}
	_, err := WriteRecord(repo, pl, dis, &Result{Accounting: "complete"})
	if err == nil {
		t.Fatal("a family listed not-run and recorded clean must be refused, not persisted")
	}
	if !strings.Contains(err.Error(), "families_not_run") {
		t.Errorf("the refusal must name the contradiction: %v", err)
	}

	// The honest shape still records.
	ok := &Discharge{SHA: pl.SHA, FamiliesNotRun: []string{"D"},
		CheckedClean: []CheckedClean{{Class: "E · single-impl interface", Method: "read them all"}}}
	if _, err := WriteRecord(repo, pl, ok, &Result{Accounting: "complete"}); err != nil {
		t.Fatalf("a non-contradictory record must still write: %v", err)
	}
}

// An uncheckable method is not a method. "-" used to pass the non-empty test and reach the durable
// record, where the next sweep is told to trust it.
func TestUncheckableMethodsAreNotRecorded(t *testing.T) {
	for _, m := range []string{"-", "n/a", "N/A", "none", " TODO ", "?"} {
		if CheckableMethod(m) {
			t.Errorf("%q passed as a checked-clean method; a reader cannot check it", m)
		}
	}
	for _, m := range []string{"build+vet on 4 GOOS/GOARCH", "read every tier-1 path"} {
		if !CheckableMethod(m) {
			t.Errorf("%q is a real method and was rejected", m)
		}
	}
}

// The shallow-clone guard was defeated by a field that did not get it. repoIdentity correctly
// refuses a graft boundary as a KEY, but WriteRecord stamped RootCommit from that same moving
// boundary — so after `git fetch --deepen` the cross-check in ListRecords rejected the
// repository's own records as "a different history".
func TestAShallowCloneDoesNotStampAMovingRootCommit(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	origin := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n"})
	run(t, origin, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "--allow-empty", "-qm", "second")

	shallow := filepath.Join(t.TempDir(), "shallow")
	if out, err := exec.Command("git", "clone", "-q", "--depth", "1",
		"file://"+origin, shallow).CombinedOutput(); err != nil {
		t.Skipf("cannot make a shallow clone here: %v %s", err, out)
	}
	key, method, err := repoIdentity(shallow)
	if err != nil {
		t.Fatal(err)
	}
	if method != "absolute-path" || !strings.HasPrefix(key, "path-") {
		t.Fatalf("a shallow clone must not key on a graft boundary: key=%q method=%q", key, method)
	}

	pl := planFor(t, shallow)
	pl.SHA = headSHA(t, shallow)
	if _, err := WriteRecord(shallow, pl, &Discharge{SHA: pl.SHA},
		&Result{Accounting: "complete"}); err != nil {
		t.Fatal(err)
	}
	recs, err := ListRecords(shallow)
	if err != nil {
		t.Fatalf("a shallow clone must be able to read back its own record: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1", len(recs))
	}
	if recs[0].RootCommit != "" {
		t.Errorf("RootCommit = %q — stamping the graft boundary is what orphaned these records "+
			"on the next `git fetch --deepen`", recs[0].RootCommit)
	}
}

// The orphan report covered only origin-keyed records, so a REMOTELESS repo with records on disk
// still listed as empty with no error — the defect that check was added to close, live inside it.
func TestOrphanedRecordsAreReportedForARemotelessRepoToo(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := gitRepo(t, map[string]string{"a.go": "package a\n"}) // no origin

	abs, _ := filepath.Abs(repo)
	h := sha256.Sum256([]byte(abs))
	old := filepath.Join(home, ".slop-ferret", "records", "path-"+hex.EncodeToString(h[:])[:8])
	must(t, os.MkdirAll(old, 0o755))
	must(t, os.WriteFile(filepath.Join(old, "abc1234.json"), []byte(`{"sha":"abc1234"}`), 0o644))

	recs, err := ListRecords(repo)
	if len(recs) != 0 {
		t.Fatalf("an old-key record must not read as current: %+v", recs)
	}
	if err == nil {
		t.Fatal("a remoteless repo with records on disk listed as empty — indistinguishable from " +
			"one nobody has ever swept, which is what this listing exists to rule out")
	}
}

// The directory a record sits in is derived from the root commits, so a record whose OWN recorded
// root disagrees did not come from this repository — it was hand-placed, copied, or migrated. The
// safe reading of a contradiction between the directory and the file is to trust neither. Mutation
// showed this cross-check was removable with the suite green.
func TestARecordClaimingADifferentHistoryIsRefused(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := gitRepo(t, map[string]string{"internal/wallet/pay.go": "package w\n"})
	pl := planFor(t, repo)
	pl.SHA = headSHA(t, repo)
	path, err := WriteRecord(repo, pl, &Discharge{SHA: pl.SHA}, &Result{Accounting: "complete"})
	if err != nil {
		t.Fatal(err)
	}
	if recs, err := ListRecords(repo); err != nil || len(recs) != 1 {
		t.Fatalf("baseline: %d records, err=%v", len(recs), err)
	}

	// Rewrite the record in place to claim a different lineage, leaving it in this repo's directory.
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var r Record
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	r.RootCommit = "0000000000000000000000000000000000000000"
	nb, _ := json.Marshal(r)
	must(t, os.WriteFile(path, nb, 0o644))

	recs, err := ListRecords(repo)
	if len(recs) != 0 {
		t.Errorf("a record claiming another history was reported as this repo's: %+v", recs)
	}
	if err == nil || !strings.Contains(err.Error(), "different history") {
		t.Errorf("the contradiction must be named, not silently dropped: %v", err)
	}
}

// familyOf decides whether a checked-clean label can be reconciled against families_not_run. It
// must recognise the forms the skill actually writes, and must return "" for a bare class name
// rather than guessing — a wrong guess would refuse an honest record.
func TestFamilyOfRecognisesTheWrittenFormsAndGuessesAtNothing(t *testing.T) {
	for in, want := range map[string]string{
		"H · latent defect":           "H",
		"A - dead-on-arrival":         "A",
		"E: single-impl interface":    "E",
		"D duplicated implementation": "D",
		"dead-on-arrival":             "", // bare class: not reconcilable, must not be guessed
		"hallucinated-api":            "",
		"I · out of range":            "", // only A-H are families
		"Z · nope":                    "",
		"":                            "",
		"H":                           "", // too short to carry a separator
	} {
		if got := familyOf(in); got != want {
			t.Errorf("familyOf(%q) = %q, want %q", in, got, want)
		}
	}
}

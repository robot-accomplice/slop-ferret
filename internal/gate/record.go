package gate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// A sha reaches a FILENAME, so it must look like an object id. The review turned the traversal
// below into overwriting ~/.claude/settings.json by creating a branch named `settings`: the only
// guard was `git cat-file -e <sha>^{commit}`, which a branch satisfies.
var objectID = regexp.MustCompile(`^[0-9a-f]{7,64}$`)

// A key becomes a directory under the records root. Anything that is not a plain safe segment is
// replaced, because the key comes from the AUDITED repository's origin URL and this tool exists to
// be pointed at repositories you have reason to distrust.
var unsafeKeySegment = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// RecordSchema is the current record format. Bump it whenever a field is renamed or its meaning
// changes, and give ListRecords a branch for the old shape — the alternative is what happened at
// schema 0: unknown keys unmarshalled to zero and the store silently lost every prior figure.
const RecordSchema = 1

// ErrLegacyRecord marks a record written before the schema field existed. It is reported, never
// rendered as blanks: a record whose figures cannot be read is not a record with no figures.
type ErrLegacyRecord struct {
	Path string
}

func (e *ErrLegacyRecord) Error() string {
	return fmt.Sprintf("%s predates the record schema (written when the fields were "+
		"`coverage_repo`/`status`) and cannot be read as current. Its figures are NOT blank — they "+ // staleprose:allow
		"are unreadable by this binary. Re-sweep the repository, or read the file directly", e.Path)
}

// A Record is one sweep of one repository at one commit.
//
// TWO KINDS OF FIELD, and the split is the point. The COMPUTED half the tool derives itself. The
// ATTESTED half comes from the discharge, because it is judgement — which classes were checked
// clean and BY WHAT METHOD, what was refuted before filing, where the report went. The tool records
// the second kind; it never invents it.
//
// The next sweep reads this to avoid re-spending budget on ground already covered. That is only
// safe if the method is recorded alongside the class: "clean" with no method is not checkable, and
// an unchecked clean is how a later sweep skips ground nobody actually covered.
type Record struct {
	// Schema is the record format's own version, and it exists because renaming the fields once
	// already destroyed information silently. `coverage_repo`/`status` became  staleprose:allow
	// `attested_repo`/`accounting` with no version and no migration, so every record written before
	// that change unmarshalled to zero values and `ferret records` printed a real prior sweep as
	// `stated-read <blank>  plan <blank>` with no accounting at all. The `open` marker that says DO
	// NOT TRUST THIS evaporated in the change that was supposed to make claims more honest.
	//
	// A record is durable input to a future sweep. Reading one whose shape you do not recognise and
	// rendering the gaps as blanks is the same defect as reporting an unperformed audit: absence
	// displayed as a value.
	Schema int `json:"schema"`
	// Origin and RootCommit are the repository's PROVENANCE, recorded so a reader can tell what
	// this record describes. RootCommit is also the key's input, so a record whose recorded root
	// disagrees with the repo being listed is refused rather than shown.
	Origin         string `json:"origin,omitempty"`
	RootCommit     string `json:"root_commit,omitempty"`
	IdentityMethod string `json:"identity_method,omitempty"`
	SHA            string `json:"sha"`
	Date           string `json:"date"`
	AttestedRepo   string `json:"attested_repo"`
	AttestedPlan   string `json:"attested_plan"`
	Denominator    int    `json:"denominator"`
	Waived         int    `json:"waived"`
	WorklistSize   int    `json:"worklist_size"`
	UnmatchedSize  int    `json:"unmatched_size"`
	Accounting     string `json:"accounting"`

	// VocabProvenance is where the H vocabulary came from and how much of it loaded, carried from the
	// plan. Without it, a sweep over a half-loaded lexicon records byte-identical to one over a repo
	// that genuinely matched nothing — the failure and the clean result look the same in recorded
	// state, which is the seam most likely to break (an unversioned markdown file in another tool's
	// config tree).
	VocabProvenance map[string]string `json:"vocab_provenance,omitempty"`

	Tier              string         `json:"tier,omitempty"`
	FamiliesNotRun    []string       `json:"families_not_run,omitempty"`
	CheckedClean      []CheckedClean `json:"checked_clean,omitempty"`
	NearMisses        []string       `json:"near_misses,omitempty"`
	FindingsVerified  int            `json:"findings_verified,omitempty"`
	FindingsSuspected int            `json:"findings_suspected,omitempty"`
	ReportPath        string         `json:"report_path,omitempty"`
}

// CheckedClean is a class recorded clean together with the method used. The method is not optional
// decoration: without it a reader cannot check the claim, and the whole value of a prior record is
// that the next sweep can trust it enough not to repeat the work.
type CheckedClean struct {
	Class  string `json:"class"`
	Method string `json:"method"`
}

func recordsRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".slop-ferret", "records"), nil
}

// RepoKey derives the records-store key for a repository, from ITS ROOT COMMITS.
//
// It used to key on `git remote get-url origin`, and that was wrong in both directions (ABORT II,
// A2):
//
//  1. NOT STABLE. `origin` is configuration, so it differs between checkouts of one repository.
//     The operator's own store held both `Users/jmachen/code/slop-ferret/` and
//     `github.com/robot-accomplice/slop-ferret/` — one repo, two keys, two disjoint histories, so
//     the second sweep could not see the first and the store silently failed at its only job.
//  2. TARGET-ASSERTED. `origin` is an unauthenticated string the AUDITED repository controls, and
//     this tool exists to be pointed at repositories you have reason to distrust. Any repo could
//     set it to a victim's URL and write `checked_clean` claims into that victim's directory —
//     claims SKILL.md Step 0.2 then tells the next sweep not to re-spend budget on. The
//     `cat-file -e` check bound nothing, because a fork contains the upstream's commits anyway.
//
// A root commit cannot be asserted by configuration: you either have that history or you do not.
// It is also invariant under moving, renaming, re-cloning and re-homing the remote, which is what
// the origin URL was reaching for and failed to provide.
//
// The observed origin is recorded INSIDE the record for display, never used to place it. That
// costs directory readability — the store is keyed by hash now — and buys an identity that is
// stable and unforgeable. `ferret records` prints the origin, so the readable name is still there
// where a human actually reads it.
func RepoKey(repo string) (string, error) {
	id, _, err := repoIdentity(repo)
	return id, err
}

// repoIdentity returns the key and how it was derived. The method is recorded because the fallback
// is genuinely weaker and a reader must be able to tell which one they are looking at.
func repoIdentity(repo string) (key, method string, err error) {
	// --reverse for a deterministic order: a repository with several root commits (a merged
	// history) must not key differently depending on git's traversal.
	if roots, e := gitLines(repo, "rev-list", "--max-parents=0", "--reverse", "HEAD"); e == nil && len(roots) > 0 {
		// A shallow clone's "root" is a graft boundary that moves when the depth changes, so it is
		// not an identity. Fall through rather than mint a key that changes under the caller.
		if sh, e := gitLines(repo, "rev-parse", "--is-shallow-repository"); e != nil ||
			len(sh) == 0 || strings.TrimSpace(sh[0]) != "true" {
			s := sha256.Sum256([]byte(strings.Join(roots, "\n")))
			return "root-" + hex.EncodeToString(s[:])[:12], "root-commit", nil
		}
	}
	abs, err := filepath.Abs(repo)
	if err != nil {
		return "", "", err
	}
	s := sha256.Sum256([]byte(abs))
	return "path-" + hex.EncodeToString(s[:])[:12], "absolute-path", nil
}

// originURL is recorded for display only. It is deliberately NOT part of the key: see RepoKey.
func originURL(repo string) string {
	out, err := gitLines(repo, "remote", "get-url", "origin")
	if err != nil || len(out) == 0 {
		return ""
	}
	u := strings.TrimSuffix(out[0], ".git")
	if strings.HasPrefix(u, "git@") {
		u = strings.Replace(strings.TrimPrefix(u, "git@"), ":", "/", 1)
	}
	return strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://")
}

// safeKey reduces a key to path segments that cannot escape. `..` and empty segments are dropped
// outright rather than sanitised into something adjacent, and every remaining character outside
// [A-Za-z0-9._-] becomes `_`.
//
// filepath.Join CLEANS AFTER JOINING, so `filepath.Join(root, "../../x")` resolves outside root
// without complaint. That is the whole mechanism of ABORT finding S1, and it is why the caller
// re-checks containment as well: this function is the belt, that check is the braces.
func safeKey(key string) string {
	var out []string
	normalised := strings.ReplaceAll(filepath.ToSlash(key), "\\", "/")
	for _, seg := range strings.Split(normalised, "/") {
		seg = strings.TrimSpace(seg)
		if seg == "" || seg == "." || seg == ".." {
			continue
		}
		out = append(out, unsafeKeySegment.ReplaceAllString(seg, "_"))
	}
	if len(out) == 0 {
		return "unknown"
	}
	return strings.Join(out, "/")
}

// WriteRecord persists one sweep and returns the path written.
//
// It REFUSES a sha that does not resolve in the target. A boundary nobody can resolve makes the
// next sweep unable to scope itself, which is how a whole-repo re-read gets spent re-covering
// ground — and two prior sweeps recorded exactly that, both taken from dirty maps whose composite
// shas evaporated when the commits were amended away.
func WriteRecord(repo string, pl *Plan, dis *Discharge, res *Result) (string, error) {
	// A record is durable input to a FUTURE sweep: SKILL.md tells the next run to read
	// `checked_clean` and not re-spend budget on those classes. A sweep that did not settle has not
	// established anything, so persisting its claims is the persistence layer converting an
	// unperformed audit into a completed-looking one — the exact invariant this tool defends.
	// (ABORT I1, the review's designated "one fix".)
	if res.Accounting != "complete" {
		return "", fmt.Errorf("this sweep has an incomplete accounting (%q) — refusing to record claims "+
			"a later sweep would skip ground on. Close the remaining items first, or re-run with "+
			"--no-record", res.Accounting)
	}
	if !objectID.MatchString(pl.SHA) {
		return "", fmt.Errorf("sha %q is not an object id — refusing to use it as a filename", pl.SHA)
	}
	if _, err := gitLines(repo, "cat-file", "-e", pl.SHA+"^{commit}"); err != nil {
		return "", fmt.Errorf("sha %q does not resolve in %s — refusing to record a boundary "+
			"nobody can re-derive", pl.SHA, repo)
	}
	key, err := RepoKey(repo)
	if err != nil {
		return "", err
	}
	root, err := recordsRoot()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, filepath.FromSlash(safeKey(key)))
	// Belt and braces: re-derive the relationship after Join has cleaned the path, because Join
	// cleans AFTER joining and that is exactly how the escape worked.
	if rel, err := filepath.Rel(root, dir); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("refusing to write outside the records root: key %q resolves to %s", key, dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	date := ""
	if lines, err := gitLines(repo, "show", "-s", "--format=%cs", pl.SHA); err == nil && len(lines) > 0 {
		date = lines[0]
	}

	// A class recorded clean WITHOUT a method is not checkable, and an unchecked clean is how a
	// later sweep skips ground nobody covered. Drop them rather than persist an unfalsifiable claim.
	// A CLASS CANNOT BE BOTH CHECKED CLEAN AND NOT RUN. A record carrying both is not merely
	// untidy: SKILL.md Step 0.2 tells the next sweep to trust `checked_clean` and not re-spend
	// budget there, so the contradiction converts a family nobody ran into a family nobody will
	// ever run. Refuse rather than record, because there is no safe reading of it.
	notRun := map[string]bool{}
	for _, f := range dis.FamiliesNotRun {
		notRun[strings.ToUpper(strings.TrimSpace(f))] = true
	}
	for _, c := range dis.CheckedClean {
		fam := strings.ToUpper(strings.TrimSpace(familyOf(c.Class)))
		if fam != "" && notRun[fam] {
			return "", fmt.Errorf("the discharge lists family %s in `families_not_run` AND records "+
				"%q as checked clean. A family that did not run cannot have been checked; the next "+
				"sweep is told to skip whatever appears here, so recording both would retire "+
				"ground nobody covered", fam, c.Class)
		}
	}

	clean := make([]CheckedClean, 0, len(dis.CheckedClean))
	for _, c := range dis.CheckedClean {
		if strings.TrimSpace(c.Class) != "" && CheckableMethod(c.Method) {
			clean = append(clean, c)
		}
	}

	// RootCommit is stamped ONLY when it is the thing the key was derived from. A shallow clone's
	// "root" is a graft boundary that moves with --depth, and repoIdentity already refuses it as a
	// key -- stamping it here anyway made ListRecords' cross-check reject the repository's own
	// records as "a different history" after any `git fetch --deepen`. The guard added to dodge the
	// moving boundary was defeated by a field that did not get the same treatment.
	_, method, _ := repoIdentity(repo)
	var roots []string
	if method == "root-commit" {
		roots, _ = gitLines(repo, "rev-list", "--max-parents=0", "--reverse", "HEAD")
	}
	rec := Record{
		Schema: RecordSchema,
		Origin: originURL(repo), RootCommit: strings.Join(roots, ","), IdentityMethod: method,
		SHA: pl.SHA, Date: date,
		AttestedRepo: res.Attested.Repo, AttestedPlan: res.Attested.Plan,
		Denominator: pl.ProductionTotal, Waived: res.Attested.Waived,
		WorklistSize: len(pl.HWorklist), UnmatchedSize: len(pl.HUnmatched),
		Accounting:      res.Accounting,
		VocabProvenance: pl.VocabProvenance,

		Tier: dis.Tier, FamiliesNotRun: dis.FamiliesNotRun,
		CheckedClean: clean, NearMisses: dis.NearMisses,
		FindingsVerified: dis.FindingsVerified, FindingsSuspected: dis.FindingsSuspected,
		ReportPath: dis.ReportPath,
	}
	b, err := json.MarshalIndent(rec, "", " ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, pl.SHA+".json")
	return path, os.WriteFile(path, b, 0o644)
}

// ListRecords returns prior sweeps of this repository, newest first. A repository that has never
// been swept returns nothing and no error: that is a normal state, and reporting it as a failure
// would make the Step 0.2 read noisy for exactly the repos where it matters least.
func ListRecords(repo string) ([]Record, error) {
	key, err := RepoKey(repo)
	if err != nil {
		return nil, err
	}
	root, err := recordsRoot()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(root, filepath.FromSlash(key))
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	// A repository that has never been swept returns nothing and no error -- a normal state. But
	// the "no records here" path must still run the legacy-location check below, or a store that
	// HAS records simply looks empty.
	wantRootLines, _ := gitLines(repo, "rev-list", "--max-parents=0", "--reverse", "HEAD")
	wantRoot := strings.Join(wantRootLines, ",")
	var out []Record
	var legacy []error
	// The key moved from origin-derived to root-commit-derived, so records written by an older
	// binary sit in a directory nothing looks in any more. Returning nothing while five records
	// exist on disk is the same "absence rendered as a value" defect this store was just fixed
	// for -- so the old location is checked and REPORTED, exactly once, rather than left silent.
	// Both shapes the previous binary could have written: origin-derived for a repo with a remote,
	// and `path-<sha256(abs)[:8]>` for one without. The first version of this check covered only
	// the first, so a REMOTELESS repo with records on disk still listed as empty with no error --
	// the exact defect the check was added to close, still live in it.
	legacyKeys := []string{}
	if o := originURL(repo); o != "" {
		legacyKeys = append(legacyKeys, safeKey(o))
	}
	if abs, err := filepath.Abs(repo); err == nil {
		h := sha256.Sum256([]byte(abs))
		legacyKeys = append(legacyKeys, "path-"+hex.EncodeToString(h[:])[:8])
	}
	for _, lk := range legacyKeys {
		if old := filepath.Join(root, filepath.FromSlash(lk)); old != dir {
			if ents, err := os.ReadDir(old); err == nil && len(ents) > 0 {
				legacy = append(legacy, fmt.Errorf("%d record(s) from an older binary are at %s, "+
					"keyed on the origin URL. The key is derived from the repository's root "+
					"commits now, because an origin is configuration and could be asserted by the "+
					"repo being audited. They are not readable as current records; re-sweep, or "+
					"read them directly", len(ents), old))
			}
		}
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var r Record
		if json.Unmarshal(b, &r) != nil {
			continue
		}
		// A record from before the rename unmarshals cleanly and carries nothing. Reporting that as
		// a row of blanks is how the store lost four real campaign records in silence; the caller
		// gets told instead.
		if r.Schema != RecordSchema {
			legacy = append(legacy, &ErrLegacyRecord{Path: filepath.Join(dir, e.Name())})
			continue
		}
		// The key is derived from the root commits, so a record sitting under this key whose
		// recorded root disagrees did not come from this repository. Belt and braces against a
		// hand-placed or migrated file: the directory says one thing, the record says another, and
		// the safe reading of a contradiction is to trust neither.
		if r.RootCommit != "" && wantRoot != "" && r.RootCommit != wantRoot {
			legacy = append(legacy, fmt.Errorf("%s claims root commit %s but this repository's is "+
				"%s — refusing to report a record that describes a different history",
				filepath.Join(dir, e.Name()), r.RootCommit, wantRoot))
			continue
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date > out[j].Date })
	return out, errors.Join(legacy...)
}

// nonMethods are strings that occupy the method field without being a method. A reader cannot check
// "-", and the whole value of a recorded clean is that the NEXT sweep can check it and skip work.
var nonMethods = map[string]bool{"-": true, "--": true, "n/a": true, "na": true, "none": true,
	"tbd": true, "todo": true, "?": true, "x": true}

// CheckableMethod reports whether a checked-clean method is falsifiable text.
//
// It is exported and lives here so that the RECORD and the REPORT apply one rule. They did not:
// WriteRecord dropped blank methods while the renderer copied them verbatim, so the page a human
// reads asserted classes clean with an empty "Method used" cell, under a heading whose premise is
// that the method is not optional. The durable artifact was stricter than the visible one, which is
// the wrong way round — a human can act on what they see.
func CheckableMethod(s string) bool {
	t := strings.ToLower(strings.TrimSpace(s))
	return t != "" && !nonMethods[t]
}

// familyOf extracts the family letter from a checked-clean class label. The skill writes them as
// "H · latent defect" or "A - dead-on-arrival", and it also writes bare class names like
// "dead-on-arrival"; only the first form can be reconciled against families_not_run, so the bare
// form returns "" and is left alone rather than guessed at.
func familyOf(class string) string {
	t := strings.TrimSpace(class)
	if len(t) < 2 {
		return ""
	}
	if c := t[0]; c >= 'A' && c <= 'H' {
		switch t[1] {
		case ' ', '\t', ':', '-', 0xC2: // 0xC2 leads the UTF-8 for '·'
			return string(c)
		}
	}
	return ""
}

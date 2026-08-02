package gate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	SHA           string `json:"sha"`
	Date          string `json:"date"`
	CoverageRepo  string `json:"coverage_repo"`
	CoveragePlan  string `json:"coverage_plan"`
	Denominator   int    `json:"denominator"`
	Waived        int    `json:"waived"`
	WorklistSize  int    `json:"worklist_size"`
	UnmatchedSize int    `json:"unmatched_size"`
	Status        string `json:"status"`

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

// RepoKey identifies a repository stably across checkouts. The result is SAFE BY CONSTRUCTION —
// it is reduced to plain path segments here rather than at the call site, because it is exported
// and derived from the AUDITED repository's origin URL. The origin URL is preferred because a
// path changes when the tree moves and the records would then look like a different repo's; the
// hash fallback keeps remoteless repos usable rather than unrecordable.
func RepoKey(repo string) (string, error) {
	out, err := gitLines(repo, "remote", "get-url", "origin")
	if err == nil && len(out) > 0 {
		u := strings.TrimSuffix(out[0], ".git")
		if strings.HasPrefix(u, "git@") {
			u = strings.Replace(strings.TrimPrefix(u, "git@"), ":", "/", 1)
		}
		u = strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://")
		if u != "" {
			return safeKey(u), nil
		}
	}
	abs, err := filepath.Abs(repo)
	if err != nil {
		return "", err
	}
	s := sha256.Sum256([]byte(abs))
	return safeKey("path-" + hex.EncodeToString(s[:])[:8]), nil
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
	if res.Status != "settled" {
		return "", fmt.Errorf("this sweep did not settle (status %q) — refusing to record claims "+
			"a later sweep would skip ground on. Close the remaining items first, or re-run with "+
			"--no-record", res.Status)
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
	clean := make([]CheckedClean, 0, len(dis.CheckedClean))
	for _, c := range dis.CheckedClean {
		if strings.TrimSpace(c.Class) != "" && strings.TrimSpace(c.Method) != "" {
			clean = append(clean, c)
		}
	}

	rec := Record{
		SHA: pl.SHA, Date: date,
		CoverageRepo: res.Coverage.Repo, CoveragePlan: res.Coverage.Plan,
		Denominator: pl.ProductionTotal, Waived: res.Coverage.Waived,
		WorklistSize: len(pl.HWorklist), UnmatchedSize: len(pl.HUnmatched),
		Status: res.Status,

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
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Record
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var r Record
		if json.Unmarshal(b, &r) == nil {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date > out[j].Date })
	return out, nil
}

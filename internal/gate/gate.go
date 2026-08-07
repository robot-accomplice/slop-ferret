// Package gate is the coded seam between the magma code map and a slop sweep.
//
// Ported from python/gate.py on 2026-08-01 and verified differentially against it: both
// implementations were run over a real repository and their plan output compared field by field
// before the Python was deleted. The measured constants below came from real repositories and are
// carried across unchanged — a port is the easiest place to quietly lose a measurement.
//
//	ferret plan   <magma-map-dir> <pinned-sha> <repo> [--since <ref>]  > plan.json
//	ferret enumerate <plan.json> <discharge.json>          ; 0 accounted, 3 items open, 4 refused
//
// THIS IS A TOOL FOR THE PERSON RUNNING THE SWEEP. It is not an evaluation of them and not a
// compliance mechanism: nobody is graded by its output, there is no adversary to design against,
// and its job is to hand back a work queue and an honest instrument reading.
//
// `plan` reads magma's per-row JSON, refuses unless it is the right tree (sha) and a shape it can
// parse (contract_version), turns map rows into per-family candidates carrying each class's
// pre-filing bar, and enumerates BOTH the signal-matched family-H worklist AND its complement —
// every other production source file.
//
// `verify` reports TWO FRACTIONS and no verdict word:
//
//	attested.repo  production source files read / total    <- "was the repo covered"
//	attested.plan  items dispositioned / items raised      <- "was the plan worked through"
//
// COMPLETE/PARTIAL/INCOMPLETE were removed because one token cannot carry two quantities.
// Measured on ghola @4f33b3c: 10/10 on the plan, 17/25 on the repo, reported COMPLETE — while the
// enumeration had never named internal/bridge/bridge.go, an unauthenticated localhost HTTP server
// making arbitrary outbound fetches, which a hand read turned into the sweep's worst finding.
// Coverage of the enumeration was being read as coverage of the repository.
//
// WHAT THIS ESTABLISHES AND WHAT IT DOES NOT. It checks that the sweep ACCOUNTED for every item
// the plan raised. It cannot establish that a file was read: read_paths is self-reported and
// nothing here corroborates it. Attestation is still worth requiring — it makes an omission a
// statement someone made rather than a gap nobody owns.
package gate

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	mapSubdir    = ".magma"
	planContract = "slop-gate/2"
	// minSHAAbbrev is the shortest git object name shaMatches will treat as a real abbreviation of a
	// different-length name. git's default abbreviation and what magma 0.2.0 stamps are both 7; below
	// this a truncated or garbage name could prefix-match an unrelated tree.
	minSHAAbbrev = 7
)

var supportedContracts = map[string]bool{"codemap-rows/1": true}

// Absence means the map cannot seed anything and the gate must refuse.
var mapFilesRequired = []string{"_dead.json", "_test-only.json"}

// Absence must DEGRADE COVERAGE HONESTLY, never fail closed and never pass silently. magma does
// not emit these yet.
//
//	_interfaces.json  approved and tractable; seeds family E when it lands.
//	_duplicates.json  deliberately NOT built. magma has no notion of similarity, and a duplicate
//	                  row that is not a duplicate is a refactor order for code that should be left
//	                  alone — the same harm as a false dead-code row being a deletion order.
//
// The families they would seed are reported as NOT SEEDED, which is what stops a missing file
// reading as a clean family.
var mapFilesOptional = map[string]string{"_interfaces.json": "E", "_duplicates.json": "D"}

// A signal must match at a path start, a SEGMENT start, or after a word separator inside a
// filename. The original anchor was `(^|/)` alone, so internal/db/user_store.go was MISSED even
// though `store` is in the persistence vocabulary. Measured on roboticus: relaxing it adds 81
// files to a 285-path worklist.
const anchor = `(^|/|[_.\-])`

type signal struct {
	reason string
	rx     *regexp.Regexp
}

// LexiconPath is where the vocabulary lives: the lexicon in the DEPLOYED SKILL, not a table
// compiled into this binary.
//
// THE SIGNALS ARE PART OF THE LEXICON. The lexicon's tables define what a class IS; the signals
// define where that class tends to LIVE. Both are the method's domain language, both are guesses
// that improve by use, and both are now covered by the lexicon's own `version:` — which the signals
// were not when they sat in a file of their own, with no version at all.
//
// Being prose, they iterate far faster than code. Compiling them in meant a word learned from one
// sweep needed a binary release to reach the next, which is the coupling the skill/binary split
// exists to remove. Now: add a word to the lexicon, reinstall the skill, done.
//
// It is A HINT, NOT A COMPLETENESS SIGNAL. Measured across five real repositories on 2026-08-02:
// 59% label precision, 20% of production files matched, 0-of-6 recall on the files that actually
// produced findings. What makes the miss safe is not the vocabulary's quality — it is that a file
// no signal reaches is reported as UNREAD rather than as clean.
var LexiconPath = func() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "skills", "slop-ferret", "references", "ai-slop-lexicon.md")
}

// Tier 1 is the blast-radius set; tier 2 is the volume set. A SEMANTIC split, not a top-N cap: a
// numeric cap is a magic number nobody can defend and would silently drop whatever sorted last.
var hTier1 = map[string]bool{"money/value": true, "crypto/signing": true, "auth/session": true,
	"consensus/ordering": true, "arithmetic/overflow": true}
var hTier2 = map[string]bool{"migration": true, "persistence/state": true,
	"untrusted-parse": true, "network/untrusted-io": true}

// Worklists at or below this are required in full: deferral exists to make a LARGE worklist
// tractable, and below the size where a full H read is feasible in one sweep it buys nothing and
// costs coverage. A judgement, stated as one. Effect on the repos to hand: ghola 10 and counterspy
// 35 required in full; roboticus 475 splits 185/290.
const hDeferFloor = 60

// Test files and prose docs are not H-read targets. VENDORED/GENERATED trees added 2026-08-01:
// SKILL.md Step 1 always mandated excluding them and this pattern never did — a nuisance while it
// only filtered the worklist, load-bearing once it also defines the coverage denominator.
var notH = regexp.MustCompile(`(?i)(_test\.|\.test\.|(^|/)tests?[_.]|(^|/)test_|(^|/)tests?/|` +
	`(^|/)testdata/|(^|/)benches?/|(^|/)examples?/|(^|/)fuzz/|_spec\.|\.spec\.|` +
	`(^|/)vendor/|(^|/)vendored/|(^|/)third_party/|(^|/)node_modules/|` +
	`(^|/)\.venv/|(^|/)dist/|(^|/)generated/|\.pb\.go$|_pb2\.py$|` +
	`\.generated\.|\.min\.js$` +
	`|\.(md|markdown|rst|txt|json|ya?ml|toml|lock|sum|mod)$)`)

// The coverage denominator is an ALLOWLIST, deliberately, and the choice is the opposite of the
// one made for H signals. An H signal guesses semantics from a name the target's authors chose,
// which is why it under-enumerates silently. A file extension is a language-level fact, and a
// language this list omits does not shrink the denominator quietly — it lands in
// production_unclassified, which the plan prints. Absence announces itself.
var sourceExt = regexp.MustCompile(`(?i)\.(go|rs|ts|tsx|js|jsx|mjs|py|rb|java|kt|kts|cs|swift|m|mm|` +
	`c|cc|cpp|cxx|h|hpp|sh|bash|zsh|sol|tf|php|scala|clj|ex|exs|erl|hs|ml|lua|pl|r|dart|vue|svelte)$`)

// What each map-seeded class must clear before a candidate becomes a filed finding.
var bars = map[string]string{
	"dead-on-arrival": "universal-negative refuter: prove nothing reaches it (reflection/init/" +
		"codegen/FFI checked), not just that the count is 0",
	"test-only": "confirm no production caller AND that the symbol is not an entry point invoked " +
		"from outside the repo",
	"duplicated-impl": "confirm the copies are semantically identical AND that no test pins them " +
		"to each other",
	"single-impl-interface": "apply the discriminator: a consumer-declared narrow port is NOT " +
		"over-abstraction; only a producer-side single-impl interface is",
}

// Heavier bar when the map's fidelity is weaker than a real call graph.
//
// THIS TABLE IS THE PRODUCER'S VOCABULARY, NOT A GUESS AT IT. magma emits exactly two values —
// `rta` (Go) and `semantic` (Rust) — documented in its README's fidelity table. This gate carried
// `reachability`, `exports`, `heuristic` and `rustc-dead_code`: four keys magma never emits, and
// no `semantic`. So every Rust candidate was labelled "UNRECOGNISED fidelity … treated as the
// weakest", which is exactly the failure magma's own README warns about by name. The guarding test
// used the fictional value "quantum-vibes" — a fixture magma never produces, asserting a true but
// irrelevant property — the same shape as the dirty-map test that certified a bug.
// TestEveryFidelityRealMagmaEmitsHasABar now pins this against magma's actual output.
var fidelityBar = map[string]string{
	"rta":      "", // Go: Rapid Type Analysis — a real call graph
	"semantic": "", // Rust: rust-analyzer name resolution + type inference — a real call graph
}

// Exit codes. These are a CONTRACT with whatever script wraps this tool, so they are named rather
// than spelled inline. 3 previously meant both "items still open" and "the tool refused": a caller
// could not tell an unfinished sweep from a map of the wrong tree, and those want opposite
// responses -- one says read the work queue, the other says nothing was measured. 4 was free, being
// the retired PARTIAL verdict's code.
const (
	ExitOK        = 0 // nothing raised is undispositioned
	ExitMisuse    = 2 // wrong arity, unreadable file
	ExitItemsOpen = 3 // the sweep is not finished; read `remaining`
	ExitRefused   = 4 // the tool declined to run: wrong tree, unknown contract, missing map
)

// Err carries an exit code so the CLI can distinguish a refusal from misuse.
type Err struct {
	Msg  string
	Code int
}

func (e *Err) Error() string { return e.Msg }

func die(code int, format string, a ...any) error {
	return &Err{Msg: fmt.Sprintf(format, a...), Code: code}
}

type WorkItem struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type Candidate struct {
	Family string `json:"family"`
	Class  string `json:"class"`
	Bar    string `json:"bar"`
	Symbol string `json:"symbol"`
	File   string `json:"file"`
	Line   int    `json:"line"`
}

type Plan struct {
	Contract               string            `json:"contract"`
	SHA                    string            `json:"sha"`
	Fidelity               string            `json:"fidelity"`
	ReachabilityComputable bool              `json:"reachability_computable"`
	MapProvenance          map[string]string `json:"map_provenance"`
	// VocabProvenance records where the H vocabulary came from and how much of it loaded. Without
	// it, a sweep run against a half-loaded lexicon leaves a plan and a record byte-identical to
	// one over a repo whose files genuinely matched nothing — the failure and the clean result look
	// the same, which makes root-causing from recorded state impossible for the seam most likely to
	// break (an unversioned markdown file in another tool's config tree).
	VocabProvenance        map[string]string `json:"vocab_provenance"`
	NotComputableReason    string            `json:"not_computable_reason,omitempty"`
	MapLimitations         []string          `json:"map_limitations,omitempty"`
	UnseededFamilies       []string          `json:"unseeded_families"`
	UnseededDetail         map[string]string `json:"unseeded_detail"`
	Candidates             []Candidate       `json:"candidates"`
	ProductionTotal        int               `json:"production_total"`
	ProductionFiles        []string          `json:"production_files"`
	ProductionUnclassified []string          `json:"production_unclassified"`
	// ProductionExcluded is source-extension files the denylist dropped (build output, tests,
	// vendored code) — announced, not silently removed, so a hand-written source file in an
	// unconventional path (JS under dist/) can be caught rather than vanishing from the denominator.
	ProductionExcluded []string   `json:"production_excluded"`
	HWorklist          []WorkItem `json:"h_worklist"`
	HRequired          []WorkItem `json:"h_required"`
	HDeferred          []WorkItem `json:"h_deferred"`
	HUnmatched         []string   `json:"h_unmatched"`
	HUnmatchedChanges  []WorkItem `json:"h_unmatched_changes"`
	ChangeBaseline     string     `json:"change_baseline"`
	Instructions       string     `json:"instructions"`
}

type rowDoc struct {
	ContractVersion        string `json:"contract_version"`
	Generator              string `json:"generator"`
	SHA                    string `json:"sha"`
	Fidelity               string `json:"fidelity"`
	Tree                   string `json:"tree"`
	ReachabilityComputable bool   `json:"reachability_computable"`
	NotComputableReason    string `json:"not_computable_reason"`
	// Limitations rides on every note. magma's contract names THIS gate as the consumer it exists
	// for: "The audit gate weights a candidate by how far to trust the map." It was parsed away
	// entirely, so a dead-on-arrival candidate shipped with a declared, machine-readable
	// false-positive mechanism (e.g. go-closure-edges: a function called only through a closure can
	// appear unreachable) while the bar listed reflection/init/codegen/FFI and not closures.
	Limitations []struct {
		ID          string `json:"id"`
		Effect      string `json:"effect"`
		Description string `json:"description"`
	} `json:"limitations"`
	Rows []struct {
		Symbol string `json:"symbol"`
		File   string `json:"file"`
		Line   int    `json:"line"`
	} `json:"rows"`
	Clusters []struct {
		Members []struct {
			Symbol string `json:"symbol"`
			File   string `json:"file"`
			Line   int    `json:"line"`
		} `json:"members"`
	} `json:"clusters"`
}

// loadSignals compiles the H vocabulary, and FAILS LOUD in three places that used to fail silent.
//
//  1. An unreadable or fence-less lexicon returned nil, so `ferret plan` exited 0 with an empty
//     worklist and said nothing about a lexicon anywhere in plan.json. That is the default state of
//     a `go install`ed binary before `ferret install` has ever run. The complaint surfaced two
//     steps later at `enumerate` and named the WRONG remedy — "extend the signals via
//     `.slop-h-signals`" — sending the operator to write regexes into the target repo when the
//     cause was a missing skill.
//  2. A signal that failed to compile was `continue`d. One unbalanced paren in the lexicon moved
//     `internal/auth/session.go` out of the required tier and into the cheap-to-waive complement,
//     with exit 0 and no warning, while the sweep still recorded family H checked clean.
//  3. Nothing counted what loaded, so a sweep run with a half-loaded vocabulary left a record
//     byte-indistinguishable from a repo whose files genuinely matched nothing.
func loadSignals(repo string) ([]signal, int, error) {
	src := [][2]string{}
	lexicon := LexiconPath()
	if lexicon != "" {
		src = append(src, parseLexiconSignals(lexicon)...)
	}
	fromLexicon := len(src)
	// Path-based H enumeration is vocabulary-bound; a project whose domain terms are missing must
	// be able to add them rather than silently get a short worklist.
	repoSignals, err := parseSignalFile(filepath.Join(repo, ".slop-h-signals"))
	if err != nil {
		return nil, 0, err
	}
	src = append(src, repoSignals...)

	out := make([]signal, 0, len(src))
	for _, p := range src {
		rx, err := regexp.Compile(`(?i)` + anchor + `(` + p[1] + `)`)
		if err != nil {
			return nil, 0, die(ExitRefused, "signal %q does not compile: %v\n\nThe H vocabulary is "+
				"the blast-radius tier. Dropping a signal that will not compile silently demotes "+
				"every path it would have matched to the cheap-to-waive complement, and the sweep "+
				"still reports family H covered. Fix the pattern in %s or in %s",
				p[0], err, lexicon, filepath.Join(repo, ".slop-h-signals"))
		}
		out = append(out, signal{reason: p[0], rx: rx})
	}
	if len(out) == 0 {
		return nil, 0, die(ExitRefused, "the H vocabulary is EMPTY — no signals loaded from %s.\n\n"+
			"A sweep with no vocabulary enumerates nothing and produces a report indistinguishable "+
			"from a clean one. This is what an uninstalled skill looks like, not a clean repo.\n"+
			"Run `ferret install` to deploy the skill, then `ferret doctor` to confirm it.",
			lexiconOrNone(lexicon))
	}
	return out, fromLexicon, nil
}

func lexiconOrNone(p string) string {
	if p == "" {
		return "(no lexicon path could be resolved — is HOME set?)"
	}
	return p
}

func gitLines(repo string, args ...string) ([]string, error) {
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		msg := ""
		if ee, ok := err.(*exec.ExitError); ok {
			msg = strings.TrimSpace(string(ee.Stderr))
		}
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return nil, die(ExitMisuse, "git %s failed in %s: %s", strings.Join(args, " "), repo, msg)
	}
	var lines []string
	for _, l := range strings.Split(strings.ReplaceAll(string(out), "\x00", "\n"), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	return lines, nil
}

// ProductionFiles is the coverage universe: every tracked file that is production source.
// `unclassified` is everything that survived the exclusion filter but carries no recognised
// source extension — reported rather than dropped, so an unsupported language cannot shrink the
// denominator without saying so. `excluded` closes the other half of that promise: a file the
// denylist dropped that nonetheless LOOKS like source (a recognised extension under test/vendor/
// dist/generated). Build output and tests belong there and are expected — but so did dr-markdown's
// hand-written `frontend/dist/src/app.js`, the largest file in the release, which vanished from the
// denominator with no trace and made the coverage fraction silently over-count. The excluded set is
// announced so a human can catch a source file in an unconventional path; a doc (`.md`, `.json`)
// dropped by the denylist is not source and is not listed.
func ProductionFiles(repo string) (production, unclassified, excluded []string, err error) {
	files, err := gitLines(repo, "ls-files", "-z")
	if err != nil {
		return nil, nil, nil, err
	}
	for _, f := range files {
		if notH.MatchString(f) {
			if sourceExt.MatchString(f) {
				excluded = append(excluded, f)
			}
			continue
		}
		if sourceExt.MatchString(f) {
			production = append(production, f)
		} else {
			unclassified = append(unclassified, f)
		}
	}
	return production, unclassified, excluded, nil
}

// enumerateWorklist ranks production paths by H signal. A signal match no longer decides whether
// a file exists to the sweep — the complement is enumerated too — it decides reading order.
func enumerateWorklist(production []string, signals []signal) []WorkItem {
	var work []WorkItem
	for _, f := range production {
		for _, s := range signals {
			if s.rx.MatchString(f) {
				work = append(work, WorkItem{Path: f, Reason: s.reason})
				break
			}
		}
	}
	return work
}

// splitWorklist partitions by consequence tier. Two floors, and the second exists because the
// first was too narrow: if NO tier-1 path exists the whole worklist is required, but that is a
// ZERO-check and one incidental tier-1 match defeats it. The size floor is the real rule.
func splitWorklist(work []WorkItem) (required, deferred []WorkItem) {
	if len(work) <= hDeferFloor {
		return append([]WorkItem{}, work...), []WorkItem{}
	}
	for _, w := range work {
		if hTier1[w.Reason] || !hTier2[w.Reason] {
			required = append(required, w)
		}
	}
	if len(required) == 0 {
		return append([]WorkItem{}, work...), []WorkItem{}
	}
	req := map[string]bool{}
	for _, w := range required {
		req[w.Path] = true
	}
	for _, w := range work {
		if !req[w.Path] {
			deferred = append(deferred, w)
		}
	}
	return required, deferred
}

// unmatchedChanges reports production files changed since `since` that no H signal reached. It
// measures the CHANGED SUBSET of the enumeration's blind spots, bounded by the baseline —
// HUnmatched is the unbounded set, and is the one that is enforced.
func unmatchedChanges(repo, since string, signals []signal) ([]WorkItem, error) {
	files, err := gitLines(repo, "diff", "--name-only", since+"..HEAD")
	if err != nil {
		return nil, err
	}
	var holes []WorkItem
	for _, f := range files {
		if notH.MatchString(f) {
			continue
		}
		matched := false
		for _, s := range signals {
			if s.rx.MatchString(f) {
				matched = true
				break
			}
		}
		if !matched {
			holes = append(holes, WorkItem{Path: f, Reason: "changed since " + since +
				" and no H signal reached it — the enumeration did not see this file, so its " +
				"absence from the worklist is not evidence that it is benign"})
		}
	}
	return holes, nil
}

// shaMatches reports whether two git object names refer to the same commit when one may be an
// abbreviation of the other. magma stamps a 7-char abbreviation; a user pins `git rev-parse HEAD`
// (40 chars). Raw string equality refused every full-length pin and prescribed an impossible remedy
// ("regenerate the map at <40-char sha>", which magma never emits), so the loop never terminated.
// Equal-length names must match exactly — that keeps the dirty-tree and same-length refusals
// unchanged. When lengths differ the shorter must be a genuine abbreviation: at least minSHAAbbrev
// chars and a prefix of the longer, so a truncated or garbage name cannot match an unrelated tree.
func shaMatches(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if len(a) == len(b) {
		return a == b
	}
	short, long := a, b
	if len(short) > len(long) {
		short, long = long, short
	}
	if len(short) < minSHAAbbrev {
		return false
	}
	return strings.HasPrefix(long, short)
}

func loadMap(mapdir, pinnedSHA string) (map[string]*rowDoc, map[string]string, error) {
	d := mapdir
	if fi, err := os.Stat(d); err != nil || !fi.IsDir() {
		return nil, nil, die(ExitRefused, "map dir %s does not exist — run magma first", mapdir)
	}
	// Tolerate being handed either the map root or the .magma subdir itself. Row files live under
	// <map>/.magma/, not at the map root; this gate read the root for its whole life and exited 3
	// on every real map.
	if filepath.Base(d) != mapSubdir {
		if fi, err := os.Stat(filepath.Join(d, mapSubdir)); err == nil && fi.IsDir() {
			d = filepath.Join(d, mapSubdir)
		}
	}
	docs := map[string]*rowDoc{}
	unseeded := map[string]string{}
	all := append(append([]string{}, mapFilesRequired...), "_interfaces.json", "_duplicates.json")
	for _, name := range all {
		p := filepath.Join(d, name)
		b, err := os.ReadFile(p)
		if err != nil {
			if fam, ok := mapFilesOptional[name]; ok {
				unseeded[name] = fam
				continue
			}
			return nil, nil, die(ExitRefused, "%s missing from %s — regenerate the map with "+
				"`magma <repo> <name> <vault>`. If magma was itself updated, pass --force: "+
				"freshness is keyed on the ANALYSED repo's sha, not on magma's version, so an "+
				"unchanged repo silently reports 'already fresh' and writes nothing.", name, d)
		}
		var doc rowDoc
		if err := json.Unmarshal(b, &doc); err != nil {
			return nil, nil, die(ExitRefused, "%s is not valid JSON: %v", name, err)
		}
		if !supportedContracts[doc.ContractVersion] {
			return nil, nil, die(ExitRefused, "%s contract_version %q not supported — magma is newer/older "+
				"than this gate; update the gate or pin magma. NOTE there are three magma "+
				"contracts and they are NOT interchangeable: codemap-rows/1 (row files, the only "+
				"one this gate may accept), codemap-graph/1 (graph.json), magma-code-graph/1 "+
				"(the architext emit).", name, doc.ContractVersion)
		}
		if !shaMatches(doc.SHA, pinnedSHA) {
			return nil, nil, die(ExitRefused, "%s sha %q != pinned %q — the map describes a "+
				"different tree than the sweep; regenerate the map at %s.",
				name, doc.SHA, pinnedSHA, pinnedSHA)
		}
		// THE DIRTY-TREE REFUSAL, and it checks `tree` because that is where the marker actually is.
		//
		// This gate compared `sha` and claimed a dirty map "refuses by construction" because magma
		// stamped a composite `<sha>+<diffhash>` there. It does not: measured against real magma on
		// 2026-08-02, a dirty tree yields sha="4f33b3c" (the clean head sha) and
		// tree="4f33b3c-dirty". The comparison therefore passed and a dirty map was ACCEPTED, exit
		// 0 — the guarantee was prose describing behaviour the code lacked, sitting in the gate,
		// about the gate's own headline safety property. Found by sweeping magma.
		//
		// Why it matters: a dirty map reports in-flight, not-yet-wired code as dead, and its
		// boundary is disproportionately likely to evaporate when the commit is amended or rebased
		// away. Two prior sweeps pinned exactly such a boundary and neither resolves today, which
		// is what made their denominators unreproducible.
		//
		// An ABSENT tree field is accepted: an older map simply did not carry one, and absence is
		// not evidence of dirtiness.
		if doc.Tree != "" && doc.Tree != doc.SHA {
			return nil, nil, die(ExitRefused, "%s is a DIRTY-tree map: sha %q but tree %q. Commit "+
				"or stash first, then regenerate. A dirty map reports in-flight code as dead and "+
				"pins a boundary that is likely to evaporate.", name, doc.SHA, doc.Tree)
		}
		docs[name] = &doc
	}
	return docs, unseeded, nil
}

const instructions = "Read every h_required path — that tier is the floor and an unattested one " +
	"leaves an item open (exit 3). h_deferred is tier-2 volume. EVERY path in `h_unmatched` must " +
	"also be attested or waived: those are the production files no signal reached, and the " +
	"enumeration's silence about them is not evidence. (family H is found by reading, not the " +
	"map). For each candidate, clear its `bar` before filing. Then write a discharge.json {sha, " +
	"read_paths:[...], families_not_run:[...], coverage_waived:[...], " +
	"candidates_filed:[{file,symbol}], candidates_cleared:[{file,symbol}], " +
	"candidates_refuted:[{file,symbol}]} and run `ferret enumerate`. `coverage_waived` entries may be " +
	"a bare path or {path, reason} — a reason is OPTIONAL. Waiving is cheap on purpose: deciding " +
	"not to read a file is a normal, correct move and should cost nothing. It settles the " +
	"ACCOUNTING and leaves `attested.repo` alone, because a waived file genuinely was not read " +
	"and the fraction is there to tell YOU what you actually looked at. No coverage floor is " +
	"enforced: there is no defensible number, and a red build for reading 67% instead of 90% " +
	"would only teach you to waive to clear it. `sha` must equal this plan's sha. " +
	"`candidates_filed` is what you actually ACCUSED; every entry must also appear in " +
	"candidates_cleared or an item stays open. EVERY candidate must appear in candidates_cleared " +
	"or candidates_refuted — a candidate you looked at and discarded goes in `candidates_refuted`; " +
	"leaving it out of both is not a clean sweep, it is an unfinished one. `families_not_run` " +
	"MUST list every family in unseeded_families. OPTIONAL attested fields enrich the record the " +
	"next sweep leans on and the report shows; a sweep that supplies none still verifies: `tier` " +
	"(the deepest tier you worked), `near_misses`:[...] (candidates you refuted before filing and " +
	"what refuted each — the report surfaces these, they are invisible everywhere else), " +
	"`checked_clean`:[{class, method}] (a family recorded clean WITH the method that checked it; " +
	"the method is not optional decoration — without it the next sweep cannot trust the claim), " +
	"`findings_verified` and `findings_suspected` (counts), and `report_path` (where you wrote " +
	"the HTML report). Before you start, scan `production_excluded`: source files the denylist " +
	"dropped from the denominator (build output, tests, vendored code). If one is hand-written " +
	"source in an unconventional path — JS under `dist/`, say — the fraction is under-counting and " +
	"you must read and count it by hand."

// BuildPlan is `ferret plan`.
func BuildPlan(mapdir, pinnedSHA, repo, since string) (*Plan, error) {
	docs, unseeded, err := loadMap(mapdir, pinnedSHA)
	if err != nil {
		return nil, err
	}
	dead := docs["_dead.json"]
	fidelity := dead.Fidelity
	if fidelity == "" {
		fidelity = "unknown"
	}
	// An UNRECOGNISED fidelity must announce itself. It once silently fell through to the
	// heuristic bar, so every candidate from a real RTA call graph carried "weak lead" —
	// measured: 628 of 628 candidates mislabelled. It errs safe, which is why nobody noticed.
	fbar, known := fidelityBar[fidelity]
	if !known {
		fbar = fmt.Sprintf(" NOTE: UNRECOGNISED fidelity %q — this gate has no bar for it, so it "+
			"is being treated as the weakest. Add it to fidelityBar rather than reading this "+
			"note as a statement about the evidence.", fidelity)
	}

	// magma declares, machine-readably, the ways its own analysis can be wrong. Those belong ON the
	// bar the sweeper has to clear, not in a field nobody reads: a `dead-on-arrival` candidate from
	// a backend that cannot see closure edges needs "check closures" in its refuter list.
	limBar := ""
	if len(dead.Limitations) > 0 {
		var parts []string
		for _, l := range dead.Limitations {
			parts = append(parts, fmt.Sprintf("%s (%s): %s", l.ID, l.Effect, l.Description))
		}
		limBar = " DECLARED MAP LIMITATIONS — the producer says these can make this row wrong: " +
			strings.Join(parts, " · ")
	}

	cands := []Candidate{}
	if dead.ReachabilityComputable {
		for _, r := range dead.Rows {
			cands = append(cands, Candidate{Family: "A", Class: "dead-on-arrival",
				Bar: bars["dead-on-arrival"] + fbar + limBar, Symbol: r.Symbol, File: r.File, Line: r.Line})
		}
		for _, r := range docs["_test-only.json"].Rows {
			cands = append(cands, Candidate{Family: "A", Class: "test-only",
				Bar: bars["test-only"] + fbar + limBar, Symbol: r.Symbol, File: r.File, Line: r.Line})
		}
	}
	if d := docs["_duplicates.json"]; d != nil {
		for _, cl := range d.Clusters {
			if len(cl.Members) == 0 {
				continue
			}
			syms := make([]string, 0, len(cl.Members))
			for _, m := range cl.Members {
				syms = append(syms, m.Symbol)
			}
			cands = append(cands, Candidate{Family: "D", Class: "duplicated-impl",
				Bar: bars["duplicated-impl"], Symbol: strings.Join(syms, " ≡ "),
				File: cl.Members[0].File, Line: cl.Members[0].Line})
		}
	}

	unseededFamilies := []string{}
	unseededDetail := map[string]string{}
	for f, fam := range unseeded {
		unseededFamilies = append(unseededFamilies, fam)
		unseededDetail[f] = fmt.Sprintf("family %s has no map seed (magma does not emit %s)", fam, f)
	}

	// A REFUSED MAP IS NOT AN EMPTY ONE. magma distinguishes rows:null (the analysis could not run)
	// from rows:[] (it ran and found nothing), and its contract calls that distinction
	// load-bearing: "a refusal must never be mistaken for 'found nothing'".
	//
	// This gate used to discard it. Measured 2026-08-02 against a real refused map — magma 0.1.0
	// has no Rust parser — the plan came back with 0 candidates, no reason, and family A absent
	// from unseeded_families. A sweep could then report family A checked-clean over an analysis
	// that never ran, which is the single thing `unseeded_families` exists to prevent.
	//
	// So a refusal marks family A unseeded on exactly the same footing as a missing file, and the
	// reason travels with it: dropping the reason loses the WHY, and a reader cannot tell "no Rust
	// parser" from "the map is broken".
	if !dead.ReachabilityComputable {
		reason := dead.NotComputableReason
		if reason == "" {
			reason = "the map reports the analysis was not computable and gave no reason"
		}
		unseededFamilies = append(unseededFamilies, "A")
		unseededDetail["_dead.json"] = "family A was NOT COMPUTED, not found-empty: " + reason
	}
	sort.Strings(unseededFamilies)

	signals, fromLexicon, err := loadSignals(repo)
	if err != nil {
		return nil, err
	}
	production, unclassified, excluded, err := ProductionFiles(repo)
	if err != nil {
		return nil, err
	}
	work := enumerateWorklist(production, signals)
	required, deferred := splitWorklist(work)

	// THE COMPLEMENT. Every production file no signal reached, not merely the changed ones.
	// Measured on ghola @4f33b3c: internal/bridge/bridge.go matched no signal AND had not changed
	// since the baseline, so it appeared in neither h_worklist nor h_unmatched_changes, and the
	// gate returned COMPLETE without it. A high-consequence file that is both unenumerated and
	// unchanged was invisible to every output this gate had.
	matched := map[string]bool{}
	for _, w := range work {
		matched[w.Path] = true
	}
	unmatchedAll := []string{}
	for _, p := range production {
		if !matched[p] {
			unmatchedAll = append(unmatchedAll, p)
		}
	}

	changes := []WorkItem{}
	if since != "" {
		changes, err = unmatchedChanges(repo, since, signals)
		if err != nil {
			return nil, err
		}
	}

	instr := instructions
	if len(unseededFamilies) > 0 {
		instr += " NOT SEEDED BY THE MAP: families " + strings.Join(unseededFamilies, ", ") +
			" — run them by hand or record them as NOT RUN with that reason; they may not be " +
			"reported as checked-clean."
	}

	return &Plan{
		Contract: planContract, SHA: pinnedSHA, Fidelity: fidelity,
		ReachabilityComputable: dead.ReachabilityComputable,
		MapProvenance: map[string]string{"generator": dead.Generator,
			"contract_version": dead.ContractVersion},
		VocabProvenance: map[string]string{
			"lexicon":              lexiconOrNone(LexiconPath()),
			"lexicon_version":      lexiconVersion(LexiconPath()),
			"signals_total":        strconv.Itoa(len(signals)),
			"signals_from_lexicon": strconv.Itoa(fromLexicon),
			"signals_from_repo":    strconv.Itoa(len(signals) - fromLexicon),
		},
		NotComputableReason: dead.NotComputableReason,
		MapLimitations:      limNames(dead.Limitations),
		UnseededFamilies:    unseededFamilies, UnseededDetail: unseededDetail,
		Candidates: cands, ProductionTotal: len(production), ProductionFiles: production,
		ProductionUnclassified: nonNil(unclassified), ProductionExcluded: nonNil(excluded),
		HWorklist: nonNilW(work),
		HRequired: nonNilW(required), HDeferred: nonNilW(deferred), HUnmatched: unmatchedAll,
		HUnmatchedChanges: changes, ChangeBaseline: since, Instructions: instr,
	}, nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func nonNilW(s []WorkItem) []WorkItem {
	if s == nil {
		return []WorkItem{}
	}
	return s
}

// UnseededDetailValues is the detail strings alone, for callers that want to scan the reasons
// without caring which file each came from.
func (p *Plan) UnseededDetailValues() []string {
	out := make([]string, 0, len(p.UnseededDetail))
	for _, v := range p.UnseededDetail {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// limNames flattens the producer's declared limitations for the plan header, so they are visible
// even on a plan that raised no candidates.
func limNames(ls []struct {
	ID          string `json:"id"`
	Effect      string `json:"effect"`
	Description string `json:"description"`
}) []string {
	out := make([]string, 0, len(ls))
	for _, l := range ls {
		out = append(out, l.ID+" ("+l.Effect+")")
	}
	return out
}

// parseLexiconSignals extracts the ```h-signals fenced block from the lexicon. Reading it out of the
// markdown rather than from a sidecar keeps ONE artifact with ONE version: a reader editing a class
// definition sees the paths that class lives on, in the same file.
func parseLexiconSignals(path string) [][2]string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var fenced []string
	in := false
	for _, line := range strings.Split(string(b), "\n") {
		t := strings.TrimSpace(line)
		if !in {
			if t == "```h-signals" {
				in = true
			}
			continue
		}
		if strings.HasPrefix(t, "```") {
			break
		}
		fenced = append(fenced, line)
	}
	return parseSignalLines(strings.Join(fenced, "\n"))
}

// parseSignalFile reads a bare `reason: regex` file — the per-repo `.slop-h-signals` extension.
// A missing file is not an error: the vocabulary is optional by construction, and an empty worklist
// is already a hard stop in `enumerate`, which says what to do about it far better than a parse
// error here would.
// Matching is O(files x signals), so the signal count is a COST the target repository controls.
// Measured on 2,000 production paths: 200 signals 4.1s, 500 15.1s, 1,000 20.2s, 2,000 59.9s — so a
// committed 100k-line file is hours. `.slop-h-signals` comes from the repo under audit, and this
// tool exists to be pointed at repositories you have reason to distrust; unbounded input from that
// source is a denial of service on the operator, not on anyone else.
//
// The caps are generous against real use (the shipped lexicon carries 9) and refuse loudly rather
// than truncating: silently reading the first N would produce a sweep whose worklist depended on
// line order, which is worse than not running.
const (
	maxSignalFileBytes = 256 << 10
	maxSignalLines     = 500
)

func parseSignalFile(path string) ([][2]string, error) {
	fi, err := os.Lstat(path)
	if err != nil || !fi.Mode().IsRegular() {
		return nil, nil
	}
	if fi.Size() > maxSignalFileBytes {
		return nil, die(ExitRefused, "%s is %d bytes, over the %d-byte cap. Signal matching is "+
			"O(files x signals) and this file comes from the repository being audited, so an "+
			"oversized one stalls the sweep rather than shortening it",
			path, fi.Size(), maxSignalFileBytes)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}
	out := parseSignalLines(string(b))
	if len(out) > maxSignalLines {
		return nil, die(ExitRefused, "%s defines %d signals, over the cap of %d. Measured cost: "+
			"2,000 signals over 2,000 paths takes ~60s, and it scales with both. Narrow the file "+
			"rather than raising this, or the worklist it produces is not one anybody will wait for",
			path, len(out), maxSignalLines)
	}
	return out, nil
}

func parseSignalLines(body string) [][2]string {
	var out [][2]string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		reason, rx := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if reason == "" || rx == "" {
			continue
		}
		out = append(out, [2]string{reason, rx})
	}
	return out
}

// lexiconVersion reads the deployed lexicon's `version:` line. It is recorded on the plan because
// the vocabulary now lives OUTSIDE the binary, on a cadence the binary does not control: without
// it, two sweeps that enumerated different worklists for the same tree leave no trace of why.
func lexiconVersion(path string) string {
	if path == "" {
		return "unknown"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "unreadable"
	}
	for _, line := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "version:"); ok {
			return strings.TrimSpace(v)
		}
	}
	return "unstated"
}

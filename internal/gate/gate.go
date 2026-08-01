// Package gate is the coded seam between the magma code map and a slop sweep.
//
// Ported from python/gate.py on 2026-08-01 and verified differentially against it: both
// implementations were run over a real repository and their plan output compared field by field
// before the Python was deleted. The measured constants below came from real repositories and are
// carried across unchanged — a port is the easiest place to quietly lose a measurement.
//
//	slop-ferret plan   <magma-map-dir> <pinned-sha> <repo> [--since <ref>]  > plan.json
//	slop-ferret verify <plan.json> <discharge.json>          ; 0 settled, 3 items open
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
//	coverage.repo  production source files read / total    <- "was the repo covered"
//	coverage.plan  items dispositioned / items raised      <- "was the plan worked through"
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
	"strings"
)

const (
	mapSubdir    = ".magma"
	planContract = "slop-gate/2"
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

// family-H is found by READING, not scanning, so the map cannot seed it. These path/name signals
// RANK the reading order; they no longer decide whether a file is looked at, because the
// complement is enumerated too.
//
// BEFORE YOU ADD A WORD HERE: these tables are COUPLED to the tier floor below. Adding a term to
// a tier-1 group can move a file into the required tier. Measured 2026-08-01: adding `ratelimit`
// to money/value matched ghola's internal/client/ratelimit.go, which defeated a zero-check floor
// and took ghola from 10 required to 1, deferring 9 files on a repo readable in one pass. EVERY
// TEST PASSED. Re-measure required/deferred on real repos after any edit here.
var hSignalSrc = [][2]string{
	// `financial|fund|spend|budget|quota|ratelimit` added 2026-08-01: roboticus's
	// engine_financial_config_rules.go — a FINANCIAL RULES ENGINE — matched nothing, because a
	// money vocabulary listing pay|ledger|billing|fee|gas|price does not contain "financial".
	{"money/value", `pay|payment|ledger|billing|invoice|wallet|treasury|balance|settle|revenue|` +
		`refund|x402|transfer|mint|burn|stake|slash|reward|supply|escrow|vault|` +
		`fee|gas|price|swap|exchange|liquidity|collateral|financial|fund|cost|` +
		`charge|spend|budget|quota|credit|debit|ratelimit|throttle`},
	{"consensus/ordering", `consensus|validator|block|blockchain|mempool|finality|fork|quorum|` +
		`bft|raft|paxos|leader|proposer|commit_reveal|ordering|nonce|frontrun|mev|reorg|slot|epoch`},
	// `policy|authority|...` added 2026-08-01: roboticus's internal/agent/policy/ is 15 production
	// files of authorization logic and matched NOTHING, because an auth vocabulary listing
	// auth|acl|rbac does not contain the word "policy".
	{"auth/session", `auth|session|login|token|oauth|jwt|permission|acl|rbac|capability|tenant|` +
		`policy|authority|approval|denial|consent|grant|privilege|role`},
	{"crypto/signing", `crypto|sign|signature|keypair|secp|ecdsa|ed25519|hmac|cipher|encrypt|` +
		`decrypt|seed|mnemonic|merkle|hash|zk|proof|commitment|nullifier`},
	{"arithmetic/overflow", `checked_arith|safe_math|overflow|saturating|decimal|precision|rounding`},
	{"migration", `migrat|schema_version|alembic|flyway|goose`},
	{"persistence/state", `repo|repository|store|dao|dal|persist|database|state|account|utxo|` +
		`trie|db|sql|journal|wal`},
	{"untrusted-parse", `parse|parser|deserial|unmarshal|decode|webhook|ingest|codec|rpc|api`},
	// Added 2026-08-01. ghola @4f33b3c — a pre-registered control repo — enumerated ZERO H-paths
	// and so could never reach a verdict, because an HTTP fetch client whose whole surface is
	// parsing untrusted remote responses matched none of the vocabulary above.
	{"network/untrusted-io", `client|http|fetch|request|response|header|cookie|redirect|tls|` +
		`ssl|cert|proxy|socket|dial|stream|download|upload|url|uri|host|dns|transport`},
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
var fidelityBar = map[string]string{
	"reachability": "",
	"rta":          "",
	"exports": " NOTE: fidelity=exports (unused-export graph, not a call graph) — also confirm no " +
		"dynamic/reflective use before trusting 'dead'",
	"heuristic": " NOTE: fidelity=heuristic (guess with confidence, no call graph) — treat as a " +
		"weak lead; read before filing",
	"rustc-dead_code": " NOTE: fidelity=rustc-dead_code (the compiler's never-used lint, real " +
		"signal but crate-local) — it cannot see `pub` API surface or cross-crate use, so also " +
		"confirm the item is not a published API, and not reached via cfg/macro/trait dispatch",
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
	UnseededFamilies       []string          `json:"unseeded_families"`
	UnseededDetail         map[string]string `json:"unseeded_detail"`
	Candidates             []Candidate       `json:"candidates"`
	ProductionTotal        int               `json:"production_total"`
	ProductionFiles        []string          `json:"production_files"`
	ProductionUnclassified []string          `json:"production_unclassified"`
	HWorklist              []WorkItem        `json:"h_worklist"`
	HRequired              []WorkItem        `json:"h_required"`
	HDeferred              []WorkItem        `json:"h_deferred"`
	HUnmatched             []string          `json:"h_unmatched"`
	HUnmatchedChanges      []WorkItem        `json:"h_unmatched_changes"`
	ChangeBaseline         string            `json:"change_baseline"`
	Instructions           string            `json:"instructions"`
}

type rowDoc struct {
	ContractVersion        string `json:"contract_version"`
	Generator              string `json:"generator"`
	SHA                    string `json:"sha"`
	Fidelity               string `json:"fidelity"`
	ReachabilityComputable bool   `json:"reachability_computable"`
	Rows                   []struct {
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

func loadSignals(repo string) []signal {
	src := append([][2]string{}, hSignalSrc...)
	// Path-based H enumeration is vocabulary-bound; a project whose domain terms are missing must
	// be able to add them rather than silently get a short worklist.
	extra := filepath.Join(repo, ".slop-h-signals")
	if fi, err := os.Lstat(extra); err == nil && fi.Mode().IsRegular() {
		if b, err := os.ReadFile(extra); err == nil {
			for _, line := range strings.Split(string(b), "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, ":") {
					continue
				}
				parts := strings.SplitN(line, ":", 2)
				src = append(src, [2]string{strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])})
			}
		}
	}
	out := make([]signal, 0, len(src))
	for _, p := range src {
		rx, err := regexp.Compile(`(?i)` + anchor + `(` + p[1] + `)`)
		if err != nil {
			continue
		}
		out = append(out, signal{reason: p[0], rx: rx})
	}
	return out
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
// denominator without saying so.
func ProductionFiles(repo string) (production, unclassified []string, err error) {
	files, err := gitLines(repo, "ls-files", "-z")
	if err != nil {
		return nil, nil, err
	}
	for _, f := range files {
		if notH.MatchString(f) {
			continue
		}
		if sourceExt.MatchString(f) {
			production = append(production, f)
		} else {
			unclassified = append(unclassified, f)
		}
	}
	return production, unclassified, nil
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
		// A dirty-tree map reports a composite `<sha>+<diffhash>` rather than a bare sha, so it can
		// never equal a pinned commit and this refuses by construction. That is intended: a dirty
		// map can report in-flight, not-yet-wired code as dead, and its sha is disproportionately
		// likely to evaporate because in-flight commits get amended or rebased away.
		if doc.SHA != pinnedSHA {
			extra := ""
			if strings.Contains(doc.SHA, "+") {
				extra = " That is a DIRTY-tree map (`<sha>+<diffhash>`); commit or stash first, " +
					"then regenerate. Never gate on a dirty map."
			}
			return nil, nil, die(ExitRefused, "%s sha %q != pinned %q — the map describes a different tree "+
				"than the sweep; regenerate the map at %s.%s", name, doc.SHA, pinnedSHA, pinnedSHA, extra)
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
	"candidates_refuted:[{file,symbol}]} and run `slop-ferret verify`. `coverage_waived` entries may be " +
	"a bare path or {path, reason} — a reason is OPTIONAL. Waiving is cheap on purpose: deciding " +
	"not to read a file is a normal, correct move and should cost nothing. It settles the " +
	"ACCOUNTING and leaves `coverage.repo` alone, because a waived file genuinely was not read " +
	"and the fraction is there to tell YOU what you actually looked at. No coverage floor is " +
	"enforced: there is no defensible number, and a red build for reading 67% instead of 90% " +
	"would only teach you to waive to clear it. `sha` must equal this plan's sha. " +
	"`candidates_filed` is what you actually ACCUSED; every entry must also appear in " +
	"candidates_cleared or an item stays open. EVERY candidate must appear in candidates_cleared " +
	"or candidates_refuted — a candidate you looked at and discarded goes in `candidates_refuted`; " +
	"leaving it out of both is not a clean sweep, it is an unfinished one. `families_not_run` " +
	"MUST list every family in unseeded_families."

// BuildPlan is `slop-ferret plan`.
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

	cands := []Candidate{}
	if dead.ReachabilityComputable {
		for _, r := range dead.Rows {
			cands = append(cands, Candidate{Family: "A", Class: "dead-on-arrival",
				Bar: bars["dead-on-arrival"] + fbar, Symbol: r.Symbol, File: r.File, Line: r.Line})
		}
		for _, r := range docs["_test-only.json"].Rows {
			cands = append(cands, Candidate{Family: "A", Class: "test-only",
				Bar: bars["test-only"] + fbar, Symbol: r.Symbol, File: r.File, Line: r.Line})
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
	sort.Strings(unseededFamilies)

	signals := loadSignals(repo)
	production, unclassified, err := ProductionFiles(repo)
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
		UnseededFamilies: unseededFamilies, UnseededDetail: unseededDetail,
		Candidates: cands, ProductionTotal: len(production), ProductionFiles: production,
		ProductionUnclassified: nonNil(unclassified), HWorklist: nonNilW(work),
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

#!/usr/bin/env python3
"""gate — the coded seam between the magma code map and a slop sweep.

Skills cannot call skills; only a script emitting a file and another script consuming it is
a real, fail-closed contract. This is that consumer. It does two jobs:

    gate.py plan   <magma-map-dir> <pinned-sha> <repo>     > plan.json
    gate.py verify <plan.json> <discharge.json>            ; exit 0 = settled, 3 = items open

THIS IS A TOOL FOR THE PERSON RUNNING THE SWEEP. It is not an evaluation of them and not a
compliance mechanism: nobody is graded by its output, there is no adversary to design
against, and its job is to hand back a work queue and an honest instrument reading. Read the
register of anything below that sounds like enforcement as a historical artifact of how this
file used to be written, not as intent.

`plan` reads magma's per-row JSON, refuses to proceed unless it is the right tree (sha) and a
shape it can parse (contract_version), turns map rows into per-family candidates carrying the
class's pre-filing bar and the backend's fidelity, and enumerates BOTH the signal-matched
family-H worklist AND its complement — every other production source file (`h_unmatched`).

`verify` reports TWO FRACTIONS and no verdict word:
    coverage.repo  production source files read / total     <- "was the repo covered"
    coverage.plan  items dispositioned / items raised       <- "was the plan worked through"
They are different numbers and the gap between them is the interesting part. The verdict
triple COMPLETE/PARTIAL/INCOMPLETE was removed 2026-08-01 because it compressed both into one
token: ghola @4f33b3c scored 10/10 on the plan and ~16/24 on the repo and reported COMPLETE,
having never enumerated `internal/bridge/bridge.go` — an unauthenticated localhost HTTP server
performing arbitrary outbound fetches, which a hand read then showed to be the worst file in
the repo. Coverage of the enumeration was being reported as coverage of the repository.

`status` ("open"/"settled") and the exit code carry bookkeeping only: whether anything raised
is still undispositioned, the way a test runner reports outstanding failures. Neither says the
repo is covered; that is a fraction, and a fraction does not fit in an exit code.

WHAT THIS GATE DOES AND DOES NOT ESTABLISH. It checks that the sweep ACCOUNTED for every
item the plan raised. It does not, and cannot, establish that a file was read: `read_paths`
is self-reported by the sweeping agent and nothing here corroborates it. Earlier text in this
docstring claimed the gate "inverts the old gate ... to 'prove you looked'" — by this skill's
own lexicon that is a `Fabricated claim` (B), shipped inside the skill's own coverage
guarantee, and the output key was renamed `h_paths_attested` to stop the report inheriting it.
Attestation is still worth requiring: it makes an omission a statement someone made rather
than a gap nobody has to own.

Only the unread-path clause was ever implemented; the sha, empty-worklist and unseeded-family
clauses were added 2026-08-01 after sweeping this skill with its own method, each reproduced
first. The filed clause had been promised here since the first version and was UNIMPLEMENTABLE
as written, because the discharge had no notion of "filed" — so a sweep could clear nothing and
still be certified COMPLETE with exit 0 (measured: 628 candidates, 0 cleared, COMPLETE).

That repair was itself incomplete, and the 2026-08-01 audit reproduced the remainder: the
filed clause only fires when the sweep FILES something, so a sweep that filed nothing was
still certified COMPLETE with every candidate unexamined (measured against a real counterspy
@531cc42 plan: 12 candidates, 0 cleared, COMPLETE, exit 0). "COMPLETE, no findings" is the
most consequential thing this skill emits, because it is the one a reader takes as "this repo
is clean" — so it is the last path that should have been unguarded. The accounting clause
below closes it.
"""
import json, sys, subprocess, pathlib, re

SUPPORTED_CONTRACTS = {"codemap-rows/1"}
# Row files live under `<output-root>/<name>/.magma/`, NOT at the map root. magma writes them
# there deliberately, to keep machine files out of the Obsidian vault view, and its own
# main.go documents ".magma/ — the audit gate reads those directly". This gate read the root
# for its whole life and exited 3 on every real map; confirmed with the magma maintainer
# 2026-08-01 that the gate is the side that moves.
MAP_SUBDIR = ".magma"

# REQUIRED: absence means the map cannot seed anything and the gate must refuse.
MAP_FILES_REQUIRED = ["_dead.json", "_test-only.json"]

# OPTIONAL: absence must DEGRADE COVERAGE HONESTLY, never fail closed and never pass silently.
# magma does not emit these yet.
#   _interfaces.json  approved and tractable (types.Implements over the module's own named
#                     types); seeds family E when it lands.
#   _duplicates.json  deliberately NOT built. magma has no notion of similarity, and a
#                     duplicate row that is not a duplicate is a refactor order for code that
#                     should be left alone -- the same class of harm as a false dead-code row
#                     being a deletion order. Emitting nothing is the correct behaviour until
#                     there is a defensible metric, so do not plan family D around it.
# The families they would seed are reported as NOT SEEDED in the plan, which is what stops a
# missing file reading as a clean family.
MAP_FILES_OPTIONAL = {"_interfaces.json": "E", "_duplicates.json": "D"}

MAP_FILES = MAP_FILES_REQUIRED + list(MAP_FILES_OPTIONAL)

# family-H is found by READING, not scanning — so the map cannot seed it. The gate instead
# enumerates the paths where H lives and REQUIRES a read of each. Path/name signals only
# (content signals would need a read, which is the point). Extend per project.
# The defaults below are DOMAIN-BOUND and that is the known limit of path-based enumeration:
# a codebase whose high-consequence vocabulary is not listed under-enumerates SILENTLY, and a
# short worklist then reads as a clean repo. Measured: the fintech/SaaS-shaped first draft
# found 11 H-paths in a 518-file blockchain and missed 88 (consensus, mempool, mev, gas,
# bridge, checked_arithmetic). Extend per repo via `.slop-h-signals` (see load_h_signals).
#
# BEFORE YOU ADD A WORD HERE, READ THIS: these tables are COUPLED TO THE TIER FLOOR below
# (H_TIER_1 / split_worklist). Adding a term to a tier-1 group can move a file into the
# required tier and thereby change what `verify` demands — and adding one to a SMALL repo's
# worklist can flip it out of the all-required floor entirely. Measured 2026-08-01: adding
# `ratelimit` to money/value matched ghola's `internal/client/ratelimit.go`, which defeated a
# zero-check floor and took ghola from 10 required to 1, deferring 9 files on a repo readable
# in one pass. EVERY TEST PASSED. Re-measure required/deferred on real repos after any edit
# here; the unit tests cannot see it.
_H = [
    # `financial|fund|spend|budget|quota|ratelimit` added 2026-08-01: roboticus's
    # `engine_financial_config_rules.go` — a FINANCIAL RULES ENGINE — matched nothing, because
    # a money vocabulary listing pay|ledger|billing|fee|gas|price does not contain the word
    # "financial". Neither did `provider_ratelimit.go`, which carries funding attribution.
    ("money/value", r"pay|payment|ledger|billing|invoice|wallet|treasury|balance|settle|revenue|"
                    r"refund|x402|transfer|mint|burn|stake|slash|reward|supply|escrow|vault|"
                    r"fee|gas|price|swap|exchange|liquidity|collateral|financial|fund|cost|"
                    r"charge|spend|budget|quota|credit|debit|ratelimit|throttle"),
    ("consensus/ordering", r"consensus|validator|block|blockchain|mempool|finality|fork|quorum|"
                           r"bft|raft|paxos|leader|proposer|commit_reveal|ordering|nonce|"
                           r"frontrun|mev|reorg|slot|epoch"),
    # `policy|authority|approval|denial|...` added 2026-08-01: roboticus's
    # `internal/agent/policy/` is 15 production files of authorization logic and matched
    # NOTHING at any anchor, because an auth vocabulary listing auth|acl|rbac does not contain
    # the word "policy".
    ("auth/session", r"auth|session|login|token|oauth|jwt|permission|acl|rbac|capability|tenant|"
                     r"policy|authority|approval|denial|consent|grant|privilege|role"),
    ("crypto/signing", r"crypto|sign|signature|keypair|secp|ecdsa|ed25519|hmac|cipher|encrypt|"
                       r"decrypt|seed|mnemonic|merkle|hash|zk|proof|commitment|nullifier"),
    ("arithmetic/overflow", r"checked_arith|safe_math|overflow|saturating|decimal|precision|rounding"),
    ("migration", r"migrat|schema_version|alembic|flyway|goose"),
    ("persistence/state", r"repo|repository|store|dao|dal|persist|database|state|account|utxo|"
                          r"trie|db|sql|journal|wal"),
    ("untrusted-parse", r"parse|parser|deserial|unmarshal|decode|webhook|ingest|codec|rpc|api"),
    # Added 2026-08-01. `ghola` @4f33b3c — one of the two PRE-REGISTERED CONTROL REPOS —
    # enumerated ZERO H-paths and so could never reach a verdict, because an HTTP fetch client
    # whose whole surface is parsing untrusted remote responses matched none of the vocabulary
    # above. The domain-bound limit the block comment warns about, firing on the project's own
    # control. Everything here is a trust boundary: bytes arriving from somewhere else.
    ("network/untrusted-io", r"client|http|fetch|request|response|header|cookie|redirect|tls|"
                             r"ssl|cert|proxy|socket|dial|stream|download|upload|url|uri|host|"
                             r"dns|transport"),
]


# A signal must match at a path start, a SEGMENT start, or after a word separator inside a
# filename. The original anchor was `(^|/)` alone, which required the keyword to begin a
# segment -- so `internal/db/user_store.go` was MISSED even though `store` is in the
# persistence vocabulary, and `internal/agent/control_token_strip.go` was missed even though
# `token` is in the auth vocabulary. Measured on roboticus: relaxing it adds 81 files to the
# 285-path worklist, including the injection control-marker strip and the actuation ledger.
# The comment below documents the VOCABULARY limit; this positional one was undocumented and
# is the sharper of the two, because a listed word reads as covered.
ANCHOR = r"(^|/|[_.\-])"

# CONSEQUENCE TIERS. Tier 1 is the blast-radius set: being wrong there costs the most, which
# is family H's own selection rule ("if this were wrong, what would it cost?", not "does this
# look odd?"). Tier 2 is the volume set — real H surface, but the paths that make a worklist
# large rather than sharp.
#
# This exists because tier 1 was UNBOUNDED and the cost was measured, not argued: roboticus
# @443681b9 enumerates 387 H-paths and `verify` required every one before returning anything
# but INCOMPLETE. On the repo the method was validated against, no honest sweep could reach a
# verdict — so the verdict carried no information, and the only route to a COMPLETE was the
# attestation hole. Deliberately a SEMANTIC split and not a top-N cap: a numeric cap is a
# magic number nobody can defend, and it would silently drop whichever paths sorted last.
H_TIER_1 = {"money/value", "crypto/signing", "auth/session", "consensus/ordering",
            "arithmetic/overflow"}
H_TIER_2 = {"migration", "persistence/state", "untrusted-parse", "network/untrusted-io"}
# A signal the operator added via `.slop-h-signals` is required, not deferred: they added it
# because the built-in vocabulary missed their domain, which is the strongest available
# evidence that those paths matter here.
H_TIER_DEFAULT_REQUIRED = True


# Worklists at or below this are required in full: deferral exists to make a LARGE worklist
# tractable, and below the size where a full H read is feasible in one sweep it buys nothing
# and costs coverage. A judgement, stated as one — no sweep has ever been completed, so there
# is no measured feasibility ceiling to derive it from. Its effect on the three repos to hand:
# ghola 10 and counterspy 35 are required in full; roboticus 475 splits 185/290.
H_DEFER_FLOOR = 60


def split_worklist(work):
    """Partition the worklist into (required, deferred) by consequence tier.

    Two floors, and the second exists because the first was too narrow. If NO tier-1 path
    exists the whole worklist is required — `ghola`'s shape, every H-path network/parse, which
    a tier-1-only rule would hand an EMPTY required set, i.e. a repo certifiable without
    reading anything. But that is a ZERO-check, and one incidental tier-1 match defeats it:
    adding `ratelimit` to the money vocabulary matched `internal/client/ratelimit.go` and took
    ghola from 10 required to 1, deferring 9 files on a repo readable in a single pass. So the
    size floor below is the real rule and the zero-check is its degenerate case.
    """
    if len(work) <= H_DEFER_FLOOR:
        return list(work), []
    required = [w for w in work
                if w["reason"] in H_TIER_1
                or (w["reason"] not in H_TIER_2 and H_TIER_DEFAULT_REQUIRED)]
    if not required:
        return list(work), []
    req_paths = {w["path"] for w in required}
    return required, [w for w in work if w["path"] not in req_paths]


def load_h_signals(repo):
    """Built-in defaults plus an optional per-repo `.slop-h-signals` (lines: `reason: regex`).
    Path-based H enumeration is vocabulary-bound; a project whose domain terms are missing
    must be able to add them rather than silently get a short worklist."""
    pairs = list(_H)
    extra = pathlib.Path(repo) / ".slop-h-signals"
    if extra.is_file() and not extra.is_symlink():
        for line in extra.read_text(errors="ignore").splitlines():
            line = line.strip()
            if not line or line.startswith("#") or ":" not in line:
                continue
            reason, alt = line.split(":", 1)
            pairs.append((reason.strip(), alt.strip()))
    return [(r, re.compile(ANCHOR + "(" + a + r")", re.I)) for r, a in pairs]


# what each map-seeded class must clear before a candidate becomes a filed finding.
BARS = {
    "dead-on-arrival": "universal-negative refuter: prove nothing reaches it (reflection/init/"
                       "codegen/FFI checked), not just that the count is 0",
    "test-only": "confirm no production caller AND that the symbol is not an entry point invoked "
                 "from outside the repo",
    "duplicated-impl": "confirm the copies are semantically identical AND that no test pins them "
                       "to each other",
    "single-impl-interface": "apply the discriminator: a consumer-declared narrow port is NOT "
                             "over-abstraction; only a producer-side single-impl interface is",
}
# heavier bar when the map's fidelity is weaker than a real call graph.
FIDELITY_BAR = {
    "reachability": "",  # deadcode RTA — the row is a strong candidate as-is
    "rta": "",           # magma's name for the same thing (Rapid Type Analysis)
    "exports": " NOTE: fidelity=exports (unused-export graph, not a call graph) — also confirm no "
               "dynamic/reflective use before trusting 'dead'",
    "heuristic": " NOTE: fidelity=heuristic (guess with confidence, no call graph) — treat as a "
                 "weak lead; read before filing",
    "rustc-dead_code": " NOTE: fidelity=rustc-dead_code (the compiler's never-used lint, real "
                       "signal but crate-local) — it cannot see `pub` API surface or cross-crate "
                       "use, so also confirm the item is not a published API, and not reached via "
                       "cfg/macro/trait dispatch",
}


def die(msg, code=2):
    print(f"gate: {msg}", file=sys.stderr)
    sys.exit(code)


def load_map(mapdir, pinned_sha):
    """Read the map files; refuse loud unless it is the right tree and a shape we parse.

    Returns (docs, unseeded) where `unseeded` maps a missing optional file to the family it
    would have seeded. Missing optional files are NOT an error, but they must reach the plan,
    because a family with no seed is a family that did not run, and a sweep that reports it
    as clean is the failure this whole gate exists to prevent.
    """
    d = pathlib.Path(mapdir)
    if not d.is_dir():
        die(f"map dir {mapdir} does not exist — run magma first", 3)
    # Tolerate being handed either the map root or the .magma subdir itself.
    if d.name != MAP_SUBDIR and (d / MAP_SUBDIR).is_dir():
        d = d / MAP_SUBDIR
    docs, unseeded = {}, {}
    for name in MAP_FILES:
        p = d / name
        if not p.is_file():
            if name in MAP_FILES_OPTIONAL:
                unseeded[name] = MAP_FILES_OPTIONAL[name]
                continue
            die(f"{name} missing from {d} — regenerate the map with `magma <repo> <name> <vault>`. "
                f"If magma was itself updated, pass --force: freshness is keyed on the ANALYSED "
                f"repo's sha, not on magma's version, so an unchanged repo silently reports "
                f"'already fresh' and writes nothing.", 3)
        doc = json.loads(p.read_text())
        cv = doc.get("contract_version")
        if cv not in SUPPORTED_CONTRACTS:
            die(f"{name} contract_version {cv!r} not in {sorted(SUPPORTED_CONTRACTS)} — "
                f"magma is newer/older than this gate; update the gate or pin magma. NOTE there "
                f"are three magma contracts and they are NOT interchangeable: codemap-rows/1 "
                f"(row files, the only one this gate may accept), codemap-graph/1 (graph.json), "
                f"magma-code-graph/1 (the architext emit).", 3)
        sha = doc.get("sha")
        # A dirty-tree map reports a composite `<sha>+<diffhash>` rather than a bare sha, so it
        # can never equal a pinned commit and this refuses by construction. That is intended:
        # a dirty map can report in-flight, not-yet-wired code as dead, and its sha is
        # DISPROPORTIONATELY likely to evaporate, because in-flight commits get amended or
        # rebased away. Two prior roboticus sweeps pinned dirty-map shas and neither resolves
        # today, which is how their denominators became unreproducible.
        if sha != pinned_sha:
            extra = ""
            if isinstance(sha, str) and "+" in sha:
                extra = (" That is a DIRTY-tree map (`<sha>+<diffhash>`); commit or stash first, "
                         "then regenerate. Never gate on a dirty map.")
            die(f"{name} sha {sha!r} != pinned {pinned_sha!r} — the map describes a "
                f"different tree than the sweep; regenerate the map at {pinned_sha}.{extra}", 3)
        docs[name] = doc
    return docs, unseeded


# H is about production logic. Test files and prose docs are not H-read targets — including
# them inflates the worklist (dogfooding roboticus: 266 → the real production surface once
# tests/docs are dropped) and would force reading a test to "discharge" a coverage item.
#
# VENDORED/GENERATED TREES ADDED 2026-08-01. SKILL.md Step 1 has always mandated excluding
# them from the denominator and this pattern never did, so on a vendored Go repo third-party
# files flooded the required tier and COMPLETE was unreachable. That was a nuisance while this
# set only filtered the WORKLIST; it became load-bearing when the same set started defining the
# coverage DENOMINATOR below, where vendored files would silently sink the fraction instead.
_NOT_H = re.compile(r'(_test\.|\.test\.|(^|/)tests?[_.]|(^|/)test_|(^|/)tests?/|'
                    r'(^|/)testdata/|(^|/)benches?/|(^|/)examples?/|(^|/)fuzz/|_spec\.|\.spec\.'
                    r'|(^|/)vendor/|(^|/)vendored/|(^|/)third_party/|(^|/)node_modules/|'
                    r'(^|/)\.venv/|(^|/)dist/|(^|/)generated/|\.pb\.go$|_pb2\.py$|'
                    r'\.generated\.|\.min\.js$'
                    r'|\.(md|markdown|rst|txt|json|ya?ml|toml|lock|sum|mod)$)', re.I)

# The coverage denominator is an ALLOWLIST, deliberately, and the choice is the opposite of the
# one made for H signals. An H signal guesses semantics from a name the target's authors chose,
# which is why it under-enumerates silently. A file extension is a language-level fact, and a
# language this list omits does not shrink the denominator quietly — it lands in
# `production_unclassified`, which the plan prints. Absence announces itself; that is the whole
# difference between this list and the vocabulary above.
_SOURCE_EXT = re.compile(r'\.(go|rs|ts|tsx|js|jsx|mjs|py|rb|java|kt|kts|cs|swift|m|mm|'
                         r'c|cc|cpp|cxx|h|hpp|sh|bash|zsh|sol|tf|php|scala|clj|ex|exs|'
                         r'erl|hs|ml|lua|pl|r|dart|vue|svelte)$', re.I)


def production_files(repo):
    """The coverage universe: every tracked file that is production source.

    Returns (production, unclassified). `unclassified` is everything that survived the
    exclusion filter but carries no recognised source extension — reported rather than
    dropped, so an unsupported language cannot shrink the denominator without saying so.
    """
    out = subprocess.run(["git", "-C", repo, "ls-files", "-z"], capture_output=True, text=True)
    if out.returncode != 0:
        die(f"git ls-files failed in {repo}: {out.stderr.strip()[:200]}", 2)
    production, unclassified = [], []
    for f in (x for x in out.stdout.split("\0") if x):
        if _NOT_H.search(f):
            continue
        (production if _SOURCE_EXT.search(f) else unclassified).append(f)
    return production, unclassified


def enumerate_h_worklist(repo, production=None):
    """Signal-matched production paths, in worklist form.

    NOTE WHAT THIS IS NOW. Before 2026-08-01 this WAS the sweep's universe: matching a signal
    was how a file got looked at, and a file no signal reached was simply absent — invisible,
    uncounted, and indistinguishable from a file that had been cleared. The worklist is now a
    RANKING over `production_files`, not an admission gate: the complement is enumerated too
    (see `plan`) and must be dispositioned. A signal moves a file up the reading order; it no
    longer decides whether the file exists.
    """
    if production is None:
        production, _ = production_files(repo)
    signals = load_h_signals(repo)
    work = []
    for f in production:
        for reason, rx in signals:
            if rx.search(f):
                work.append({"path": f, "reason": reason})
                break
    return work


def unmatched_changes(repo, since):
    """Production files changed since `since` that NO H signal reached.

    THE COVERAGE HOLE THE GATE COULD NOT SEE. Path-based H enumeration is vocabulary-bound,
    and the vocabulary's own gaps are invisible from inside it: a subsystem nobody named is
    simply absent from the worklist, and a short worklist reads as a clean repo. Extending the
    vocabulary (or `.slop-h-signals`) fixes an instance and never the class, because nothing
    checks whether the extension was sufficient.

    This inverts the question. Instead of trying to enumerate every high-consequence path by
    name — unbounded, and unknowable from here — compare the enumeration against a set already
    known to matter: what actually changed. Every changed production file the signals failed to
    reach is a measured hole, printed on the face of the plan.

    Measured on roboticus @443681b9: 6 of 6 production .go files changed in the last 12 release
    commits were on neither the 387-path worklist nor the 129-path required tier.
    """
    out = subprocess.run(["git", "-C", repo, "diff", "--name-only", f"{since}..HEAD"],
                         capture_output=True, text=True)
    if out.returncode != 0:
        die(f"git diff {since}..HEAD failed in {repo}: {out.stderr.strip()[:200]}", 2)
    signals = load_h_signals(repo)
    holes = []
    for f in (x.strip() for x in out.stdout.splitlines() if x.strip()):
        if _NOT_H.search(f):
            continue
        if not any(rx.search(f) for _, rx in signals):
            holes.append({"path": f,
                          "reason": f"changed since {since} and no H signal reached it — the "
                                    "enumeration did not see this file, so its absence from "
                                    "the worklist is not evidence that it is benign"})
    return holes


def plan(mapdir, pinned_sha, repo, since=None):
    docs, unseeded = load_map(mapdir, pinned_sha)
    fidelity = docs["_dead.json"].get("fidelity", "unknown")
    # An UNRECOGNISED fidelity must announce itself. This silently fell through to the
    # heuristic bar, so every candidate from a real magma RTA call graph carried
    # "fidelity=heuristic (guess with confidence, no call graph) — treat as a weak lead",
    # because the table had no `rta` key. Measured: 628 of 628 candidates, all mislabelled.
    # It errs safe, which is precisely why nobody noticed for the life of the table.
    if fidelity in FIDELITY_BAR:
        fbar = FIDELITY_BAR[fidelity]
    else:
        fbar = (f" NOTE: UNRECOGNISED fidelity {fidelity!r} — this gate has no bar for it, so it "
                "is being treated as the weakest. Add it to FIDELITY_BAR rather than reading "
                "this note as a statement about the evidence.")
    computable = docs["_dead.json"].get("reachability_computable", False)
    cands = []

    def add(rows, cls, family):
        for r in rows or []:
            cands.append({"family": family, "class": cls, "bar": BARS[cls] + fbar,
                          "symbol": r["symbol"], "file": r["file"], "line": r["line"]})

    if computable:
        add(docs["_dead.json"]["rows"], "dead-on-arrival", "A")
        add(docs["_test-only.json"]["rows"], "test-only", "A")
    for cl in docs.get("_duplicates.json", {}).get("clusters", []):
        m = cl["members"][0]
        cands.append({"family": "D", "class": "duplicated-impl", "bar": BARS["duplicated-impl"],
                      "symbol": " ≡ ".join(x["symbol"] for x in cl["members"]),
                      "file": m["file"], "line": m["line"]})
    for r in docs.get("_interfaces.json", {}).get("rows", []):
        if len(r["impls_src"]) == 1:
            cands.append({"family": "E", "class": "single-impl-interface",
                          "bar": BARS["single-impl-interface"], "symbol": r["interface"],
                          "file": r["file"], "line": r["line"]})

    # A family the map could not seed is a family that DID NOT RUN. It must be reported as
    # such, in the plan and then on the face of the sweep, because the alternative is a
    # missing input silently reading as a clean result.
    unseeded_families = sorted(set(unseeded.values()))
    prod, unclassified = production_files(repo)
    h_work = enumerate_h_worklist(repo, prod)
    h_required, h_deferred = split_worklist(h_work)

    # THE COMPLEMENT. Every production file no signal reached, not merely the changed ones.
    # `--since` was described here as the thing that measures the enumeration's blind spots; it
    # measures the CHANGED SUBSET of them, and the difference is not academic. Measured on
    # ghola @4f33b3c: `internal/bridge/bridge.go` — an unauthenticated localhost HTTP server
    # that performs arbitrary outbound fetches, the highest-consequence file in the repo —
    # matched no signal AND had not changed since the baseline, so it appeared in neither
    # `h_worklist` nor `h_unmatched_changes`, and the gate returned COMPLETE without it. It was
    # found by hand and produced the sweep's worst finding. A high-consequence file that is
    # both unenumerated and unchanged was invisible to every output this gate had.
    matched = {w["path"] for w in h_work}
    unmatched_all = [p for p in prod if p not in matched]

    return {"contract": "slop-gate/2", "sha": pinned_sha, "fidelity": fidelity,
            "reachability_computable": computable,
            "map_provenance": {"generator": docs["_dead.json"].get("generator"),
                               "contract_version": docs["_dead.json"].get("contract_version")},
            "unseeded_families": unseeded_families,
            "unseeded_detail": {k: f"family {v} has no map seed (magma does not emit {k})"
                                for k, v in unseeded.items()},
            "candidates": cands,
            "production_total": len(prod),
            "production_files": prod,
            "production_unclassified": unclassified,
            "h_worklist": h_work,
            "h_required": h_required,
            "h_deferred": h_deferred,
            "h_unmatched": unmatched_all,
            "h_unmatched_changes": unmatched_changes(repo, since) if since else [],
            "change_baseline": since,
            "instructions": "Read every h_required path — that tier is the floor and an unattested "
                            "one leaves an item open (exit 3). h_deferred is tier-2 volume. "
                            "EVERY path in `h_unmatched` must also be attested or waived: those are "
                            "the production files no signal reached, and the enumeration's silence "
                            "about them is not evidence. (family H is found by reading, not the "
                            "map). For each candidate, clear its `bar` before filing. Then write a "
                            "discharge.json {sha, read_paths:[...], families_not_run:[...], "
                            "coverage_waived:[...], candidates_filed:[{file,symbol}], "
                            "candidates_cleared:[{file,symbol}], candidates_refuted:[{file,symbol}]}"
                            " and run `gate.py verify`. "
                            "`coverage_waived` entries may be a bare path or {path, reason} — a "
                            "reason is OPTIONAL. Waiving is cheap on purpose: deciding not to read "
                            "a file is a normal, correct move and should cost nothing. It settles "
                            "the ACCOUNTING and leaves `coverage.repo` alone, because a waived file "
                            "genuinely was not read and the fraction is there to tell YOU what you "
                            "actually looked at. No coverage floor is enforced: there is no "
                            "defensible number, and a red build for reading 67%% instead of 90%% "
                            "would only teach you to waive to clear it. "
                            "`sha` must equal this plan's sha. "
                            "`candidates_filed` is what you actually ACCUSED; every entry must "
                            "also appear in candidates_cleared or the gate returns INCOMPLETE. "
                            "EVERY candidate must appear in candidates_cleared or "
                            "candidates_refuted — a candidate you looked at and discarded goes "
                            "in `candidates_refuted`; leaving it out of both is not a clean "
                            "sweep, it is an unfinished one. "
                            "`families_not_run` MUST list every family in unseeded_families."
                            + (f" NOT SEEDED BY THE MAP: families {', '.join(unseeded_families)}"
                               " — run them by hand or record them as NOT RUN with that reason;"
                               " they may not be reported as checked-clean."
                               if unseeded_families else "")}


def verify(plan_path, discharge_path):
    """The coverage guarantee. INCOMPLETE unless ALL of these hold.

    Every clause below was absent and is here because its absence was reproduced, not
    imagined. The docstring at the top of this file promised the FILED clause from the start
    while only the unread clause existed, so a sweep could clear nothing and still be
    certified COMPLETE with exit 0.
    """
    pl = json.loads(pathlib.Path(plan_path).read_text())
    dis = json.loads(pathlib.Path(discharge_path).read_text())

    reasons = []

    # 1. The discharge must belong to THIS plan. verify referenced neither sha nor contract,
    #    so a discharge from any other sweep satisfied any plan -- and stale artifacts
    #    demonstrably survive across sessions.
    plan_sha, dis_sha = pl.get("sha"), dis.get("sha")
    if not dis_sha:
        reasons.append("discharge has no `sha`; it cannot be shown to belong to this plan "
                       f"(plan sha {plan_sha!r})")
    elif dis_sha != plan_sha:
        reasons.append(f"discharge sha {dis_sha!r} != plan sha {plan_sha!r}: this discharge "
                       "belongs to a different sweep")

    # 2. A worklist that enumerated NOTHING cannot certify anything. H enumeration is
    #    vocabulary-bound and under-enumerates silently on an unfamiliar domain, which the
    #    signal table above says in as many words -- so an empty worklist is the one case
    #    where "nothing to read" must never read as "everything was read".
    if not pl["h_worklist"]:
        reasons.append("h_worklist is EMPTY: the plan enumerated no family-H path, so this "
                       "run proves nothing. Extend the signals via `.slop-h-signals` and "
                       "re-plan; do not accept a zero worklist as coverage")

    # Required paths are a floor and PARTIAL is not a discount on them: an unattested
    # blast-radius path is INCOMPLETE, exactly as an unattested worklist used to be. A plan
    # written before the split has no `h_required`, so everything in it is required — old
    # plans must not become easier to satisfy by being old.
    read = set(dis.get("read_paths", []))
    required = pl.get("h_required", pl["h_worklist"])
    deferred = pl.get("h_deferred", [])
    unread = [w for w in required if w["path"] not in read]
    deferred_unattested = [w for w in deferred if w["path"] not in read]

    # A changed file no signal reached is the enumeration's own blind spot, so it cannot be
    # deferred on a consequence argument — the whole point is that nobody assessed its
    # consequence. Attest it or waive it explicitly; silence is not an option, because silence
    # is what the vocabulary gap already produced.
    # A waiver may be a bare path or {path, reason}; the reason is optional by policy. Waiving
    # is deliberately cheap — what it buys is the ACCOUNTING, never the coverage number.
    waived = set()
    for w in dis.get("coverage_waived", []):
        waived.add(w.get("path") if isinstance(w, dict) else w)
    waived.discard(None)

    holes = [u for u in pl.get("h_unmatched_changes", [])
             if u["path"] not in read and u["path"] not in waived]
    if holes:
        reasons.append(f"{len(holes)} changed file(s) that no H signal reached went neither "
                       "attested nor waived. The enumeration never saw them, so their absence "
                       "from the worklist says nothing about them. Read each, or list it in "
                       "`coverage_waived`")

    # THE COMPLEMENT CLAUSE. Same argument as the one above, minus the baseline restriction
    # that made it a subset. A production file no signal reached is undispositioned until the
    # sweep says something about it; before this, matching no signal meant a file was never
    # raised at all, so the gate could not tell "cleared" from "never existed". ghola's
    # `internal/bridge/bridge.go` sat in exactly that gap and the run still certified COMPLETE.
    undispositioned = [p for p in pl.get("h_unmatched", [])
                       if p not in read and p not in waived]
    if undispositioned:
        reasons.append(
            f"{len(undispositioned)} production file(s) no H signal reached are still "
            "unread. A signal miss is not a clearance — nothing has looked at them yet, so "
            "they are the natural next place to spend time. Read what is worth reading and "
            "waive the rest in `coverage_waived` (a reason is optional; waived counts as "
            "unread in `coverage.repo`, which is the point)")
    if unread:
        reasons.append(f"{len(unread)} REQUIRED family-H path(s) unattested: the sweep did not "
                       "look at a blast-radius path")

    # A family the map could not seed must be ACKNOWLEDGED as not-run, not merely printed at
    # it. `plan` emitted unseeded_families and told the sweep in prose that they "may not be
    # reported as checked-clean" -- and prose is not a gate. That is the same defect this
    # function was just repaired for: a condition promised in text and enforced nowhere, so a
    # sweep could be certified COMPLETE while silently treating those families as clean.
    unseeded = set(pl.get("unseeded_families", []))
    declared = set(dis.get("families_not_run", []))
    unacknowledged = sorted(unseeded - declared)
    if unacknowledged:
        reasons.append(
            f"families {', '.join(unacknowledged)} had no map seed and the discharge does not "
            f"list them in `families_not_run`. They were not run; say so, or run them by hand "
            f"and drop them from the plan's unseeded set. They may not read as checked-clean")

    # 3. Every candidate the sweep FILED must have cleared its bar. Note this is the filed
    #    set, not every candidate: most candidates are correctly discarded (21 of 23 dead
    #    rows in one real map were test mocks), so requiring all of them would make every
    #    sweep INCOMPLETE and the gate would be ignored. The discharge had no notion of
    #    "filed" at all, which is why the promised clause was unimplementable as written.
    cleared = {(c.get("file"), c.get("symbol")) for c in dis.get("candidates_cleared", [])}
    filed = [c for c in dis.get("candidates_filed", []) if isinstance(c, dict)]
    filed_unbarred = [c for c in filed if (c.get("file"), c.get("symbol")) not in cleared]
    if filed_unbarred:
        reasons.append(f"{len(filed_unbarred)} FILED candidate(s) did not clear their bar: "
                       "an accusation was made without the evidence its class requires")

    # 4. Every candidate the plan raised must be ACCOUNTED FOR — cleared (and so fileable) or
    #    explicitly refuted. Clause 3 above only bites when the sweep FILES something, so the
    #    clean-sweep path — file nothing, clear nothing, attest the reads — was certified
    #    COMPLETE with every candidate unexamined. Reproduced on a real plan before this was
    #    written; see the module docstring.
    #    Requiring every candidate to CLEAR would swing too far and make every sweep INCOMPLETE
    #    (21 of 23 dead rows in one real map were test mocks), which is precisely why
    #    accounting could not be required before: the discharge had no way to say "looked,
    #    discarded". `candidates_refuted` is that way, and it is cheap. What stops being free
    #    is discarding a candidate SILENTLY.
    refuted = {(c.get("file"), c.get("symbol"))
               for c in dis.get("candidates_refuted", []) if isinstance(c, dict)}
    accounted = cleared | refuted
    unaccounted = [c for c in pl["candidates"] if (c["file"], c["symbol"]) not in accounted]
    if unaccounted:
        reasons.append(f"{len(unaccounted)} candidate(s) were neither cleared nor refuted: the "
                       "plan raised them and the sweep never says what became of them. Clear "
                       "each one's bar, or list it in `candidates_refuted` to record that you "
                       "looked and discarded it. A sweep that files nothing is not clean by "
                       "default")

    # TWO FRACTIONS, NO VERDICT WORD. COMPLETE/PARTIAL/INCOMPLETE compressed two independent
    # quantities — how much of the PLAN was dispositioned, and how much of the REPO was read —
    # into one token, and a reader takes the token. They are not the same number and the gap
    # between them is where the failure lived: ghola @4f33b3c scored 10/10 on the first and
    # ~16/24 on the second, and reported COMPLETE. Coverage of the enumeration is not coverage
    # of the repository, so the output no longer offers a word that can be mistaken for either.
    #
    # THIS IS AN INSTRUMENT READING, NOT A SCORE. It exists so the person doing the sweep can
    # see where they actually are — nobody is being graded, and there is no adversary to
    # design against. Waived files count as UNREAD for that reason and no other: choosing not
    # to read a file is a normal, correct move, and a fraction that quietly counted it as
    # covered would be lying to the only person who reads it. Waivers stay free of a written
    # justification because charging for a routine decision buys nothing.
    prod_all = pl.get("production_files") or []
    prod_read = [p for p in prod_all if p in read]
    repo_pct = round(100.0 * len(prod_read) / len(prod_all), 1) if prod_all else None

    enum_items = len(required) + len(deferred) + len(pl.get("h_unmatched", []))
    enum_open = len(unread) + len(deferred_unattested) + len(undispositioned)
    enum_pct = round(100.0 * (enum_items - enum_open) / enum_items, 1) if enum_items else None

    result = {"plan_sha": plan_sha,
              "coverage": {
                  "repo": f"{len(prod_read)}/{len(prod_all)}",
                  "repo_pct": repo_pct,
                  "repo_note": "production source files attested. Waived files count as UNREAD. "
                               "This is the number a reader means by 'was the repo covered'.",
                  "plan": f"{enum_items - enum_open}/{enum_items}",
                  "plan_pct": enum_pct,
                  "plan_note": "items the plan raised that were dispositioned. High here and low "
                               "in `repo` means the enumeration was narrow, not that the repo is "
                               "clean.",
                  "waived": len(waived),
                  "unclassified": len(pl.get("production_unclassified", [])),
              },
              "h_worklist_total": len(pl["h_worklist"]),
              "h_required_total": len(required),
              "h_paths_attested": len(pl["h_worklist"]) - len(unread) - len(deferred_unattested),
              "h_required_unattested": unread,
              "h_deferred_unattested": len(deferred_unattested),
              "change_baseline": pl.get("change_baseline"),
              "unmatched_changes_total": len(pl.get("h_unmatched_changes", [])),
              "unmatched_changes_open": [u["path"] for u in holes],
              "candidates_total": len(pl["candidates"]),
              "candidates_cleared": len(cleared),
              "candidates_refuted": len(refuted),
              "candidates_filed": len(filed),
              "filed_without_bar": [f'{c.get("symbol")}' for c in filed_unbarred],
              "candidates_unaccounted": [f'{c["class"]} {c["symbol"]}' for c in unaccounted],
              "unread_unmatched": undispositioned[:50],
              "unread_unmatched_total": len(undispositioned),
              "unseeded_families": sorted(unseeded),
              "families_declared_not_run": sorted(declared),
              # A WORK QUEUE, NOT A CHARGE SHEET. Each entry is the next thing worth doing,
              # phrased as an action. This list used to be `incomplete_because` and read as a
              # list of failings; it is the same data and the affordance is the opposite one.
              "remaining": reasons}

    # ONE BINARY MACHINE SIGNAL, about bookkeeping only — which is all an exit code can carry
    # honestly. It means "there are still items on the list", the way a test runner means
    # "there are still failures": useful to a script, not a judgement about the person running
    # it. It says NOTHING about whether the repo was covered; that is a fraction, and fractions
    # do not fit in a byte. PARTIAL is gone with the other two words — deferral is now visible
    # as the gap between the two fractions rather than as a third verdict, and on a small
    # worklist it was unreachable anyway (ghola: h_deferred empty, one reachable state).
    result["status"] = "open" if reasons else "settled"
    result["headline"] = (
        f"read {result['coverage']['repo']} source files"
        + (f" ({repo_pct}%)" if repo_pct is not None else "")
        + f" · {result['coverage']['plan']} of the plan dispositioned"
        + (f" · {len(waived)} waived (count as unread)" if waived else "")
        + (f" · {len(reasons)} item(s) still open" if reasons else " · nothing left open"))
    print(json.dumps(result, indent=1))
    sys.exit(3 if reasons else 0)


if __name__ == "__main__":
    argv, since = list(sys.argv), None
    if "--since" in argv:
        i = argv.index("--since")
        if i + 1 >= len(argv):
            die("--since needs a git ref", 2)
        since = argv[i + 1]
        del argv[i:i + 2]

    if len(argv) >= 2 and argv[1] == "plan" and len(argv) == 5:
        print(json.dumps(plan(argv[2], argv[3], argv[4], since), indent=1))
    elif len(argv) == 4 and argv[1] == "verify":
        verify(argv[2], argv[3])
    else:
        die("usage: gate.py plan <magma-map-dir> <pinned-sha> <repo> [--since <ref>] | "
            "gate.py verify <plan.json> <discharge.json>\n"
            "  verify exits 0 when the ACCOUNTING is clean and 3 when it is open. It emits no "
            "verdict word: coverage is reported as two fractions (`coverage.repo`, "
            "`coverage.plan`) because they are different numbers and one word cannot carry both.\n"
            "  --since additionally flags changed production files no H signal reached. It "
            "measures the CHANGED SUBSET of the enumeration's blind spots, bounded by the "
            "baseline — `h_unmatched` is the unbounded set, and is the one that is enforced.", 2)

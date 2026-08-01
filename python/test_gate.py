#!/usr/bin/env python3
"""Tests for gate.py — the coded codemap->sweep seam. Each test names a fail-closed
property the abort required: refuse on wrong tree / wrong shape / missing map, and return
INCOMPLETE when a family-H path went unread.

    python3 test_gate.py
"""
import json, os, pathlib, subprocess, sys, tempfile, unittest

GATE = pathlib.Path(__file__).resolve().parent / "gate.py"


def write_map(d, sha="abc123", contract="codemap-rows/1", fidelity="reachability",
              computable=True, dead_rows=None):
    """A minimal codemap output dir with the four JSON files."""
    d = pathlib.Path(d)
    d.mkdir(parents=True, exist_ok=True)
    head = {"contract_version": contract, "generator": "codemap/4.2", "sha": sha,
            "fidelity": fidelity, "reachability_computable": computable}
    (d / "_dead.json").write_text(json.dumps({**head, "rows": dead_rows if computable else None}))
    (d / "_test-only.json").write_text(json.dumps({**head, "rows": [] if computable else None}))
    (d / "_duplicates.json").write_text(json.dumps({**head, "clusters": []}))
    (d / "_interfaces.json").write_text(json.dumps({**head, "rows": []}))
    return d


def git_repo(files):
    d = pathlib.Path(tempfile.mkdtemp(prefix="gate-repo-"))
    for rel, body in files.items():
        p = d / rel
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(body)
    subprocess.run(["git", "-C", str(d), "init", "-q"], check=True)
    subprocess.run(["git", "-C", str(d), "add", "-A"], check=True)
    subprocess.run(["git", "-C", str(d), "-c", "user.email=t@t", "-c", "user.name=t",
                    "commit", "-qm", "x"], check=True)
    return d


def run(*args):
    return subprocess.run([sys.executable, str(GATE), *args], capture_output=True, text=True)


class Base(unittest.TestCase):
    def setUp(self):
        self.trash = []

    def tearDown(self):
        import shutil
        for d in self.trash:
            shutil.rmtree(d, ignore_errors=True)

    def tmp(self, prefix):
        d = pathlib.Path(tempfile.mkdtemp(prefix=prefix))
        self.trash.append(d)
        return d


class TestPlanRefusals(Base):
    def test_refuses_on_wrong_tree_sha(self):
        """The map must describe the tree the sweep pinned. A sha mismatch is a map of a
        different tree — refuse, don't seed stale rows."""
        m = write_map(self.tmp("gate-map-") / "m", sha="OLDSHA")
        repo = self.tmp("gate-src-")
        p = run("plan", str(m), "NEWSHA", str(repo))
        self.assertEqual(p.returncode, 3)
        self.assertIn("different tree", p.stderr)

    def test_refuses_on_unsupported_contract(self):
        """A shape the gate can't parse must fail loud (like codemap does against deadcode),
        never silently mis-parse — the un-versioned-contract drift the abort named."""
        m = write_map(self.tmp("gate-map-") / "m", contract="codemap-rows/99")
        p = run("plan", str(m), "abc123", str(self.tmp("gate-src-")))
        self.assertEqual(p.returncode, 3)
        self.assertIn("contract_version", p.stderr)

    def test_refuses_on_missing_map(self):
        p = run("plan", str(self.tmp("gate-empty-")), "abc123", str(self.tmp("gate-src-")))
        self.assertEqual(p.returncode, 3)
        self.assertIn("missing", p.stderr)


class TestPlan(Base):
    def test_dead_rows_become_candidates_with_a_bar(self):
        m = write_map(self.tmp("gate-map-") / "m",
                      dead_rows=[{"symbol": "x::Orphan", "file": "x/x.go", "line": 3}])
        repo = git_repo({"go.mod": "module x\n", "x/x.go": "package x\n"})
        self.trash.append(repo)
        p = run("plan", str(m), "abc123", str(repo))
        self.assertEqual(p.returncode, 0, p.stderr)
        plan = json.loads(p.stdout)
        cand = [c for c in plan["candidates"] if "Orphan" in c["symbol"]]
        self.assertTrue(cand, "dead row did not become a candidate")
        self.assertEqual(cand[0]["class"], "dead-on-arrival")
        self.assertIn("refuter", cand[0]["bar"])

    def test_weaker_fidelity_adds_a_heavier_bar(self):
        """A `_dead` from an export-graph or heuristic tool must earn a heavier refuter than
        one from a real call graph."""
        m = write_map(self.tmp("gate-map-") / "m", fidelity="heuristic",
                      dead_rows=[{"symbol": "x::G", "file": "x/x.go", "line": 1}])
        repo = git_repo({"go.mod": "module x\n", "x/x.go": "package x\n"})
        self.trash.append(repo)
        plan = json.loads(run("plan", str(m), "abc123", str(repo)).stdout)
        self.assertIn("heuristic", plan["candidates"][0]["bar"])

    def test_h_worklist_enumerates_high_consequence_paths(self):
        m = write_map(self.tmp("gate-map-") / "m", dead_rows=[])
        repo = git_repo({"go.mod": "module x\n",
                         "internal/wallet/send.go": "package wallet\n",
                         "internal/auth/session.go": "package auth\n",
                         "internal/util/strings.go": "package util\n"})
        self.trash.append(repo)
        plan = json.loads(run("plan", str(m), "abc123", str(repo)).stdout)
        paths = {w["path"] for w in plan["h_worklist"]}
        self.assertIn("internal/wallet/send.go", paths)
        self.assertIn("internal/auth/session.go", paths)
        self.assertNotIn("internal/util/strings.go", paths, "benign path must not be on the worklist")

    def test_h_worklist_reaches_network_io_paths(self):
        """Measured on `ghola` @4f33b3c — one of the two pre-registered control repos — which
        enumerated **0** H-paths and so could never reach a verdict at all. ghola is an HTTP
        fetch client whose entire surface is parsing untrusted remote responses, and not one of
        client/download/stream/redirect/header/cookie/tls was in the vocabulary. The comment on
        `_H` warns that path-based enumeration under-enumerates SILENTLY on an unfamiliar
        domain; this is that warning firing on the project's own control."""
        m = write_map(self.tmp("gate-map-") / "m", dead_rows=[])
        repo = git_repo({"go.mod": "module x\n",
                         "internal/client/download.go": "package client\n",
                         "internal/client/stream.go": "package client\n",
                         "internal/transport/redirect.go": "package transport\n",
                         "internal/util/strings.go": "package util\n"})
        self.trash.append(repo)
        paths = {w["path"] for w in json.loads(run("plan", str(m), "abc123", str(repo)).stdout)["h_worklist"]}
        self.assertIn("internal/client/download.go", paths)
        self.assertIn("internal/client/stream.go", paths)
        self.assertIn("internal/transport/redirect.go", paths)
        self.assertNotIn("internal/util/strings.go", paths, "benign path must stay off the worklist")

    def test_h_worklist_reaches_authorization_and_financial_paths(self):
        """roboticus @443681b9: `internal/agent/policy/` — 15 production files including
        `engine_financial_config_rules.go` — matched NOTHING. A money vocabulary listing
        pay|ledger|billing|fee|gas|price does not contain the word "financial", and an
        authorization vocabulary listing auth|acl|rbac does not contain "policy"."""
        m = write_map(self.tmp("gate-map-") / "m", dead_rows=[])
        repo = git_repo({"go.mod": "module x\n",
                         "internal/agent/policy/engine_financial_config_rules.go": "package p\n",
                         "internal/agent/policy/denial_copy.go": "package p\n",
                         "internal/llm/provider_ratelimit.go": "package llm\n",
                         "internal/pipeline/turn_diagnostics_db.go": "package pipeline\n",
                         "internal/util/strings.go": "package util\n"})
        self.trash.append(repo)
        paths = {w["path"] for w in json.loads(run("plan", str(m), "abc123", str(repo)).stdout)["h_worklist"]}
        for p in ["internal/agent/policy/engine_financial_config_rules.go",
                  "internal/agent/policy/denial_copy.go",
                  "internal/llm/provider_ratelimit.go",
                  "internal/pipeline/turn_diagnostics_db.go"]:
            self.assertIn(p, paths)
        self.assertNotIn("internal/util/strings.go", paths, "benign path must stay off the worklist")

    def test_h_worklist_excludes_tests_and_docs(self):
        """H is production logic. A `_test.go` on an auth path and an auth `.md` doc are not
        H-read targets — including them inflates the worklist and would let a test discharge
        a coverage item. (Found by dogfooding roboticus: 266 worklist was test/doc-inflated.)"""
        m = write_map(self.tmp("gate-map-") / "m", dead_rows=[])
        repo = git_repo({"go.mod": "module x\n",
                         "internal/auth/session.go": "package auth\n",
                         "internal/auth/session_test.go": "package auth\n",
                         "docs/auth-policy.md": "# auth\n"})
        self.trash.append(repo)
        paths = {w["path"] for w in json.loads(run("plan", str(m), "abc123", str(repo)).stdout)["h_worklist"]}
        self.assertIn("internal/auth/session.go", paths)
        self.assertNotIn("internal/auth/session_test.go", paths, "test file must not be H-required")
        self.assertNotIn("docs/auth-policy.md", paths, "doc must not be H-required")


class TestVerifyCoverage(Base):
    def _plan_with_worklist(self):
        m = write_map(self.tmp("gate-map-") / "m", dead_rows=[])
        repo = git_repo({"go.mod": "module x\n", "internal/ledger/book.go": "package ledger\n"})
        self.trash.append(repo)
        return json.loads(run("plan", str(m), "abc123", str(repo)).stdout)

    def test_verify_leaves_the_item_OPEN_when_an_H_path_went_unread(self):
        """An enumerated H path with no read attached is still an open item: nothing has
        looked at it. exit code 3, status "open". Amended 2026-08-01 with the removal of the
        verdict triple — the behaviour is unchanged, the word INCOMPLETE is gone."""
        plan = self._plan_with_worklist()
        pf = self.tmp("gate-pf-") / "plan.json"; pf.write_text(json.dumps(plan))
        df = self.tmp("gate-df-") / "discharge.json"
        df.write_text(json.dumps({"sha": "abc123", "read_paths": [], "candidates_cleared": []}))
        p = run("verify", str(pf), str(df))
        self.assertEqual(p.returncode, 3, "an unread H path must leave an item open")
        out = json.loads(p.stdout)
        self.assertEqual(out["status"], "open")
        self.assertNotIn("verdict", out, "the verdict triple must not come back")
        self.assertTrue(any("ledger" in w["path"] for w in out["h_required_unattested"]))

    def test_verify_SETTLES_when_every_H_path_was_read(self):
        plan = self._plan_with_worklist()
        pf = self.tmp("gate-pf-") / "plan.json"; pf.write_text(json.dumps(plan))
        df = self.tmp("gate-df-") / "discharge.json"
        df.write_text(json.dumps({"sha": "abc123",
                                  "read_paths": [w["path"] for w in plan["h_worklist"]],
                                  "candidates_cleared": []}))
        p = run("verify", str(pf), str(df))
        self.assertEqual(p.returncode, 0, p.stdout)
        out = json.loads(p.stdout)
        self.assertEqual(out["status"], "settled")
        self.assertNotIn("verdict", out, "the verdict triple must not come back")



class TestChangeSetCrossCheck(Base):
    """The gate could not measure its own vocabulary coverage.

    REPRODUCED on roboticus @443681b9, 2026-08-01: of the 6 production .go files changed in
    the last 12 commits of release work, **0** were on the 387-path H worklist and 0 were in
    the 129-path required tier — including `engine_financial_config_rules.go`, a financial
    rules engine missed by a money vocabulary that lists pay|ledger|billing|fee|gas|price.
    `internal/agent/policy/` (15 production files) matches nothing at ANY anchor.

    Extending the vocabulary fixes the instance and not the class: the next unenumerated
    subsystem is just as silent, and nothing checks whether a `.slop-h-signals` addition was
    sufficient. The cross-check inverts it — compare the enumeration against a set already
    known to matter (what changed), and report what it failed to reach. The blind spot becomes
    a number printed every sweep instead of an unfalsifiable worry.
    """

    def _repo_with_history(self):
        repo = git_repo({"go.mod": "module x\n",
                         "internal/wallet/pay.go": "package wallet\n"})
        (repo / "internal" / "agent").mkdir(parents=True)
        (repo / "internal" / "agent" / "widget_shape.go").write_text("package agent\n")
        subprocess.run(["git", "-C", str(repo), "add", "-A"], check=True)
        subprocess.run(["git", "-C", str(repo), "-c", "user.email=t@t", "-c", "user.name=t",
                        "commit", "-qm", "second"], check=True)
        self.trash.append(repo)
        return repo

    def _plan(self, repo, *extra):
        m = write_map(self.tmp("gate-map-") / "m", dead_rows=[])
        return json.loads(run("plan", str(m), "abc123", str(repo), *extra).stdout)

    def test_a_changed_file_no_signal_matched_is_reported(self):
        repo = self._repo_with_history()
        plan = self._plan(repo, "--since", "HEAD~1")
        unmatched = {u["path"] for u in plan["h_unmatched_changes"]}
        self.assertIn("internal/agent/widget_shape.go", unmatched,
                      "a changed production file no signal reached must be reported, not silently absent")

    def test_without_since_there_is_no_cross_check(self):
        """Backward compatible: a whole-repo sweep has no change set to compare against."""
        repo = self._repo_with_history()
        self.assertEqual(self._plan(repo)["h_unmatched_changes"], [])

    def test_an_unattested_unmatched_change_is_INCOMPLETE(self):
        repo = self._repo_with_history()
        plan = self._plan(repo, "--since", "HEAD~1")
        pf = self.tmp("gate-pf-") / "plan.json"; pf.write_text(json.dumps(plan))
        df = self.tmp("gate-df-") / "d.json"
        df.write_text(json.dumps({"sha": "abc123",
                                  "read_paths": [w["path"] for w in plan["h_worklist"]],
                                  "candidates_cleared": []}))
        r = run("verify", str(pf), str(df))
        self.assertEqual(r.returncode, 3, r.stdout)
        self.assertIn("no H signal reached", r.stdout)

    def test_attesting_the_unmatched_change_clears_it(self):
        repo = self._repo_with_history()
        plan = self._plan(repo, "--since", "HEAD~1")
        pf = self.tmp("gate-pf-") / "plan.json"; pf.write_text(json.dumps(plan))
        df = self.tmp("gate-df-") / "d.json"
        df.write_text(json.dumps(
            {"sha": "abc123",
             "read_paths": [w["path"] for w in plan["h_worklist"]]
                           + [u["path"] for u in plan["h_unmatched_changes"]],
             "candidates_cleared": []}))
        self.assertEqual(run("verify", str(pf), str(df)).returncode, 0)

    def test_an_unmatched_change_can_be_WAIVED_explicitly(self):
        """Not every changed file is H. The escape must exist or the gate is unusable on a
        large diff — but it must be an explicit act, recorded in the discharge, not a default."""
        repo = self._repo_with_history()
        plan = self._plan(repo, "--since", "HEAD~1")
        pf = self.tmp("gate-pf-") / "plan.json"; pf.write_text(json.dumps(plan))
        df = self.tmp("gate-df-") / "d.json"
        df.write_text(json.dumps({"sha": "abc123",
                                  "read_paths": [w["path"] for w in plan["h_worklist"]],
                                  "coverage_waived": ["internal/agent/widget_shape.go"],
                                  "candidates_cleared": []}))
        self.assertEqual(run("verify", str(pf), str(df)).returncode, 0)


class TestTieredWorklist(Base):
    """Tier 1 was unbounded, and the cost was measured rather than argued: roboticus
    @443681b9 enumerates 387 H-paths, every one of which `verify` required before it would
    return anything but INCOMPLETE. On the repo the method was validated against, no honest
    sweep could reach a verdict — so the verdict carried no information and the only way to a
    COMPLETE was the attestation hole. The split is by CONSEQUENCE, which is family H's own
    selection rule: "if this were wrong, what would it cost?"
    """

    def _repo(self):
        """Padded past H_DEFER_FLOOR so the tier split actually engages. Below the floor the
        whole worklist is required and there is nothing to test here."""
        files = {"go.mod": "module x\n",
                 "internal/wallet/pay.go": "package wallet\n",      # tier 1 money
                 "internal/auth/session.go": "package auth\n",      # tier 1 auth
                 "internal/store/rows.go": "package store\n",       # tier 2 persistence
                 "internal/client/fetch.go": "package client\n"}    # tier 2 network
        for i in range(60):                                         # tier 2 padding
            files[f"internal/parse/decode{i}.go"] = "package parse\n"
        return git_repo(files)

    def _plan(self, repo):
        m = write_map(self.tmp("gate-map-") / "m", dead_rows=[])
        return json.loads(run("plan", str(m), "abc123", str(repo)).stdout)

    def test_plan_splits_the_worklist_by_consequence(self):
        repo = self._repo(); self.trash.append(repo)
        plan = self._plan(repo)
        req = {w["path"] for w in plan["h_required"]}
        dfr = {w["path"] for w in plan["h_deferred"]}
        self.assertEqual(req, {"internal/wallet/pay.go", "internal/auth/session.go"},
                         "only blast-radius paths are required")
        self.assertIn("internal/store/rows.go", dfr)
        self.assertIn("internal/client/fetch.go", dfr)
        self.assertEqual(len(plan["h_worklist"]), len(req) + len(dfr), "the split must partition")
        self.assertEqual(req & dfr, set(), "no path may be both")

    def test_deferred_paths_show_in_the_FRACTION_not_in_a_third_verdict(self):
        """PARTIAL is gone. It existed so "blast radius covered, volume deferred" was sayable
        at all, and two fractions say it better: the deferral is visible as the gap in
        coverage.plan instead of as a word. It was also unreachable on any worklist at or
        below H_DEFER_FLOOR (ghola: h_deferred empty, so the verdict had one live state)."""
        repo = self._repo(); self.trash.append(repo)
        plan = self._plan(repo)
        pf = self.tmp("gate-pf-") / "plan.json"; pf.write_text(json.dumps(plan))
        df = self.tmp("gate-df-") / "d.json"
        df.write_text(json.dumps({"sha": "abc123",
                                  "read_paths": [w["path"] for w in plan["h_required"]],
                                  "candidates_cleared": []}))
        r = run("verify", str(pf), str(df))
        self.assertEqual(r.returncode, 0, r.stdout)
        out = json.loads(r.stdout)
        self.assertEqual(out["status"], "settled", "deferral is not an open item")
        self.assertNotIn("verdict", out)
        self.assertEqual(out["h_deferred_unattested"], len(plan["h_deferred"]))
        self.assertGreater(out["h_deferred_unattested"], 0, "the split must have deferred something")
        done, total = out["coverage"]["plan"].split("/")
        self.assertLess(int(done), int(total),
                        "an unattested deferred path must LOWER coverage.plan — that is the "
                        "signal PARTIAL used to carry, and it is now a number")

    def test_attesting_everything_settles_and_fills_the_fraction(self):
        repo = self._repo(); self.trash.append(repo)
        plan = self._plan(repo)
        pf = self.tmp("gate-pf-") / "plan.json"; pf.write_text(json.dumps(plan))
        df = self.tmp("gate-df-") / "d.json"
        df.write_text(json.dumps({"sha": "abc123",
                                  "read_paths": [w["path"] for w in plan["h_worklist"]],
                                  "candidates_cleared": []}))
        r = run("verify", str(pf), str(df))
        self.assertEqual(r.returncode, 0, r.stdout)
        out = json.loads(r.stdout)
        self.assertEqual(out["status"], "settled")
        done, total = out["coverage"]["plan"].split("/")
        self.assertEqual(done, total, "nothing left undispositioned")

    def test_missing_a_REQUIRED_path_leaves_an_item_open(self):
        """Required paths are a floor, not a discount. Skipping a blast-radius path is the
        thing the worklist exists to surface, and it stays an open item."""
        repo = self._repo(); self.trash.append(repo)
        plan = self._plan(repo)
        pf = self.tmp("gate-pf-") / "plan.json"; pf.write_text(json.dumps(plan))
        df = self.tmp("gate-df-") / "d.json"
        df.write_text(json.dumps({"sha": "abc123",
                                  "read_paths": [plan["h_required"][0]["path"]],
                                  "candidates_cleared": []}))
        r = run("verify", str(pf), str(df))
        self.assertEqual(r.returncode, 3, r.stdout)
        self.assertEqual(json.loads(r.stdout)["status"], "open")

    def test_a_SMALL_worklist_is_never_deferred(self):
        """REGRESSION, caught 2026-08-01 while measuring the vocabulary fix. The floor was a
        ZERO-check ("no tier-1 ⇒ require everything"), so ONE incidental tier-1 match collapsed
        it. Measured on ghola: adding `ratelimit` to the money vocabulary matched
        `internal/client/ratelimit.go`, and required went 10 → 1 with 9 deferred — on a repo
        whose whole H surface is 10 files and trivially readable in one pass.

        Deferral exists to make a LARGE worklist tractable. Below the size where a full read is
        feasible it buys nothing and costs coverage, so the floor must be a size check."""
        repo = git_repo({"go.mod": "module x\n",
                         "internal/wallet/pay.go": "package wallet\n",     # tier 1
                         "internal/client/fetch.go": "package client\n",   # tier 2
                         "internal/store/rows.go": "package store\n"})     # tier 2
        self.trash.append(repo)
        plan = self._plan(repo)
        self.assertEqual(len(plan["h_required"]), 3,
                         "a 3-path worklist has nothing worth deferring")
        self.assertEqual(plan["h_deferred"], [])

    def test_a_repo_with_no_blast_radius_paths_requires_its_WHOLE_worklist(self):
        """ghola's shape: every H-path is network/parse, so a tier-1-only required set would
        be EMPTY and the repo would be certifiable without reading anything. When there is no
        higher tier to defer to, the lower tier IS the required set."""
        repo = git_repo({"go.mod": "module x\n",
                         "internal/client/download.go": "package client\n",
                         "internal/client/stream.go": "package client\n"})
        self.trash.append(repo)
        plan = self._plan(repo)
        self.assertEqual(len(plan["h_required"]), 2, "no tier-1 path ⇒ tier 2 becomes required")
        self.assertEqual(plan["h_deferred"], [])


class TestMagmaSeam(unittest.TestCase):
    """The map->gate seam as magma actually emits it.

    This gate read the map ROOT for its whole life while magma has always written row files
    to `<map>/.magma/`, so it exited 3 on every real map and the family-H worklist was never
    fail-closed in practice. These pin the seam so it cannot silently break again.
    """

    def test_finds_row_files_in_the_magma_subdir(self):
        root = pathlib.Path(tempfile.mkdtemp(prefix="gate-map-"))
        write_map(root / ".magma", sha="deadbee")
        repo = git_repo({"internal/wallet/pay.go": "package wallet\n"})
        r = run("plan", str(root), "deadbee", str(repo))
        code, out, err = r.returncode, r.stdout, r.stderr
        self.assertEqual(code, 0, f"gate must read <map>/.magma/: {err}")
        self.assertEqual(json.loads(out)["sha"], "deadbee")

    def test_missing_optional_row_file_degrades_coverage_rather_than_failing(self):
        """magma emits no _interfaces/_duplicates yet. Absence must not fail the gate, and
        must not pass silently either: the families they seed are reported NOT SEEDED, so a
        sweep cannot record them as checked-clean."""
        root = pathlib.Path(tempfile.mkdtemp(prefix="gate-map-"))
        m = write_map(root / ".magma", sha="deadbee")
        (m / "_interfaces.json").unlink()
        (m / "_duplicates.json").unlink()
        repo = git_repo({"internal/wallet/pay.go": "package wallet\n"})
        r = run("plan", str(root), "deadbee", str(repo))
        code, out, err = r.returncode, r.stdout, r.stderr
        self.assertEqual(code, 0, f"optional row files must not fail the gate: {err}")
        plan = json.loads(out)
        self.assertEqual(plan["unseeded_families"], ["D", "E"])
        self.assertIn("NOT SEEDED", plan["instructions"])

    def test_missing_REQUIRED_row_file_still_refuses(self):
        root = pathlib.Path(tempfile.mkdtemp(prefix="gate-map-"))
        m = write_map(root / ".magma", sha="deadbee")
        (m / "_dead.json").unlink()
        repo = git_repo({"internal/wallet/pay.go": "package wallet\n"})
        r = run("plan", str(root), "deadbee", str(repo))
        code, err = r.returncode, r.stderr
        self.assertEqual(code, 3)
        self.assertIn("--force", err, "the message must name the magma freshness trap: "
                                      "freshness is keyed on the analysed repo's sha, not on "
                                      "magma's version, so an unchanged repo writes nothing")

    def test_dirty_tree_map_is_refused_and_says_why(self):
        """magma stamps a dirty tree as `<sha>+<diffhash>`, which can never equal a pinned
        commit. Gating on a dirty map is how two earlier sweeps recorded boundaries that do
        not resolve today: in-flight commits get amended or rebased away."""
        root = pathlib.Path(tempfile.mkdtemp(prefix="gate-map-"))
        write_map(root / ".magma", sha="deadbee+9f1c2a")
        repo = git_repo({"internal/wallet/pay.go": "package wallet\n"})
        r = run("plan", str(root), "deadbee", str(repo))
        code, err = r.returncode, r.stderr
        self.assertEqual(code, 3)
        self.assertIn("DIRTY", err)


class TestCoverageGuaranteeHolds(unittest.TestCase):
    """The three ways `verify` certified COMPLETE while proving nothing.

    Found by sweeping this skill with its own method, 2026-08-01, and each REPRODUCED before
    being fixed. The module docstring had promised the filed-candidate clause from the very
    first version while only the unread-path clause existed.
    """

    def _plan(self, **over):
        base = {"sha": "abc123", "h_worklist": [{"path": "internal/wallet/pay.go",
                                                 "reason": "money/value"}],
                "candidates": [{"family": "A", "class": "dead-on-arrival", "bar": "x",
                                "symbol": "Unused", "file": "a.go", "line": 1}]}
        base.update(over)
        d = pathlib.Path(tempfile.mkdtemp(prefix="gate-cov-"))
        (d / "plan.json").write_text(json.dumps(base))
        return d

    def _run(self, d, discharge):
        (d / "dis.json").write_text(json.dumps(discharge))
        return run("verify", str(d / "plan.json"), str(d / "dis.json"))

    def test_a_FILED_candidate_that_never_cleared_its_bar_is_INCOMPLETE(self):
        """628 candidates, 0 cleared, verdict COMPLETE, exit 0 -- reproduced against a real
        plan before this was fixed. An accusation without its class's evidence is the one
        thing this whole skill exists to prevent."""
        d = self._plan()
        r = self._run(d, {"sha": "abc123", "read_paths": ["internal/wallet/pay.go"],
                          "candidates_filed": [{"file": "a.go", "symbol": "Unused"}],
                          "candidates_cleared": []})
        self.assertEqual(r.returncode, 3, r.stdout)
        self.assertIn("did not clear their bar", r.stdout)

    def test_clearing_the_filed_candidate_is_COMPLETE(self):
        """The fix must not swing the other way: requiring EVERY candidate to clear would
        make every real sweep INCOMPLETE (21 of 23 dead rows in one real map were test
        mocks), and a gate that is always red is a gate nobody reads."""
        d = self._plan()
        r = self._run(d, {"sha": "abc123", "read_paths": ["internal/wallet/pay.go"],
                          "candidates_filed": [{"file": "a.go", "symbol": "Unused"}],
                          "candidates_cleared": [{"file": "a.go", "symbol": "Unused"}]})
        self.assertEqual(r.returncode, 0, r.stdout)

    def test_a_candidate_neither_cleared_nor_refuted_is_INCOMPLETE(self):
        """The clean-sweep hole. The filed-candidate clause only fires when the sweep FILES
        something, so a sweep that files nothing was certified COMPLETE with every candidate
        unexamined -- and "COMPLETE, no findings" is the single most consequential thing this
        skill emits, because it is the one a reader takes as "this repo is clean".

        REPRODUCED 2026-08-01 against a real counterspy @531cc42 plan: a discharge generated
        mechanically FROM the plan, with no file opened, returned 12 candidates_cleared: 0,
        verdict COMPLETE, exit 0. That is the same `628 candidates, 0 cleared, COMPLETE`
        defect the module docstring records as already repaired."""
        d = self._plan()
        r = self._run(d, {"sha": "abc123", "read_paths": ["internal/wallet/pay.go"],
                          "candidates_filed": [], "candidates_cleared": []})
        self.assertEqual(r.returncode, 3, r.stdout)
        self.assertIn("neither cleared nor refuted", r.stdout)

    def test_refuting_a_candidate_accounts_for_it(self):
        """The escape the old contract lacked. Requiring every candidate to CLEAR would make
        every sweep INCOMPLETE (21 of 23 dead rows in one real map were test mocks) -- which
        is exactly why accounting could not be required before. Refuting is the cheap explicit
        act that makes it requirable: say you looked and discarded it, and the gate is
        satisfied. Discarding silently is what stops being free."""
        d = self._plan()
        r = self._run(d, {"sha": "abc123", "read_paths": ["internal/wallet/pay.go"],
                          "candidates_filed": [],
                          "candidates_refuted": [{"file": "a.go", "symbol": "Unused"}]})
        self.assertEqual(r.returncode, 0, r.stdout)

    def test_the_H_evidence_is_reported_as_ATTESTED_not_as_proven(self):
        """`read_paths` is self-reported and nothing corroborates it, so the gate records an
        assertion. The module docstring claimed this "inverts the old gate ... to 'prove you
        looked'" -- a fabricated claim by this skill's own lexicon, in the skill's own
        coverage guarantee. The output key is the contract; pin it."""
        d = self._plan()
        r = self._run(d, {"sha": "abc123", "read_paths": ["internal/wallet/pay.go"],
                          "candidates_refuted": [{"file": "a.go", "symbol": "Unused"}]})
        out = json.loads(r.stdout)
        self.assertIn("h_paths_attested", out)
        self.assertNotIn("h_worklist_read", out, "'read' overstates a self-reported input")

    def test_an_EMPTY_worklist_cannot_certify_anything(self):
        """H enumeration is vocabulary-bound and under-enumerates SILENTLY on an unfamiliar
        domain. A zero worklist previously returned COMPLETE: nothing to read read as
        everything read."""
        d = self._plan(h_worklist=[], candidates=[])
        r = self._run(d, {"sha": "abc123", "read_paths": [], "candidates_cleared": []})
        self.assertEqual(r.returncode, 3, r.stdout)
        self.assertIn("EMPTY", r.stdout)

    def test_a_discharge_from_another_sweep_is_refused(self):
        """verify referenced neither sha nor contract, so any discharge naming the right
        paths satisfied any plan. Stale artifacts demonstrably survive across sessions."""
        d = self._plan()
        r = self._run(d, {"sha": "deadbee", "read_paths": ["internal/wallet/pay.go"],
                          "candidates_cleared": []})
        self.assertEqual(r.returncode, 3, r.stdout)
        self.assertIn("different sweep", r.stdout)

    def test_an_unseeded_family_must_be_ACKNOWLEDGED_not_merely_printed(self):
        """`plan` emitted unseeded_families and told the sweep in PROSE that they may not be
        reported as checked-clean. Prose is not a gate -- the same defect this function was
        just repaired for, reintroduced in the same function on the same day by the author of
        the repair. A family the map could not seed did not run; the discharge must say so."""
        d = self._plan(unseeded_families=["D", "E"])
        r = self._run(d, {"sha": "abc123", "read_paths": ["internal/wallet/pay.go"],
                          "candidates_cleared": []})
        self.assertEqual(r.returncode, 3, r.stdout)
        self.assertIn("families_not_run", r.stdout)

    def test_acknowledging_the_unseeded_families_is_COMPLETE(self):
        """Amended 2026-08-01: as written this asserted COMPLETE while leaving the plan's one
        candidate unaccounted, so it encoded the clean-sweep hole rather than the clause it
        names. The candidate is now refuted; the property under test is unchanged."""
        d = self._plan(unseeded_families=["D", "E"])
        r = self._run(d, {"sha": "abc123", "read_paths": ["internal/wallet/pay.go"],
                          "families_not_run": ["D", "E"], "candidates_cleared": [],
                          "candidates_refuted": [{"file": "a.go", "symbol": "Unused"}]})
        self.assertEqual(r.returncode, 0, r.stdout)

    def test_acknowledging_only_SOME_unseeded_families_still_refuses(self):
        """A partial acknowledgement is the shape that would otherwise slip through."""
        d = self._plan(unseeded_families=["D", "E"])
        r = self._run(d, {"sha": "abc123", "read_paths": ["internal/wallet/pay.go"],
                          "families_not_run": ["D"], "candidates_cleared": []})
        self.assertEqual(r.returncode, 3, r.stdout)
        self.assertIn("E", r.stdout)

    def test_a_discharge_with_no_sha_is_refused(self):
        d = self._plan()
        r = self._run(d, {"read_paths": ["internal/wallet/pay.go"], "candidates_cleared": []})
        self.assertEqual(r.returncode, 3, r.stdout)
        self.assertIn("no `sha`", r.stdout)


class _RetiredSkillVersionTests:
    """RETIRED 2026-08-01 with the extraction. `skill_version.py` stamped a digest over the
    DEPLOYED skill because that was the only copy — a stand-in for version control in a tree
    that had none. This repo is the version control, and `slop doctor` answers the question the
    digest could only gesture at: not "did something change" but "WHAT changed, and in which
    direction". Its coverage moves to TestInstall below."""

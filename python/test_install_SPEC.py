#!/usr/bin/env python3
"""Tests for install/doctor — the deployment half.

The property under test is the one whose absence produced a security-shaped defect: an install
is COMPLETE or it is not, and a half-install must be detectable. `/slop-ferret:report` was linked
and `/slop-ferret` was not, so the parent skill could not be invoked, so its `allowed-tools` never
applied and the withholding of Edit/Artifact was prose only. `doctor` has to catch that.
"""
import json, os, pathlib, shutil, subprocess, sys, tempfile, unittest

REPO = pathlib.Path(__file__).resolve().parent.parent


def run(*args, home=None):
    env = dict(os.environ)
    if home:
        env["HOME"] = str(home)
    return subprocess.run([sys.executable, "-m", "slop", *args],
                          capture_output=True, text=True, cwd=str(REPO), env=env)


class Base(unittest.TestCase):
    def setUp(self):
        self.home = pathlib.Path(tempfile.mkdtemp(prefix="slop-home-"))
        (self.home / ".claude").mkdir(parents=True)

    def tearDown(self):
        shutil.rmtree(self.home, ignore_errors=True)

    @property
    def dest(self):
        return self.home / ".claude" / "skills" / "slop-ferret"

    @property
    def cmds(self):
        return self.home / ".claude" / "commands"


class TestInstall(Base):
    def test_install_deploys_the_skill_and_BOTH_command_entries(self):
        r = run("install", home=self.home)
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertTrue((self.dest / "SKILL.md").is_file())
        self.assertTrue((self.dest / "references" / "ai-slop-lexicon.md").is_file())
        for name in ("slop-ferret.md", "slop-ferret/report.md"):
            self.assertTrue((self.cmds / name).is_symlink(),
                            f"{name} must be linked — installing one entry and not the other is "
                            f"the original defect")

    def test_doctor_CATCHES_a_half_install(self):
        """The exact 2026-08-01 state: report linked, parent missing."""
        run("install", home=self.home)
        (self.cmds / "slop-ferret.md").unlink()
        r = run("doctor", home=self.home)
        self.assertEqual(r.returncode, 1, r.stdout)
        self.assertIn("command entry missing", r.stdout)
        self.assertIn("allowed-tools never apply", r.stdout,
                      "doctor must say WHY a missing entry matters, not just that it is missing")

    def test_doctor_is_clean_right_after_install(self):
        run("install", home=self.home)
        r = run("doctor", home=self.home)
        self.assertEqual(r.returncode, 0, r.stdout)

    def test_install_REFUSES_to_clobber_a_hand_edited_deployed_file(self):
        """The failure this guards is 'I edited the deployed copy by mistake', not an attack.
        Overwriting silently would eat real work, so it stops and shows what would be lost."""
        run("install", home=self.home)
        (self.dest / "SKILL.md").write_text("# my in-progress edits\n")
        r = run("install", home=self.home)
        self.assertEqual(r.returncode, 3, r.stdout)
        self.assertIn("REFUSING", r.stdout)
        self.assertIn("SKILL.md", r.stdout)
        self.assertEqual((self.dest / "SKILL.md").read_text(), "# my in-progress edits\n",
                         "the hand edit must survive a refused install")

    def test_force_overwrites_after_being_told_what_is_lost(self):
        run("install", home=self.home)
        (self.dest / "SKILL.md").write_text("# my in-progress edits\n")
        self.assertEqual(run("install", "--force", home=self.home).returncode, 0)
        self.assertNotEqual((self.dest / "SKILL.md").read_text(), "# my in-progress edits\n")

    def test_doctor_names_the_file_that_was_edited_in_place(self):
        """What the digest could never do: say WHICH file, and in which direction."""
        run("install", home=self.home)
        (self.dest / "references" / "families.md").write_text("# edited\n")
        r = run("doctor", home=self.home)
        self.assertEqual(r.returncode, 1)
        self.assertIn("families.md", r.stdout)
        self.assertIn("edited in place", r.stdout)

    def test_doctor_reports_not_installed_rather_than_crashing(self):
        r = run("doctor", home=self.home)
        self.assertEqual(r.returncode, 1)
        self.assertIn("not installed", r.stdout)


if __name__ == "__main__":
    unittest.main()

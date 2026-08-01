package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robot-accomplice/slop-ferret/internal/gate"
	"github.com/robot-accomplice/slop-ferret/internal/install"
)

func init() { install.Embedded = skillFS }

func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestNoArgsPrintsUsage(t *testing.T) {
	code, _, errs := runCLI(t)
	if code != 2 || !strings.Contains(errs, "slop-ferret") {
		t.Fatalf("code=%d err=%q", code, errs)
	}
}

func TestUnknownSubcommandPrintsUsage(t *testing.T) {
	if code, _, _ := runCLI(t, "sweep-everything"); code != 2 {
		t.Fatalf("code=%d, want 2", code)
	}
}

// release.yml parses `--version` to check the tag against the binary, the way the sibling
// projects' release gate does. If this stops printing a parseable version, releases silently stop
// being verifiable — so the spelling is pinned by a test rather than by convention.
func TestVersionIsParseableByTheReleaseGate(t *testing.T) {
	for _, spelling := range []string{"version", "--version", "-v"} {
		code, out, _ := runCLI(t, spelling)
		if code != 0 {
			t.Fatalf("%s: code=%d", spelling, code)
		}
		fields := strings.Fields(out)
		if len(fields) < 2 || fields[0] != "slop-ferret" {
			t.Fatalf("%s: %q is not `slop-ferret <version> ...`", spelling, out)
		}
		if fields[1] != binVersion {
			t.Errorf("%s: field 2 = %q, want %q — release.yml reads this field",
				spelling, fields[1], binVersion)
		}
		// release.yml also reads field 6 for the embedded skill stamp. Both offsets are pinned
		// here because the release gate parses positionally: reword this line and the tag check
		// silently starts comparing the wrong token, which is how a release stops being verified
		// without anything going red.
		if len(fields) < 6 {
			t.Fatalf("%s: %q has fewer than 6 fields; release.yml reads field 6", spelling, out)
		}
		if want := install.SkillVersion(install.EmbeddedSource(binVersion)); fields[5] != want {
			t.Errorf("%s: field 6 = %q, want the skill stamp %q", spelling, fields[5], want)
		}
	}
}

func TestPlanRejectsWrongArity(t *testing.T) {
	if code, _, _ := runCLI(t, "plan", "only-one-arg"); code != 2 {
		t.Fatal("plan needs three positional args")
	}
}

func TestVerifyRejectsWrongArity(t *testing.T) {
	if code, _, _ := runCLI(t, "verify", "only-one"); code != 2 {
		t.Fatal("verify needs two positional args")
	}
}

func TestPlanSurfacesTheGatesExitCode(t *testing.T) {
	// A map dir that does not exist is a REFUSAL, not a usage error and not an unfinished sweep.
	// The three are separate exit codes precisely so a wrapping script can tell them apart.
	code, _, errs := runCLI(t, "plan", filepath.Join(t.TempDir(), "nope"), "abc123", t.TempDir())
	if code != gate.ExitRefused {
		t.Fatalf("code=%d, want ExitRefused (%d): %s", code, gate.ExitRefused, errs)
	}
}

func TestInstallFromRejectsADirWithNoSkillTree(t *testing.T) {
	code, _, errs := runCLI(t, "install", "--from", t.TempDir())
	if code != 2 || !strings.Contains(errs, "skill") {
		t.Fatalf("code=%d err=%q", code, errs)
	}
}

func TestDoctorReportsNotInstalledOnACleanHome(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	code, out, _ := runCLI(t, "doctor")
	if code != 1 || !strings.Contains(out, "not installed") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestInstallThenDoctorRoundTrips(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if code, out, errs := runCLI(t, "install"); code != 0 {
		t.Fatalf("install code=%d out=%s err=%s", code, out, errs)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "slop-ferret", "SKILL.md")); err != nil {
		t.Fatalf("skill not deployed: %v", err)
	}
	code, out, _ := runCLI(t, "doctor")
	if code != 0 {
		t.Fatalf("doctor after install: code=%d out=%s", code, out)
	}
}

// The embedded tree is the bootstrap floor; if it ever ships without the files the method needs,
// a fresh `go install` produces a sweep with no vocabulary.
func TestTheEmbeddedSkillCarriesWhatTheMethodNeeds(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	runCLI(t, "install")
	dest := filepath.Join(home, ".claude", "skills", "slop-ferret")
	for _, rel := range []string{
		"SKILL.md",
		"references/ai-slop-lexicon.md",
		"references/families.md",
		"commands/slop-ferret-report.md",
	} {
		if _, err := os.Stat(filepath.Join(dest, filepath.FromSlash(rel))); err != nil {
			t.Errorf("embedded skill is missing %s: %v", rel, err)
		}
	}
}

// `update` must leave the installed skill untouched when the fetch fails, and say so. A failed
// update that half-applies is worse than a stale one.
func TestUpdateFailsCleanlyAndSaysTheInstallIsUntouched(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	runCLI(t, "install")
	// An unresolvable ref against the real endpoint: no network mutation, no partial write.
	code, _, errs := runCLI(t, "update", "--ref", "refs/heads/definitely-not-a-branch-xyz")
	if code != 2 {
		t.Fatalf("code=%d, want 2", code)
	}
	if !strings.Contains(errs, "installed skill is untouched") {
		t.Errorf("a failed update must say the deployment was left alone: %q", errs)
	}
	if code2, out, _ := runCLI(t, "doctor"); code2 != 0 {
		t.Errorf("doctor after a failed update: code=%d out=%s", code2, out)
	}
}

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robot-accomplice/slop-ferret/internal/gate"
)

func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestNoArgsPrintsUsage(t *testing.T) {
	code, _, errs := runCLI(t)
	if code != gate.ExitMisuse || !strings.Contains(errs, "ferret") {
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
		if len(fields) < 2 || fields[0] != "ferret" {
			t.Fatalf("%s: %q is not `ferret <version>`", spelling, out)
		}
		if fields[1] != binVersion {
			t.Errorf("%s: field 2 = %q, want %q — release.yml reads this field",
				spelling, fields[1], binVersion)
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
	if code, out, errs := runCLI(t, "install", "--from", fakeCheckout(t)); code != 0 {
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

// A checkout stands in for the repo the default would fetch. Synthetic on purpose: these tests are
// about DEPLOYMENT and must not start failing because someone edited the real SKILL.md.
func fakeCheckout(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for rel, body := range map[string]string{
		"skill/SKILL.md":                       "# skill\n",
		"skill/VERSION":                        `{"version":"2026-08-01.8"}`,
		"skill/commands/slop-ferret-report.md": "# report\n",
		"skill/references/ai-slop-lexicon.md":  "# lexicon\n",
	} {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// install and update are synonyms (D4): they differ only in whether something is already there,
// which the tool can see for itself.
func TestInstallAndUpdateAreSynonyms(t *testing.T) {
	dir := fakeCheckout(t)
	for _, verb := range []string{"install", "update"} {
		t.Setenv("HOME", t.TempDir())
		if code, out, errs := runCLI(t, verb, "--from", dir); code != 0 {
			t.Fatalf("%s: code=%d out=%s err=%s", verb, code, out, errs)
		}
	}
}

// With no compiled-in copy and no tag to resolve, a bare install must say what to do instead of
// silently falling back to HEAD -- that fallback is the unpinned install the default exists to avoid.
func TestBareInstallBeforeAnyReleaseNamesTheAlternatives(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	code, _, errs := runCLI(t, "install")
	if code == 0 {
		t.Fatal("with no tag to resolve, a bare install must not silently succeed")
	}
	for _, want := range []string{"--from", "--ref"} {
		if !strings.Contains(errs, want) {
			t.Errorf("error must name %s: %q", want, errs)
		}
	}
}

package install

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// A minimal stand-in for the real skill tree. Using a synthetic FS rather than the embedded one
// keeps these tests about DEPLOYMENT behaviour: they must not start failing because someone
// edited SKILL.md.
func fakeSkill() fstest.MapFS {
	return fstest.MapFS{
		"skill/SKILL.md":                       {Data: []byte("# skill\n")},
		"skill/VERSION":                        {Data: []byte(`{"version":"2026-08-01.8"}`)},
		"skill/commands/slop-ferret-report.md": {Data: []byte("# report\n")},
		"skill/references/ai-slop-lexicon.md":  {Data: []byte("# lexicon\n")},
		"skill/references/families.md":         {Data: []byte("# families\n")},
	}
}

func setup(t *testing.T) (home string, out *bytes.Buffer) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	SkillFS = fakeSkill()
	return home, &bytes.Buffer{}
}

func dest(home string) string { return filepath.Join(home, ".claude", "skills", "slop-ferret") }
func cmds(home string) string { return filepath.Join(home, ".claude", "commands") }

func TestInstallDeploysSkillAndBothCommandEntries(t *testing.T) {
	home, out := setup(t)
	if code := Install(out, false); code != 0 {
		t.Fatalf("install = %d: %s", code, out)
	}
	for _, rel := range []string{"SKILL.md", "references/ai-slop-lexicon.md"} {
		if _, err := os.Stat(filepath.Join(dest(home), filepath.FromSlash(rel))); err != nil {
			t.Errorf("%s not deployed: %v", rel, err)
		}
	}
	// Installing one entry and not the other is the original defect; both or neither.
	for _, name := range []string{"slop-ferret.md", "slop-ferret/report.md"} {
		p := filepath.Join(cmds(home), filepath.FromSlash(name))
		fi, err := os.Lstat(p)
		if err != nil || fi.Mode()&os.ModeSymlink == 0 {
			t.Errorf("%s must be a symlink: %v", name, err)
		}
	}
}

func TestDoctorIsCleanRightAfterInstall(t *testing.T) {
	_, out := setup(t)
	Install(out, false)
	out.Reset()
	if code := Doctor(out); code != 0 {
		t.Fatalf("doctor = %d, want 0: %s", code, out)
	}
}

// The exact 2026-08-01 state: /slop-ferret:report linked, /slop-ferret missing. The skill could
// not be invoked, so allowed-tools never applied, so Edit and Artifact were withheld in prose
// only — and a pre-registered control ran the whole method holding both.
func TestDoctorCatchesAHalfInstall(t *testing.T) {
	home, out := setup(t)
	Install(out, false)
	if err := os.Remove(filepath.Join(cmds(home), "slop-ferret.md")); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := Doctor(out); code != 1 {
		t.Fatalf("doctor = %d, want 1: %s", code, out)
	}
	if !strings.Contains(out.String(), "command entry missing") {
		t.Errorf("must name the missing entry: %s", out)
	}
	if !strings.Contains(out.String(), "allowed-tools never apply") {
		t.Errorf("must say WHY a missing entry matters, not just that it is missing: %s", out)
	}
}

// The failure guarded against is "I edited the deployed copy by mistake", not an attack.
// Overwriting silently would eat real work.
func TestInstallRefusesToClobberAHandEditedFile(t *testing.T) {
	home, out := setup(t)
	Install(out, false)
	mine := "# my in-progress edits\n"
	if err := os.WriteFile(filepath.Join(dest(home), "SKILL.md"), []byte(mine), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := Install(out, false); code != 3 {
		t.Fatalf("install = %d, want 3: %s", code, out)
	}
	if !strings.Contains(out.String(), "REFUSING") || !strings.Contains(out.String(), "SKILL.md") {
		t.Errorf("must name what would be lost: %s", out)
	}
	got, _ := os.ReadFile(filepath.Join(dest(home), "SKILL.md"))
	if string(got) != mine {
		t.Error("a refused install must leave the hand edit intact")
	}
}

func TestForceOverwritesAfterSayingWhatIsLost(t *testing.T) {
	home, out := setup(t)
	Install(out, false)
	os.WriteFile(filepath.Join(dest(home), "SKILL.md"), []byte("# mine\n"), 0o644)
	out.Reset()
	if code := Install(out, true); code != 0 {
		t.Fatalf("forced install = %d: %s", code, out)
	}
	got, _ := os.ReadFile(filepath.Join(dest(home), "SKILL.md"))
	if string(got) == "# mine\n" {
		t.Error("--force must overwrite")
	}
}

// What the retired digest scheme could never do: say WHICH file, and in which direction.
func TestDoctorNamesTheFileEditedInPlace(t *testing.T) {
	home, out := setup(t)
	Install(out, false)
	os.WriteFile(filepath.Join(dest(home), "references", "families.md"), []byte("# ed\n"), 0o644)
	out.Reset()
	if code := Doctor(out); code != 1 {
		t.Fatalf("doctor = %d, want 1: %s", code, out)
	}
	if !strings.Contains(out.String(), "families.md") ||
		!strings.Contains(out.String(), "edited in place") {
		t.Errorf("must name the file and the direction: %s", out)
	}
}

// Deployed file untouched since install, but the binary has moved on: that is "out of date",
// which is a different message and a different fix from "you edited it".
func TestDoctorDistinguishesOutOfDateFromEditedInPlace(t *testing.T) {
	_, out := setup(t)
	Install(out, false)
	fs := fakeSkill()
	fs["skill/references/families.md"] = &fstest.MapFile{Data: []byte("# newer upstream\n")}
	SkillFS = fs
	out.Reset()
	if code := Doctor(out); code != 1 {
		t.Fatalf("doctor = %d, want 1: %s", code, out)
	}
	if !strings.Contains(out.String(), "out of date") {
		t.Errorf("want 'out of date': %s", out)
	}
	if strings.Contains(out.String(), "edited in place") {
		t.Errorf("must NOT accuse the operator of editing a file they did not touch: %s", out)
	}
}

func TestDoctorReportsNotInstalledRatherThanCrashing(t *testing.T) {
	_, out := setup(t)
	if code := Doctor(out); code != 1 {
		t.Fatalf("doctor = %d, want 1: %s", code, out)
	}
	if !strings.Contains(out.String(), "not installed") {
		t.Errorf("want 'not installed': %s", out)
	}
}

func TestInstallIsIdempotent(t *testing.T) {
	_, out := setup(t)
	Install(out, false)
	out.Reset()
	if code := Install(out, false); code != 0 {
		t.Fatalf("second install = %d: %s", code, out)
	}
	if !strings.Contains(out.String(), "(0 changed)") {
		t.Errorf("a no-op install should report 0 changed: %s", out)
	}
}

func TestVersionComesFromTheEmbeddedStamp(t *testing.T) {
	setup(t)
	if got := Version(); got != "2026-08-01.8" {
		t.Errorf("Version() = %q", got)
	}
}

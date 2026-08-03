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
		// The fence is not decoration: gate.loadSignals reads the H vocabulary from it, and
		// doctor now reports a deployed lexicon without one, because that deployment produces
		// sweeps with no vocabulary that read exactly like clean ones.
		"skill/references/ai-slop-lexicon.md": {Data: []byte("# lexicon\n\n```h-signals\nmoney/value: pay|wallet\n```\n")},
		"skill/references/families.md":        {Data: []byte("# families\n")},
	}
}

func setup(t *testing.T) (home string, src Source, out *bytes.Buffer) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	return home, Source{FS: fakeSkill(), Desc: "test"}, &bytes.Buffer{}
}

func dest(home string) string { return filepath.Join(home, ".claude", "skills", "slop-ferret") }
func cmds(home string) string { return filepath.Join(home, ".claude", "commands") }

func TestInstallDeploysSkillAndBothCommandEntries(t *testing.T) {
	home, src, out := setup(t)
	if code := Install(out, src, false); code != 0 {
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
	_, src, out := setup(t)
	Install(out, src, false)
	out.Reset()
	if code := Doctor(out, src, "test-bin"); code != 0 {
		t.Fatalf("doctor = %d, want 0: %s", code, out)
	}
}

// The exact 2026-08-01 state: /slop-ferret:report linked, /slop-ferret missing. The skill could
// not be invoked, so allowed-tools never applied, so Edit and Artifact were withheld in prose
// only — and a pre-registered control ran the whole method holding both.
func TestDoctorCatchesAHalfInstall(t *testing.T) {
	home, src, out := setup(t)
	Install(out, src, false)
	if err := os.Remove(filepath.Join(cmds(home), "slop-ferret.md")); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := Doctor(out, src, "test-bin"); code != 1 {
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
	home, src, out := setup(t)
	Install(out, src, false)
	mine := "# my in-progress edits\n"
	if err := os.WriteFile(filepath.Join(dest(home), "SKILL.md"), []byte(mine), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := Install(out, src, false); code != 3 {
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
	home, src, out := setup(t)
	Install(out, src, false)
	must(t, os.WriteFile(filepath.Join(dest(home), "SKILL.md"), []byte("# mine\n"), 0o644))
	out.Reset()
	if code := Install(out, src, true); code != 0 {
		t.Fatalf("forced install = %d: %s", code, out)
	}
	got, _ := os.ReadFile(filepath.Join(dest(home), "SKILL.md"))
	if string(got) == "# mine\n" {
		t.Error("--force must overwrite")
	}
}

// What the retired digest scheme could never do: say WHICH file, and in which direction.
func TestDoctorNamesTheFileEditedInPlace(t *testing.T) {
	home, src, out := setup(t)
	Install(out, src, false)
	must(t, os.WriteFile(filepath.Join(dest(home), "references", "families.md"), []byte("# ed\n"), 0o644))
	out.Reset()
	if code := Doctor(out, src, "test-bin"); code != 1 {
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
	_, src, out := setup(t)
	Install(out, src, false)
	newer := fakeSkill()
	newer["skill/references/families.md"] = &fstest.MapFile{Data: []byte("# newer upstream\n")}
	src = Source{FS: newer, Desc: "test-newer"}
	out.Reset()
	if code := Doctor(out, src, "test-bin"); code != 1 {
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
	_, src, out := setup(t)
	if code := Doctor(out, src, "test-bin"); code != 1 {
		t.Fatalf("doctor = %d, want 1: %s", code, out)
	}
	if !strings.Contains(out.String(), "not installed") {
		t.Errorf("want 'not installed': %s", out)
	}
}

func TestInstallIsIdempotent(t *testing.T) {
	_, src, out := setup(t)
	Install(out, src, false)
	out.Reset()
	if code := Install(out, src, false); code != 0 {
		t.Fatalf("second install = %d: %s", code, out)
	}
	if !strings.Contains(out.String(), "(0 changed)") {
		t.Errorf("a no-op install should report 0 changed: %s", out)
	}
}

func TestSkillVersionIsIndependentOfTheBinaryVersion(t *testing.T) {
	_, src, _ := setup(t)
	if got := SkillVersion(src); got != "2026-08-01.8" {
		t.Errorf("SkillVersion() = %q", got)
	}
}

// The whole point of splitting the source out: a skill tree with NEWER prose installs over the
// same binary, and doctor reports the two versions separately rather than one number that cannot
// distinguish "new binary" from "new lexicon".
func TestANewerSkillInstallsWithoutANewBinary(t *testing.T) {
	home, src, out := setup(t)
	Install(out, src, false)
	newer := fakeSkill()
	newer["skill/VERSION"] = &fstest.MapFile{Data: []byte(`{"version":"2026-09-01.1"}`)}
	newer["skill/references/ai-slop-lexicon.md"] = &fstest.MapFile{
		Data: []byte("# new class\n\n```h-signals\nmoney/value: pay|wallet\n```\n")}
	next := Source{FS: newer, Desc: "repo@main (deadbeef)"}
	out.Reset()
	if c := Install(out, next, false); c != 0 {
		t.Fatalf("install of a newer skill = %d: %s", c, out)
	}
	if !strings.Contains(out.String(), "2026-09-01.1") ||
		!strings.Contains(out.String(), "repo@main") {
		t.Errorf("install must report the skill version AND its provenance: %s", out)
	}
	got, _ := os.ReadFile(filepath.Join(dest(home), "references", "ai-slop-lexicon.md"))
	if !strings.Contains(string(got), "# new class") {
		t.Errorf("the newer lexicon must reach the deployed copy: %q", got)
	}
	out.Reset()
	Doctor(out, next, "test-bin")
	if !strings.Contains(out.String(), "binary test-bin") ||
		!strings.Contains(out.String(), "skill 2026-09-01.1") {
		t.Errorf("doctor must print both versions separately: %s", out)
	}
}

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// doctor must describe what is ON DISK even with no source reachable. It once panicked on a nil FS
// -- "I cannot reach a source" and "the deployment is broken" are different findings and the tool
// has to be able to report the second without the first.
func TestDoctorWorksWithNoSourceReachable(t *testing.T) {
	home, src, out := setup(t)
	Install(out, src, false)
	out.Reset()
	code := Doctor(out, Source{}, "test-bin")
	if code != 0 {
		t.Fatalf("doctor with no source = %d, want 0: %s", code, out)
	}
	if !strings.Contains(out.String(), "no source reachable") {
		t.Errorf("must say the source was unreachable rather than imply drift: %s", out)
	}
	_ = home
}

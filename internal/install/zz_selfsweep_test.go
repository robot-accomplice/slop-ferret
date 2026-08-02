package install

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FOUND BY THE SELF-SWEEP. Install refuses to clobber a hand-edited file in the skill tree and
// printed what would be lost; the command entries live OUTSIDE that tree and had no protection at
// all, so a user's own ~/.claude/commands/slop-ferret.md was deleted without warning. A guard
// applied where the author was looking rather than to the class is the lexicon's `Sited guard`.
func TestInstallRefusesToDestroyAHandWrittenCommandEntry(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	mine := filepath.Join(home, ".claude", "commands", "slop-ferret.md")
	if err := os.MkdirAll(filepath.Dir(mine), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: slop-ferret\n---\n# my own hand-written command\n"
	if err := os.WriteFile(mine, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if code := Install(&out, Source{FS: fakeSkill(), Desc: "test"}, false); code == 0 {
		t.Fatalf("install must not silently replace a hand-written command entry: %s", out.String())
	}
	got, err := os.ReadFile(mine)
	if err != nil || string(got) != content {
		t.Fatal("the hand-written command entry must survive a refused install")
	}

	// --force still overwrites, after the refusal has said what would be lost.
	out.Reset()
	if code := Install(&out, Source{FS: fakeSkill(), Desc: "test"}, true); code != 0 {
		t.Fatalf("--force should proceed: %s", out.String())
	}
	if fi, err := os.Lstat(mine); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Error("--force should have replaced it with our symlink")
	}
}

// ABORT C1. Content was written before links and the manifest after both, so a refusal at the link
// step left the tree deployed and unmanifested — and the NEXT install then accused the user of
// editing files ferret had written itself. Nothing may reach disk if the install is going to refuse.
func TestARefusedInstallLeavesNothingOnDisk(t *testing.T) {
	home, src, out := setup(t)
	mine := filepath.Join(home, ".claude", "commands", "slop-ferret.md")
	must(t, os.MkdirAll(filepath.Dir(mine), 0o755))
	must(t, os.WriteFile(mine, []byte("# mine\n"), 0o644))

	if code := Install(out, src, false); code == 0 {
		t.Fatalf("expected a refusal: %s", out.String())
	}
	if !strings.Contains(out.String(), "Nothing has been written") {
		t.Errorf("the refusal must say the tree was left alone: %s", out.String())
	}
	if _, err := os.Stat(filepath.Join(dest(home), "SKILL.md")); err == nil {
		t.Error("a refused install deployed the skill tree anyway")
	}

	// And doctor must not now blame the user for a state ferret created.
	out.Reset()
	Doctor(out, src, "test-bin")
	if strings.Contains(out.String(), "edited in place") {
		t.Errorf("doctor accuses the user after a refused install: %s", out.String())
	}
}

// THE SHAPE THE PREVIOUS GUARD MISSED, and the reason it missed it.
//
// TestARefusedInstallLeavesNothingOnDisk above asserts the CLASS ("nothing may reach disk if the
// install is going to refuse") but exercises exactly one instance of it: a regular file AT the link
// path, which `os.Lstat` reports cleanly. Here the regular file is at the link's PARENT, so Lstat
// returns ENOTDIR, the old pre-flight's `err == nil` was false, no refusal fired, and MkdirAll then
// failed after the whole tree was on disk — 20/20 under review, 14 of them leaving one command
// linked and the other missing.
//
// A guard whose test pins the one shape the author had in mind is this repo's own `Sited guard`
// class. Table-driven so a third shape is one line, not a new copy of the test.
func TestNoInstallShapeCanWriteASubset(t *testing.T) {
	shapes := []struct {
		name    string
		prepare func(t *testing.T, home string)
	}{
		{"regular file AT the link path", func(t *testing.T, home string) {
			p := filepath.Join(home, ".claude", "commands", "slop-ferret.md")
			must(t, os.MkdirAll(filepath.Dir(p), 0o755))
			must(t, os.WriteFile(p, []byte("# mine\n"), 0o644))
		}},
		{"regular file where a link's PARENT DIR must go", func(t *testing.T, home string) {
			p := filepath.Join(home, ".claude", "commands", "slop-ferret")
			must(t, os.MkdirAll(filepath.Dir(p), 0o755))
			must(t, os.WriteFile(p, []byte("not a directory\n"), 0o644))
		}},
		{"commands/ is read-only", func(t *testing.T, home string) {
			p := filepath.Join(home, ".claude", "commands")
			must(t, os.MkdirAll(p, 0o755))
			must(t, os.Chmod(p, 0o500))
			t.Cleanup(func() { _ = os.Chmod(p, 0o755) })
		}},
	}
	for _, s := range shapes {
		t.Run(s.name, func(t *testing.T) {
			home, src, out := setup(t)
			s.prepare(t, home)

			code := Install(out, src, false)
			if code == 0 {
				t.Fatalf("expected a refusal: %s", out.String())
			}
			// The invariant is not "it failed" — a half-install also fails. It is that the
			// deployment is untouched, so the next install is not lied to.
			if _, err := os.Stat(filepath.Join(dest(home), "SKILL.md")); err == nil {
				t.Error("the skill tree was deployed by an install that refused")
			}
			if _, err := os.Stat(filepath.Join(dest(home), manifestName)); err == nil {
				t.Error("a manifest was written by an install that refused")
			}
			// And never one command entry without the other.
			linked := 0
			for name := range commands {
				l := filepath.Join(home, ".claude", "commands", filepath.FromSlash(name))
				if fi, err := os.Lstat(l); err == nil && fi.Mode()&os.ModeSymlink != 0 {
					linked++
				}
			}
			if linked != 0 && linked != len(commands) {
				t.Errorf("%d of %d command entries linked — a subset is the original defect: "+
					"/slop-ferret:report resolving while /slop-ferret does not means the skill's "+
					"allowed-tools never apply", linked, len(commands))
			}
		})
	}
}

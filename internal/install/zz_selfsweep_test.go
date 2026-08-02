package install

import (
	"bytes"
	"os"
	"path/filepath"
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

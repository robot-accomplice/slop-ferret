package install

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// tarball builds the shape codeload actually serves: every entry under a `<repo>-<sha>/` prefix.
func tarball(t *testing.T, sha string, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		h := &tar.Header{Name: archivePrefix + sha + "/" + name, Mode: 0o644,
			Size: int64(len(body)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	// These are WRITERS: Close flushes, and an unflushed archive is a corrupt fixture that would
	// fail the test it feeds for the wrong reason. Not an idiomatic cleanup discard.
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func serve(t *testing.T, sha string, files map[string]string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/commits/"):
			ref := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			if ref == "no-such-ref" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write([]byte(sha))
		default:
			_, _ = w.Write(tarball(t, sha, files))
		}
	}))
	t.Cleanup(srv.Close)
	oldT, oldR := tarballURL, refAPIURL
	tarballURL = srv.URL + "/tar.gz/"
	refAPIURL = srv.URL + "/commits/"
	t.Cleanup(func() { tarballURL, refAPIURL = oldT, oldR })
}

func TestFetchExtractsOnlyTheSkillTree(t *testing.T) {
	serve(t, "deadbeefcafe1234", map[string]string{
		"skill/SKILL.md":                      "# skill\n",
		"skill/VERSION":                       `{"version":"2026-09-09.1"}`,
		"skill/references/ai-slop-lexicon.md": "# lexicon\n",
		"main.go":                             "package main\n", // the code half must NOT come along
		"internal/gate/gate.go":               "package gate\n",
	})
	src, cleanup, err := Fetch("main")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if _, err := fs.ReadFile(src.FS, "skill/SKILL.md"); err != nil {
		t.Errorf("skill/SKILL.md not extracted: %v", err)
	}
	if _, err := fs.Stat(src.FS, "main.go"); err == nil {
		t.Error("the code half must not be extracted; the binary is its own problem")
	}
	if got := SkillVersion(src); got != "2026-09-09.1" {
		t.Errorf("SkillVersion = %q", got)
	}
}

// The provenance must name the COMMIT, not echo the ref back. The first version read the sha out
// of the archive's top-level directory, which for a branch is named after the branch — so
// `repo@main (main)` was a restatement of the input dressed as a resolution of it, and the comment
// claiming the ref was pinned was a Fabricated claim by this tool's own lexicon.
func TestFetchProvenanceNamesTheResolvedCommitNotTheRef(t *testing.T) {
	serve(t, "0f7ac19abcdef000", map[string]string{"skill/SKILL.md": "# s\n"})
	src, cleanup, err := Fetch("main")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if !strings.Contains(src.Desc, "0f7ac19abcde") {
		t.Errorf("Desc = %q, must carry the resolved commit", src.Desc)
	}
	if strings.Contains(src.Desc, "(main)") {
		t.Errorf("Desc = %q echoes the ref instead of resolving it", src.Desc)
	}
}

func TestFetchFailsLoudOnAnUnknownRef(t *testing.T) {
	serve(t, "abc", map[string]string{"skill/SKILL.md": "# s\n"})
	if _, _, err := Fetch("no-such-ref"); err == nil ||
		!strings.Contains(err.Error(), "is the ref right?") {
		t.Fatalf("want a ref-resolution failure, got %v", err)
	}
}

// An archive with no skill/ tree must fail rather than install nothing and report success.
func TestFetchRefusesAnArchiveWithNoSkillTree(t *testing.T) {
	serve(t, "abc0000000000000", map[string]string{"main.go": "package main\n"})
	if _, _, err := Fetch("main"); err == nil || !strings.Contains(err.Error(), "no skill/") {
		t.Fatalf("want a refusal, got %v", err)
	}
}

func TestFetchDefaultsToMain(t *testing.T) {
	serve(t, "1111111111111111", map[string]string{"skill/SKILL.md": "# s\n"})
	src, cleanup, err := Fetch("")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if !strings.Contains(src.Desc, "repo@main") {
		t.Errorf("Desc = %q, want the default ref named", src.Desc)
	}
}

func TestDirSourceAcceptsACheckoutRoot(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"skill/SKILL.md": "# s\n"})
	src, err := DirSource(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.ReadFile(src.FS, "skill/SKILL.md"); err != nil {
		t.Errorf("DirSource did not root at the checkout: %v", err)
	}
}

func TestDirSourceRejectsADirWithNoSkillTree(t *testing.T) {
	if _, err := DirSource(t.TempDir()); err == nil {
		t.Fatal("want an error naming the missing skill/ dir")
	}
}

func TestSkillVersionIsUnknownRatherThanEmptyWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"skill/SKILL.md": "# s\n"})
	src, _ := DirSource(dir)
	if got := SkillVersion(src); got != "unknown" {
		t.Errorf("SkillVersion = %q, want %q — a missing stamp must announce itself", got, "unknown")
	}
}

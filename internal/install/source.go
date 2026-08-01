package install

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// A Source is where a skill tree is being installed FROM.
//
// WHY THIS IS PLUGGABLE. The skill was embedded in the binary, which made every lexicon wording
// change a binary release: a new class definition could not reach a running sweep without a
// rebuild, a `go install`, and a reinstall. That is a 1:1 coupling between two things with
// genuinely different cadences — the prose half of this method changes far more often than the
// code half, and usually for reasons the code does not care about.
//
// So there are three sources and one install path:
//
//	embedded   the copy compiled in. The bootstrap floor: always present, works offline, and is
//	           what a fresh `go install` has before it has talked to anything.
//	repo       the live `skill/` tree from the public repository at a ref. `slop-ferret update`.
//	dir        a local checkout, for the edit-build-install loop. `slop-ferret install --from`.
//
// The skill carries its OWN version (skill/VERSION), which is now independent of the binary's.
// `doctor` prints both and says which source the deployed copy came from, because "which skill am
// I running" and "which binary am I running" stopped being the same question.
type Source struct {
	FS   fs.FS  // rooted so that "skill/..." resolves
	Desc string // human-readable provenance, recorded in the manifest
}

const (
	// The public repo. No token, no API, no update server: one HTTPS GET of a tarball the
	// forge already builds. For one user, anything more is infrastructure for its own sake.
	tarballURL = "https://codeload.github.com/robot-accomplice/slop-ferret/tar.gz/refs/heads/"
	fetchLimit = 32 << 20 // a skill tree is prose; anything near this is wrong
	// The archive's top-level dir is `<repo>-<sha>`; that is how a ref gets pinned to the
	// commit it actually resolved to at fetch time.
	archivePrefix = "slop-ferret-"
)

// Fetch downloads the skill tree from the repository at ref and returns it as a Source.
//
// It writes to a temp dir and returns an os.DirFS over it rather than streaming straight into
// place: a half-applied update is worse than a stale one, and an interrupted download must not be
// able to leave the deployed skill in a state that is neither the old nor the new version.
func Fetch(ref string) (Source, func(), error) {
	if ref == "" {
		ref = "main"
	}
	noop := func() {}
	req, err := http.NewRequest("GET", tarballURL+ref, nil)
	if err != nil {
		return Source{}, noop, err
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return Source{}, noop, fmt.Errorf("fetching skill at %q: %w", ref, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Source{}, noop, fmt.Errorf("fetching skill at %q: HTTP %d (is the ref right?)",
			ref, resp.StatusCode)
	}

	tmp, err := os.MkdirTemp("", "slop-ferret-skill-")
	if err != nil {
		return Source{}, noop, err
	}
	cleanup := func() { os.RemoveAll(tmp) }

	gz, err := gzip.NewReader(io.LimitReader(resp.Body, fetchLimit))
	if err != nil {
		cleanup()
		return Source{}, noop, fmt.Errorf("skill archive is not gzip: %w", err)
	}
	defer gz.Close()

	// The archive's top-level directory is `slop-<resolved-sha>`, which is how a ref name gets
	// pinned to the commit it actually resolved to at fetch time. Recording the ref alone would
	// make "installed from main" unreproducible the moment main moves.
	resolved, n := "", 0
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			cleanup()
			return Source{}, noop, fmt.Errorf("reading skill archive: %w", err)
		}
		name := path.Clean(hdr.Name)
		parts := strings.SplitN(name, "/", 2)
		if resolved == "" && strings.HasPrefix(parts[0], archivePrefix) {
			resolved = strings.TrimPrefix(parts[0], archivePrefix)
		}
		if len(parts) < 2 || !strings.HasPrefix(parts[1], "skill/") {
			continue // only the skill tree; the code half is this binary's problem, not the skill's
		}
		rel := parts[1]
		// A tar entry naming a path outside the extraction root is the classic archive defect.
		// path.Clean above plus this check is the whole guard, and it is cheap.
		if strings.Contains(rel, "..") {
			cleanup()
			return Source{}, noop, fmt.Errorf("skill archive contains a traversing path: %q", hdr.Name)
		}
		dst := filepath.Join(tmp, filepath.FromSlash(rel))
		if hdr.Typeflag == tar.TypeDir {
			os.MkdirAll(dst, 0o755)
			continue
		}
		if hdr.Typeflag != tar.TypeReg {
			continue // no symlinks or devices in a prose tree
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			cleanup()
			return Source{}, noop, err
		}
		f, err := os.Create(dst)
		if err != nil {
			cleanup()
			return Source{}, noop, err
		}
		if _, err := io.Copy(f, io.LimitReader(tr, fetchLimit)); err != nil {
			f.Close()
			cleanup()
			return Source{}, noop, err
		}
		f.Close()
		n++
	}
	if n == 0 {
		cleanup()
		return Source{}, noop, fmt.Errorf("no skill/ tree in the archive at %q — nothing to install", ref)
	}
	if len(resolved) > 12 {
		resolved = resolved[:12]
	}
	return Source{FS: os.DirFS(tmp), Desc: fmt.Sprintf("repo@%s (%s)", ref, resolved)}, cleanup, nil
}

// DirSource installs from a local checkout: the edit-build-install loop, and the escape hatch when
// the network is not available or the ref is not pushed yet.
func DirSource(dir string) (Source, error) {
	if fi, err := os.Stat(filepath.Join(dir, embedRoot)); err != nil || !fi.IsDir() {
		// Tolerate being handed the skill dir itself rather than its parent.
		if fi2, err2 := os.Stat(filepath.Join(dir, "SKILL.md")); err2 == nil && !fi2.IsDir() {
			parent, leaf := filepath.Split(strings.TrimRight(dir, string(filepath.Separator)))
			if leaf == embedRoot {
				return Source{FS: os.DirFS(parent), Desc: "dir:" + dir}, nil
			}
		}
		return Source{}, fmt.Errorf("no %s/ directory under %s", embedRoot, dir)
	}
	return Source{FS: os.DirFS(dir), Desc: "dir:" + dir}, nil
}

// SkillVersion reads the version a Source declares. Distinct from the binary's own version, and
// that distinction is the point of this file.
func SkillVersion(src Source) string {
	b, err := fs.ReadFile(src.FS, embedRoot+"/VERSION")
	if err != nil {
		return "unknown"
	}
	var v struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(b, &v) != nil || v.Version == "" {
		return "unknown"
	}
	return v.Version
}

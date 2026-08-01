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

// The public repo. No token and no update server: the forge already builds the tarball.
// Two requests, not one — see resolveRef. Vars, not consts, so the fetch path is testable
// against a local server: an updater whose only exercise is against the live internet is one
// that gets tested by its users.
var (
	tarballURL = "https://codeload.github.com/robot-accomplice/slop-ferret/tar.gz/"
	refAPIURL  = "https://api.github.com/repos/robot-accomplice/slop-ferret/commits/"
)

const (
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
	client := &http.Client{Timeout: 60 * time.Second}

	// Resolve the ref to a commit BEFORE downloading, and download that commit.
	//
	// The first version fetched `/tar.gz/refs/heads/<ref>` and read the sha out of the archive's
	// top-level directory name. For a branch that directory is `<repo>-<branch>`, not
	// `<repo>-<sha>`, so the recorded provenance was `repo@main (main)` — a restatement of the
	// input dressed as a resolution of it. The comment asserting it pinned the commit was, by
	// this tool's own lexicon, a `Fabricated claim`: prose describing behaviour the code lacked,
	// shipped inside the updater. Resolving first makes the claim true and the download
	// reproducible, which is the point of recording a sha at all.
	sha, err := resolveRef(client, ref)
	if err != nil {
		return Source{}, noop, err
	}
	resp, err := client.Get(tarballURL + sha)
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

	n := 0
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
			if err := os.MkdirAll(dst, 0o755); err != nil {
				cleanup()
				return Source{}, noop, err
			}
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
	short := sha
	if len(short) > 12 {
		short = short[:12]
	}
	return Source{FS: os.DirFS(tmp), Desc: fmt.Sprintf("repo@%s %s", ref, short)}, cleanup, nil
}

// resolveRef turns a branch, tag or sha into the commit sha it names right now. Recording the ref
// alone would make "installed from main" unreproducible the moment main moves.
func resolveRef(client *http.Client, ref string) (string, error) {
	req, err := http.NewRequest("GET", refAPIURL+ref, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github.sha")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("resolving ref %q: %w", ref, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("resolving ref %q: HTTP %d (is the ref right?)", ref, resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 128))
	if err != nil {
		return "", err
	}
	sha := strings.TrimSpace(string(b))
	if len(sha) < 7 {
		return "", fmt.Errorf("resolving ref %q: unexpected response %q", ref, sha)
	}
	return sha, nil
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

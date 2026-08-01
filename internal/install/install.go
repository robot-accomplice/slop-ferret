// Package install deploys the slop-ferret skill from this binary into ~/.claude, and reports
// drift between the two.
//
// WHY THIS EXISTS. Before it, the INSTALLED copy of the skill was the only copy. There was no
// repo, so a script stamped a digest over the deployed tree and stood in for version control.
// That could answer "something changed" and never "what changed" — and it could not notice the
// failure it most needed to: an install that was never finished.
//
// It was not finished. ~/.claude/commands/slop-ferret/report.md existed, so /slop-ferret:report
// resolved; nothing linked /slop-ferret itself, so the parent skill could not be invoked, so its
// allowed-tools never applied, so the withholding of Edit and Artifact — which SKILL.md names as
// the runtime enforcement of additive-only and never-publish — was prose with nothing behind it.
// A pre-registered control ran the entire method that way, holding both tools it was meant to be
// denied. A distribution defect presenting as a safety one, and hand-installation produced it:
// someone linked one of the two entries, once, and nothing ever checked for the other.
//
// The skill is EMBEDDED in the binary rather than read from a checkout, which is what makes the
// upgrade path a single line:
//
//	go install github.com/robot-accomplice/slop@latest && slop install
//
// Direction of truth: edit in the repo, build, install. `slop doctor` catches the case where the
// hand edited the deployed copy instead — which is the common mistake, not a hypothetical.
package install

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SkillFS is the embedded skill tree, injected by main. It lives at the repo root rather than
// beside this package so the thing a human edits is the obvious top-level `skill/` directory;
// go:embed can only reach inside its own package dir, so main owns the directive and hands the
// filesystem down.
var SkillFS fs.FS

const (
	embedRoot    = "skill"
	manifestName = ".slop-install.json"
)

// Both entries, always, together. Installing one and not the other IS the original defect, so
// they are one table and there is no code path that writes a subset.
var commands = map[string]string{
	"slop-ferret.md":        "SKILL.md",
	"slop-ferret/report.md": "commands/slop-ferret-report.md",
}

type manifest struct {
	Version string            `json:"version"`
	Files   map[string]string `json:"files"`
}

type paths struct{ home, dest, cmds string }

func newPaths() (paths, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return paths{}, err
	}
	return paths{
		home: h,
		dest: filepath.Join(h, ".claude", "skills", "slop-ferret"),
		cmds: filepath.Join(h, ".claude", "commands"),
	}, nil
}

func hashBytes(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])[:16]
}

// srcFiles lists the embedded skill, relative to embedRoot.
func srcFiles() ([]string, error) {
	var out []string
	err := fs.WalkDir(SkillFS, embedRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || strings.HasSuffix(p, manifestName) {
			return nil
		}
		rel, _ := filepath.Rel(embedRoot, p)
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(out)
	return out, err
}

func srcBytes(rel string) ([]byte, error) {
	return fs.ReadFile(SkillFS, embedRoot+"/"+rel)
}

// Version reads the embedded VERSION stamp.
func Version() string {
	b, err := srcBytes("VERSION")
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

func readManifest(p paths) manifest {
	var m manifest
	b, err := os.ReadFile(filepath.Join(p.dest, manifestName))
	if err != nil {
		return m
	}
	_ = json.Unmarshal(b, &m)
	return m
}

// state of one deployed file relative to the embedded copy.
type state int

const (
	stMissing state = iota // not deployed
	stSame                 // deployed == embedded
	stStale                // deployed differs, but matches what we last wrote: the binary moved on
	stLocal                // deployed differs AND was not written by us: edited in place
)

// classify compares every embedded file against its deployed counterpart.
//
// stLocal is the one that blocks. It means somebody edited the deployed copy, which is
// recoverable information — so it is reported rather than resolved. This tool does not merge,
// and guessing which side was intended is exactly the judgement a program should not make alone.
func classify(p paths) (map[string]state, error) {
	files, err := srcFiles()
	if err != nil {
		return nil, err
	}
	man := readManifest(p).Files
	out := make(map[string]state, len(files))
	for _, rel := range files {
		want, err := srcBytes(rel)
		if err != nil {
			return nil, err
		}
		got, err := os.ReadFile(filepath.Join(p.dest, filepath.FromSlash(rel)))
		switch {
		case err != nil:
			out[rel] = stMissing
		case hashBytes(got) == hashBytes(want):
			out[rel] = stSame
		case man[rel] != "" && man[rel] == hashBytes(got):
			out[rel] = stStale
		default:
			out[rel] = stLocal
		}
	}
	return out, nil
}

func linkTargets(p paths) map[string]string {
	out := make(map[string]string, len(commands))
	for name, target := range commands {
		out[filepath.Join(p.cmds, filepath.FromSlash(name))] =
			filepath.Join(p.dest, filepath.FromSlash(target))
	}
	return out
}

func relink(link, target string) error {
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return err
	}
	if _, err := os.Lstat(link); err == nil {
		if err := os.Remove(link); err != nil {
			return err
		}
	}
	return os.Symlink(target, link)
}

func slashCommand(name string) string {
	return "/" + strings.TrimSuffix(strings.ReplaceAll(name, "/", ":"), ".md")
}

// Install writes the embedded skill and BOTH command entries.
func Install(w io.Writer, force bool) int {
	p, err := newPaths()
	if err != nil {
		fmt.Fprintf(w, "slop: %v\n", err)
		return 2
	}
	st, err := classify(p)
	if err != nil {
		fmt.Fprintf(w, "slop: %v\n", err)
		return 2
	}

	var local []string
	for rel, s := range st {
		if s == stLocal {
			local = append(local, rel)
		}
	}
	sort.Strings(local)

	if len(local) > 0 && !force {
		fmt.Fprintf(w, "slop install: REFUSING — these deployed files differ from the embedded "+
			"copy and were not written by this installer, so they were edited in place:\n\n")
		for _, rel := range local {
			fmt.Fprintf(w, "  %s\n", rel)
		}
		fmt.Fprintf(w, "\n  Your edits are in the DEPLOYED copy, not the repo. Move them into the "+
			"repo, or re-run with --force to overwrite them. Diff one with:\n")
		fmt.Fprintf(w, "    diff %s <repo>/skill/%s\n",
			filepath.Join(p.dest, filepath.FromSlash(local[0])), local[0])
		return 3
	}

	files, _ := srcFiles()
	written := make(map[string]string, len(files))
	changed := 0
	for _, rel := range files {
		b, err := srcBytes(rel)
		if err != nil {
			fmt.Fprintf(w, "slop: %v\n", err)
			return 2
		}
		dst := filepath.Join(p.dest, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			fmt.Fprintf(w, "slop: %v\n", err)
			return 2
		}
		if err := os.WriteFile(dst, b, 0o644); err != nil {
			fmt.Fprintf(w, "slop: %v\n", err)
			return 2
		}
		written[rel] = hashBytes(b)
		if st[rel] != stSame {
			changed++
		}
	}

	for link, target := range linkTargets(p) {
		if err := relink(link, target); err != nil {
			fmt.Fprintf(w, "slop: linking %s: %v\n", link, err)
			return 2
		}
	}

	mb, _ := json.MarshalIndent(manifest{Version: Version(), Files: written}, "", " ")
	if err := os.WriteFile(filepath.Join(p.dest, manifestName), mb, 0o644); err != nil {
		fmt.Fprintf(w, "slop: %v\n", err)
		return 2
	}

	fmt.Fprintf(w, "slop install: %s — %d files deployed (%d changed), %d command entries linked\n",
		Version(), len(written), changed, len(commands))
	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, slashCommand(name))
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(w, "  %s\n", n)
	}
	return 0
}

// Doctor reports drift in both directions, and whether the install is actually complete.
func Doctor(w io.Writer) int {
	p, err := newPaths()
	if err != nil {
		fmt.Fprintf(w, "slop: %v\n", err)
		return 2
	}
	var problems []string

	if _, err := os.Stat(p.dest); err != nil {
		problems = append(problems, fmt.Sprintf("not installed: %s does not exist", p.dest))
	} else {
		if readManifest(p).Files == nil {
			problems = append(problems,
				"no install manifest — the deployed skill was not installed by this tool")
		}
		st, err := classify(p)
		if err != nil {
			fmt.Fprintf(w, "slop: %v\n", err)
			return 2
		}
		keys := make([]string, 0, len(st))
		for rel := range st {
			keys = append(keys, rel)
		}
		sort.Strings(keys)
		for _, rel := range keys {
			switch st[rel] {
			case stLocal:
				problems = append(problems, fmt.Sprintf(
					"deployed copy edited in place: %s (your change is NOT in the repo)", rel))
			case stStale:
				problems = append(problems, fmt.Sprintf(
					"out of date: %s (this binary is newer — run `slop install`)", rel))
			case stMissing:
				problems = append(problems, fmt.Sprintf("missing from the deployment: %s", rel))
			}
		}
	}

	for link, target := range linkTargets(p) {
		fi, err := os.Lstat(link)
		if err != nil || fi.Mode()&os.ModeSymlink == 0 {
			problems = append(problems, fmt.Sprintf(
				"command entry missing: %s -> %s (%s will not resolve, so the skill's "+
					"allowed-tools never apply and Edit/Artifact are withheld in prose only)",
				link, target, slashCommand(filepath.ToSlash(strings.TrimPrefix(link, p.cmds+string(filepath.Separator))))))
			continue
		}
		got, _ := os.Readlink(link)
		if got != target {
			problems = append(problems, fmt.Sprintf(
				"command entry points elsewhere: %s -> %s", link, got))
		}
	}

	fmt.Fprintf(w, "slop doctor: embedded skill %s\n", Version())
	if len(problems) == 0 {
		fmt.Fprintf(w, "  ok — deployed copy matches the binary, both commands resolve\n")
		return 0
	}
	for _, s := range problems {
		fmt.Fprintf(w, "  ! %s\n", s)
	}
	return 1
}

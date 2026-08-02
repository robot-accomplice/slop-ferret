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
// NO PROSE IS COMPILED INTO THE BINARY. `install` acquires the skill — from the repository at the
// tag matching this binary's version, from an explicit `--ref`, or from a `--from` checkout. That
// makes the two release cadences structural rather than conventional: a binary that cannot carry
// prose cannot re-couple them by accident.
//
// Direction of truth: edit in the repo, install. `ferret doctor` catches the case where the hand
// edited the deployed copy instead — which is the common mistake, not a hypothetical.
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

const (
	embedRoot    = "skill"
	manifestName = ".slop-install.json"
)

// Both entries, always, together. Installing one and not the other IS the original defect, and
// `linkAll` enforces it: every entry is created before any content is written, in sorted order,
// and the failure of any one undoes the rest. See the comment at its call site for the two shapes
// that defeated the previous instance-shaped guard.
var commands = map[string]string{
	"slop-ferret.md":        "SKILL.md",
	"slop-ferret/report.md": "commands/slop-ferret-report.md",
}

type manifest struct {
	Version string            `json:"version"` // the SKILL's version, not the binary's
	Source  string            `json:"source"`  // where it came from: embedded / repo@ref / dir:
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

// srcFiles lists a source's skill tree, relative to embedRoot. A zero Source has no tree, which is
// a legitimate state: `doctor` runs with no source when nothing can be reached, and still has to
// report what is deployed.
func srcFiles(src Source) ([]string, error) {
	if src.FS == nil {
		return nil, nil
	}
	var out []string
	err := fs.WalkDir(src.FS, embedRoot, func(p string, d fs.DirEntry, err error) error {
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

func srcBytes(src Source, rel string) ([]byte, error) {
	return fs.ReadFile(src.FS, embedRoot+"/"+rel)
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

// state of one deployed file relative to the source being installed from.
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
// classify compares a source against the deployed copy. With no source it returns nothing to
// compare -- not an error: "I could not reach a source" and "the deployment is broken" are
// different findings and must not be conflated.
func classify(p paths, src Source) (map[string]state, error) {
	files, err := srcFiles(src)
	if err != nil {
		return nil, err
	}
	man := readManifest(p).Files
	out := make(map[string]state, len(files))
	for _, rel := range files {
		want, err := srcBytes(src, rel)
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

// relink points a command entry at the deployed skill.
//
// It replaces a SYMLINK freely — that is ours, and repointing it is the whole job. It refuses to
// replace a REGULAR FILE unless forced: that is somebody's own hand-written command, and deleting
// it is data loss with no warning.
//
// Found by sweeping this repo with its own method. `Install` went to real lengths to refuse
// clobbering a hand-edited file in the skill tree and gave the command entries — which live outside
// that tree — no protection at all. A guard applied where the author was looking rather than to the
// class is the lexicon's `Sited guard`, and this was one.
func relink(link, target string, force bool) error {
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return err
	}
	if fi, err := os.Lstat(link); err == nil {
		if fi.Mode()&os.ModeSymlink == 0 && !force {
			return fmt.Errorf("%s exists and is not a symlink this installer created — refusing "+
				"to delete it. Move it aside, or re-run with --force to overwrite it", link)
		}
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
func Install(w io.Writer, src Source, force bool) int {
	p, err := newPaths()
	if err != nil {
		fmt.Fprintf(w, "ferret: %v\n", err)
		return 2
	}
	st, err := classify(p, src)
	if err != nil {
		fmt.Fprintf(w, "ferret: %v\n", err)
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
		fmt.Fprintf(w, "ferret install: REFUSING — these deployed files differ from the source "+
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

	// PRE-FLIGHT EVERY PREDICTABLE FAILURE BEFORE THE FIRST WRITE.
	//
	// Content was written first, links second, manifest third. A user who owned
	// ~/.claude/commands/slop-ferret.md got an abort with the tree already deployed and no
	// manifest — so the NEXT install saw every file as edited-in-place and told them their own
	// edits were at risk, about files ferret had written itself 200ms earlier. The refusal was
	// correct and its timing made the tool lie.
	//
	// THE LINKS GO FIRST, ALL OF THEM, AND THEY ROLL BACK.
	//
	// An earlier fix pre-flighted only `os.Lstat(link)` returning a non-symlink AT the link path.
	// That is one INSTANCE of the failure, not the class: any other relink failure still landed
	// after the tree was on disk. Reproduced 20/20 under review — with
	// ~/.claude/commands/slop-ferret present as a regular FILE, Lstat returns ENOTDIR so the
	// pre-flight passed, MkdirAll then failed mid-loop, and 14 runs left /slop-ferret linked with
	// /slop-ferret:report missing while 6 left neither; every run deployed the full tree with no
	// manifest, so the next install accused the user of editing files ferret had written itself.
	//
	// A symlink does not require its target to exist, so the whole link phase can be completed and
	// verified before any content is written. If any link fails, the ones already made are undone
	// and nothing else has been touched. That covers the class instead of the one shape a test
	// happened to pin.
	if code, err := linkAll(p, force); err != nil {
		fmt.Fprintf(w, "ferret install: REFUSING — %v\n  Nothing has been written: the command "+
			"links are created before the skill tree, and the ones already made were undone.\n", err)
		return code
	}

	files, _ := srcFiles(src)
	written := make(map[string]string, len(files))
	changed := 0
	for _, rel := range files {
		b, err := srcBytes(src, rel)
		if err != nil {
			fmt.Fprintf(w, "ferret: %v\n", err)
			return 2
		}
		dst := filepath.Join(p.dest, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			fmt.Fprintf(w, "ferret: %v\n", err)
			return 2
		}
		if err := os.WriteFile(dst, b, 0o644); err != nil {
			fmt.Fprintf(w, "ferret: %v\n", err)
			return 2
		}
		written[rel] = hashBytes(b)
		if st[rel] != stSame {
			changed++
		}
	}

	mb, _ := json.MarshalIndent(manifest{Version: SkillVersion(src), Source: src.Desc,
		Files: written}, "", " ")
	if err := os.WriteFile(filepath.Join(p.dest, manifestName), mb, 0o644); err != nil {
		fmt.Fprintf(w, "ferret: %v\n", err)
		return 2
	}

	fmt.Fprintf(w, "ferret install: skill %s from %s — %d files deployed (%d changed), "+
		"%d command entries linked\n", SkillVersion(src), src.Desc, len(written), changed,
		len(commands))
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
func Doctor(w io.Writer, src Source, binVersion string) int {
	p, err := newPaths()
	if err != nil {
		fmt.Fprintf(w, "ferret: %v\n", err)
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
		st, err := classify(p, src)
		if err != nil {
			fmt.Fprintf(w, "ferret: %v\n", err)
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
					"out of date: %s (the source is newer — run `ferret update`)", rel))
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

	man := readManifest(p)
	installed := man.Version
	if installed == "" {
		installed = "unknown"
	}
	from := man.Source
	if from == "" {
		from = "unrecorded"
	}
	available := "unknown (no source reachable)"
	if src.FS != nil {
		available = SkillVersion(src)
	}
	// TWO VERSIONS, deliberately. They used to be one number because the skill was compiled in, and
	// "which skill am I running" was answerable only as "whichever this binary shipped".
	fmt.Fprintf(w, "ferret doctor: binary %s · skill %s (installed from %s) · available %s\n",
		binVersion, installed, from, available)
	if len(problems) == 0 {
		fmt.Fprintf(w, "  ok — deployed copy matches the binary, both commands resolve\n")
		return 0
	}
	for _, s := range problems {
		fmt.Fprintf(w, "  ! %s\n", s)
	}
	return 1
}

// linkAll creates every command entry, or none. Both entries are one table (installing one and not
// the other IS the original defect), so the failure of any one undoes the rest.
//
// Order is SORTED, not map order. The original defect returned on the first error while ranging a
// map, so which entry survived a half-install depended on Go's randomised iteration — a failure
// that reproduces differently every run is one nobody can diagnose from a report.
func linkAll(p paths, force bool) (int, error) {
	targets := linkTargets(p)
	links := make([]string, 0, len(targets))
	for link := range targets {
		links = append(links, link)
	}
	sort.Strings(links)

	var made []string
	undo := func() {
		for _, l := range made {
			// Best-effort: this runs on a path that is already failing, and a failure to undo must
			// not mask the failure being reported.
			_ = os.Remove(l)
		}
	}
	for _, link := range links {
		if err := relink(link, targets[link], force); err != nil {
			undo()
			return 3, err
		}
		made = append(made, link)
	}
	return 0, nil
}

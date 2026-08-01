// Command slop-ferret is the tool half of the slop-ferret method.
//
//	slop-ferret plan <map-dir> <sha> <repo> [--since <ref>]   > plan.json
//	slop-ferret verify <plan.json> <discharge.json>            ; 0 settled, 3 items open
//	slop-ferret update [--ref main]                            pull the skill from the repo
//	slop-ferret install [--from <dir>] [--force]               deploy a skill tree
//	slop-ferret doctor                                         drift, in both directions
//	slop-ferret version                                        binary and embedded skill versions
//
// The name is the hunter, not the quarry: this ferrets slop out, it does not produce it.
//
// The split it enforces: DETERMINISTIC TRANSFORMS belong here, JUDGEMENT belongs in the skill.
// Enumerating files and computing coverage fractions need no model. Deciding whether a finding
// clears its pre-filing bar does, and no amount of Go will do it — which is also why the HTML
// report is authored rather than generated, and why its spec lives in the skill where it can be
// revised without a binary release.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/robot-accomplice/slop-ferret/internal/gate"
	"github.com/robot-accomplice/slop-ferret/internal/install"
)

// binVersion is this binary's own version, and it is DELIBERATELY not the skill's. They were one
// number while the skill was compiled in, which meant a lexicon wording change needed a binary
// release to reach a sweep. Two artifacts, two cadences, two versions.
const binVersion = "0.1.0"

const usage = `slop-ferret — ferrets AI slop out of a repository

  slop-ferret plan <map-dir> <sha> <repo> [--since <ref>]   > plan.json
  slop-ferret verify <plan.json> <discharge.json>            0 settled, 3 items open
  slop-ferret update [--ref main]                            pull the skill from the repo
  slop-ferret install [--from <dir>] [--force]               deploy a skill tree into ~/.claude
  slop-ferret doctor                                         drift, in both directions
  slop-ferret version                                        binary + skill versions

The skill (SKILL.md, the lexicon) ships separately from this binary and updates on its own
cadence: ` + "`update`" + ` pulls it from the repo, ` + "`install`" + ` falls back to the copy compiled in.

Pairs with magma, which builds the call map ` + "`plan`" + ` reads. Run magma first; a map of a
different tree is refused by construction.`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is main's body with the process boundary lifted out, so dispatch is testable. main() itself
// then contains nothing that can be wrong except the wiring a compiler already checks.
func run(argv []string, stdout, stderr io.Writer) int {
	if len(argv) < 1 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	args := argv[1:]
	switch argv[0] {
	case "plan":
		return cmdPlan(args, stdout, stderr)
	case "verify":
		return cmdVerify(args, stdout, stderr)
	case "update":
		return cmdUpdate(args, stdout, stderr)
	case "install":
		return cmdInstall(args, stdout, stderr)
	case "doctor":
		return install.Doctor(stdout, install.EmbeddedSource(binVersion), binVersion)
	// `--version` is spelled both ways on purpose: release.yml parses `--version` to check the tag
	// against the binary, matching the sibling projects' release gate.
	case "version", "--version", "-v":
		fmt.Fprintf(stdout, "slop-ferret %s · embedded skill %s\n", binVersion,
			install.SkillVersion(install.EmbeddedSource(binVersion)))
		return 0
	default:
		fmt.Fprintln(stderr, usage)
		return 2
	}
}

func flagValue(args []string, name string) ([]string, string) {
	for i := 0; i < len(args); i++ {
		if args[i] == name && i+1 < len(args) {
			v := args[i+1]
			return append(args[:i:i], args[i+2:]...), v
		}
	}
	return args, ""
}

func has(args []string, name string) bool {
	for _, a := range args {
		if a == name {
			return true
		}
	}
	return false
}

// cmdUpdate is the reason the skill is no longer welded to the binary: prose changes land here
// without a rebuild.
func cmdUpdate(args []string, stdout, stderr io.Writer) int {
	_, ref := flagValue(args, "--ref")
	src, cleanup, err := install.Fetch(ref)
	if err != nil {
		fmt.Fprintf(stderr, "slop-ferret: %v\n", err)
		fmt.Fprintln(stderr, "  the installed skill is untouched; `install` still works offline "+
			"from the copy compiled in")
		return 2
	}
	defer cleanup()
	return install.Install(stdout, src, has(args, "--force"))
}

func cmdInstall(args []string, stdout, stderr io.Writer) int {
	args, from := flagValue(args, "--from")
	src := install.EmbeddedSource(binVersion)
	if from != "" {
		s, err := install.DirSource(from)
		if err != nil {
			fmt.Fprintf(stderr, "slop-ferret: %v\n", err)
			return 2
		}
		src = s
	}
	return install.Install(stdout, src, has(args, "--force"))
}

func fail(err error, stderr io.Writer) int {
	code := 2
	if e, ok := err.(*gate.Err); ok {
		code = e.Code
	}
	fmt.Fprintf(stderr, "slop-ferret: %v\n", err)
	return code
}

func cmdPlan(args []string, stdout, stderr io.Writer) int {
	args, since := flagValue(args, "--since")
	if len(args) != 3 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	p, err := gate.BuildPlan(args[0], args[1], args[2], since)
	if err != nil {
		return fail(err, stderr)
	}
	b, _ := json.MarshalIndent(p, "", " ")
	fmt.Fprintln(stdout, string(b))
	return 0
}

func cmdVerify(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	res, code, err := gate.Verify(args[0], args[1])
	if err != nil {
		return fail(err, stderr)
	}
	b, _ := json.MarshalIndent(res, "", " ")
	fmt.Fprintln(stdout, string(b))
	return code
}

// Command ferret is the tool half of the slop-ferret method.
//
//	ferret plan <map-dir> <sha> <repo> [--since <ref>]   > plan.json
//	ferret verify <plan.json> <discharge.json>            ; 0 settled, 3 open, 4 refused
//	ferret install|update [--ref <r>] [--from <dir>]      acquire and deploy the skill
//	ferret doctor                                         drift, in both directions
//	ferret version                                        binary version
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

const usage = `ferret — ferrets AI slop out of a repository

  ferret plan <map-dir> <sha> <repo> [--since <ref>]   > plan.json
  ferret verify <plan.json> <discharge.json>            0 settled · 3 items open · 4 refused
  ferret install [--ref <ref>] [--from <dir>]           acquire the skill and deploy it
  ferret update                                         synonym of install
  ferret doctor                                         drift, in both directions
  ferret version                                        binary version

The skill (SKILL.md, the lexicon) is NOT compiled into this binary. install fetches it from the
repository at the tag matching this binary's version; --ref tracks something else, --from reads a
local checkout.

Pairs with magma, which builds the call map plan reads. Run magma first; a map of a different tree
is refused by construction.`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is main's body with the process boundary lifted out, so dispatch is testable. main() itself
// then contains nothing that can be wrong except the wiring a compiler already checks.
func run(argv []string, stdout, stderr io.Writer) int {
	if len(argv) < 1 {
		fmt.Fprintln(stderr, usage)
		return gate.ExitMisuse
	}
	args := argv[1:]
	switch argv[0] {
	case "plan":
		return cmdPlan(args, stdout, stderr)
	case "verify":
		return cmdVerify(args, stdout, stderr)
	case "install", "update": // D4: synonyms — both acquire prose and deploy it
		return cmdInstall(args, stdout, stderr)
	case "doctor":
		// doctor describes what is ON DISK, so it must work with no source at all. "I cannot reach
		// the network" is not a reason to refuse to report the deployed copy.
		src, cleanup, err := sourceFor(args)
		if err != nil {
			return install.Doctor(stdout, install.Source{}, binVersion)
		}
		defer cleanup()
		return install.Doctor(stdout, src, binVersion)
	// `--version` is spelled both ways on purpose: release.yml parses it to check the tag against
	// the binary, matching the sibling projects' release gate. It reports only the BINARY version —
	// the binary no longer carries prose, so it has no skill version to report (D3).
	case "version", "--version", "-v":
		fmt.Fprintf(stdout, "ferret %s\n", binVersion)
		return gate.ExitOK
	default:
		fmt.Fprintln(stderr, usage)
		return gate.ExitMisuse
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

// sourceFor turns the flags into a Source. Order matters: an explicit --from or --ref beats the
// default, and the default is the repository at this binary's own version (D8). There is no
// compiled-in copy to fall back on, so every path here acquires prose from somewhere real.
func sourceFor(args []string) (install.Source, func(), error) {
	noop := func() {}
	if _, dir := flagValue(args, "--from"); dir != "" {
		s, err := install.DirSource(dir)
		return s, noop, err
	}
	if _, ref := flagValue(args, "--ref"); ref != "" {
		return install.Fetch(ref)
	}
	return install.DefaultSource(binVersion)
}

func cmdInstall(args []string, stdout, stderr io.Writer) int {
	src, cleanup, err := sourceFor(args)
	if err != nil {
		fmt.Fprintf(stderr, "ferret: %v\n", err)
		fmt.Fprintln(stderr, "  the installed skill is untouched")
		return gate.ExitMisuse
	}
	defer cleanup()
	return install.Install(stdout, src, has(args, "--force"))
}

func fail(err error, stderr io.Writer) int {
	code := gate.ExitMisuse
	if e, ok := err.(*gate.Err); ok {
		code = e.Code
	}
	fmt.Fprintf(stderr, "ferret: %v\n", err)
	return code
}

func cmdPlan(args []string, stdout, stderr io.Writer) int {
	args, since := flagValue(args, "--since")
	if len(args) != 3 {
		fmt.Fprintln(stderr, usage)
		return gate.ExitMisuse
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
		return gate.ExitMisuse
	}
	res, code, err := gate.Verify(args[0], args[1])
	if err != nil {
		return fail(err, stderr)
	}
	b, _ := json.MarshalIndent(res, "", " ")
	fmt.Fprintln(stdout, string(b))
	return code
}

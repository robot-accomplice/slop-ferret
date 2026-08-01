// Command slop is the tool half of the slop-ferret method.
//
//	slop install [--force]   deploy the embedded skill into ~/.claude
//	slop doctor              report drift, in both directions
//	slop version             the embedded skill version
//
// The split is deliberate and it is the same split everywhere: DETERMINISTIC TRANSFORMS belong
// here, JUDGEMENT belongs in the skill. Enumerating files, computing coverage fractions and
// laying out a report need no model, and all three were being done by hand — one of them badly
// enough to ship two HTML defects in a single report. Deciding whether a finding clears its bar
// does need a model, and no amount of Go will do it.
//
// This is a tool for the person running the sweep. It is not an evaluation of them, it has one
// user, and there is nothing to defend against — so its outputs are a work queue and an honest
// instrument reading, never a score.
package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"

	"github.com/robot-accomplice/slop/internal/gate"
	"github.com/robot-accomplice/slop/internal/install"
)

//go:embed all:skill
var skillFS embed.FS

const usage = `slop — companion tool for the slop-ferret sweep

  slop plan <map-dir> <sha> <repo> [--since <ref>]   > plan.json
  slop verify <plan.json> <discharge.json>            ; 0 settled, 3 items open
  slop install [--force]                              deploy the embedded skill into ~/.claude
  slop doctor                                         drift, in both directions
  slop version                                        the embedded skill version

Pairs with magma (github.com/robot-accomplice/magma), which builds the call map slop plans from.
Run magma first; slop refuses a map of a different tree by construction.`

func main() {
	install.SkillFS = skillFS
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "plan":
		os.Exit(cmdPlan(os.Args[2:]))
	case "verify":
		os.Exit(cmdVerify(os.Args[2:]))
	case "install":
		os.Exit(install.Install(os.Stdout, len(os.Args) > 2 && os.Args[2] == "--force"))
	case "doctor":
		os.Exit(install.Doctor(os.Stdout))
	case "version":
		fmt.Println(install.Version())
	default:
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
}

func fail(err error) int {
	code := 2
	if e, ok := err.(*gate.Err); ok {
		code = e.Code
	}
	fmt.Fprintf(os.Stderr, "slop: %v\n", err)
	return code
}

func cmdPlan(args []string) int {
	since := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--since" {
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "slop: --since needs a git ref")
				return 2
			}
			since = args[i+1]
			args = append(args[:i], args[i+2:]...)
			break
		}
	}
	if len(args) != 3 {
		fmt.Fprintln(os.Stderr, usage)
		return 2
	}
	p, err := gate.BuildPlan(args[0], args[1], args[2], since)
	if err != nil {
		return fail(err)
	}
	b, _ := json.MarshalIndent(p, "", " ")
	fmt.Println(string(b))
	return 0
}

func cmdVerify(args []string) int {
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, usage)
		return 2
	}
	res, code, err := gate.Verify(args[0], args[1])
	if err != nil {
		return fail(err)
	}
	b, _ := json.MarshalIndent(res, "", " ")
	fmt.Println(string(b))
	return code
}

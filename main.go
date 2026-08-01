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
	"fmt"
	"os"

	"github.com/robot-accomplice/slop/internal/install"
)

//go:embed all:skill
var skillFS embed.FS

const usage = `slop — companion tool for the slop-ferret sweep

  slop install [--force]   deploy the embedded skill into ~/.claude (both command entries)
  slop doctor              report drift between the embedded skill and the deployed copy
  slop version             the embedded skill version

  plan/verify are not ported yet — see python/gate.py. Deferred deliberately: their behaviour is
  pinned by 44 tests whose measurements (ANCHOR anchoring, tier split, defer floor) were each
  derived from a real repo, and porting them in the same pass as this restructure would leave
  neither half checkable. install/doctor went first because they are the part that must be a
  binary: they run before any toolchain exists, and a half-finished install is what let a
  pre-registered control run holding the two tools the skill withholds.`

func main() {
	install.SkillFS = skillFS
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	switch os.Args[1] {
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

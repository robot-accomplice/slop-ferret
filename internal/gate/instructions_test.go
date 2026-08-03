package gate

import (
	"reflect"
	"strings"
	"testing"
)

// The plan's `instructions` string is the machine-readable definition of the discharge a sweeper
// writes — the role the removed `discharge` command filled before it was deleted. FromSweep and the
// record read
// six OPTIONAL attested fields (tier, checked_clean, near_misses, findings_verified,
// findings_suspected, report_path) that the instructions never named, so a sweeper had no way to
// know they exist: discharge.json never carried them, FromSweep read them empty, and the report's
// "near-misses are shown" and "checked-clean carries its method" guarantees were unreachable from
// any documented input — prose describing behaviour no input could produce, which is this tool's
// own subject.
//
// The required set is DERIVED from the Discharge struct, never hand-listed: a hand list records
// what the author believed the struct held and stays green when a field is renamed or added. Break
// it: delete any field's json name from the `instructions` const and this goes red.
func TestEveryDischargeFieldIsNamedInThePlanInstructions(t *testing.T) {
	for tag := range jsonTagsOf(reflect.TypeOf(Discharge{})) {
		if !strings.Contains(instructions, tag) {
			t.Errorf("discharge field %q is consumed by the code (record.go/report.go) but is not "+
				"named in the plan instructions — a sweeper cannot produce a field the spec never "+
				"mentions, so any page guarantee that reads it is unreachable", tag)
		}
	}
}

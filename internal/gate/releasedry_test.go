package gate

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// `just release-dry` calls itself "a dry run of release.yml": it exists so the release checklist can
// verify the archives before publishing. That is only true if it cross-compiles exactly the
// platform set release.yml ships. justfile kept building windows/amd64 after release.yml dropped it,
// so the dry run green-lit a windows archive that would never ship and skipped verifying nothing it
// actually publishes — the checklist's own verification step verifying the wrong thing.
//
// Extract each file's `for t in <platforms>; do` list and assert they match. Break it: add or remove
// a platform in one file only, and this goes red.
func TestReleaseDryMirrorsReleaseYmlPlatforms(t *testing.T) {
	root := repoRoot(t)
	loopRe := regexp.MustCompile(`for t in ([a-z0-9/ ]+); do`)

	platforms := func(relPath string) []string {
		b, err := os.ReadFile(filepath.Join(root, relPath))
		if err != nil {
			t.Fatal(err)
		}
		m := loopRe.FindStringSubmatch(string(b))
		if m == nil {
			t.Fatalf("no `for t in ...; do` build loop found in %s — the match broke", relPath)
		}
		ps := strings.Fields(m[1])
		sort.Strings(ps)
		return ps
	}

	dry := platforms("justfile")
	ship := platforms(".github/workflows/release.yml")
	if !reflect.DeepEqual(dry, ship) {
		t.Errorf("release-dry builds %v but release.yml ships %v — the dry run verifies a different "+
			"asset set than what ships", dry, ship)
	}
}

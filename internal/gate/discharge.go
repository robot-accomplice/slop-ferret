package gate

import (
	"encoding/json"
	"fmt"
)

// Skeleton builds a discharge pre-populated with every item the plan raised, each marked
// undispositioned.
//
// WHY THE BINARY PRODUCES THIS. Assembling a discharge is deterministic work: it is the plan's item
// list, transposed. Having a model do it costs tokens, varies run to run, and is where every
// malformed discharge came from — a mistyped sha that binds to no plan, a `families_not_run` that
// forgets an unseeded family, a candidate quietly omitted rather than refuted. None of those are
// judgement calls; they are transcription.
//
// So the tool transcribes and the model decides. The agent's remaining job is the part only it can
// do: move each path from `unread` to `read_paths` or `coverage_waived`, and each candidate from
// `candidates_undispositioned` to cleared or refuted, having actually looked.
//
// The skeleton is deliberately WRONG until edited — every path unread, every candidate
// undispositioned — so an unedited skeleton enumerates as incomplete rather than passing silently.
// A template that defaults to "done" is a template that gets submitted unread.
func Skeleton(pl *Plan) ([]byte, error) {
	if pl.SHA == "" {
		return nil, fmt.Errorf("plan has no sha; cannot bind a discharge to it")
	}
	unread := make([]string, 0, len(pl.ProductionFiles))
	unread = append(unread, pl.ProductionFiles...)

	cands := make([]map[string]string, 0, len(pl.Candidates))
	for _, c := range pl.Candidates {
		cands = append(cands, map[string]string{
			"file": c.File, "symbol": c.Symbol, "class": c.Class, "bar": c.Bar,
		})
	}

	// Pre-filled from the plan: these are facts the plan already states, and requiring the agent to
	// retype them only creates opportunities to retype them wrong.
	out := map[string]any{
		"sha":              pl.SHA,
		"families_not_run": nonNil(pl.UnseededFamilies),

		"read_paths":      []string{},
		"coverage_waived": []string{},

		"candidates_cleared": []map[string]string{},
		"candidates_refuted": []map[string]string{},
		"candidates_filed":   []map[string]string{},

		"tier":               "",
		"checked_clean":      []map[string]string{},
		"near_misses":        []string{},
		"findings_verified":  0,
		"findings_suspected": 0,
		"report_path":        "",

		// Worklists for the agent, stripped by `enumerate` — they are scaffolding, not claims.
		"_unread":                     unread,
		"_candidates_undispositioned": cands,
		"_how": "Move each path from _unread into read_paths (you read it) or coverage_waived " +
			"(you decided not to; a reason is optional). Move each entry from " +
			"_candidates_undispositioned into candidates_refuted (looked, discarded) or " +
			"candidates_cleared (cleared its bar) — and into candidates_filed if you accused it. " +
			"Fields starting with _ are scaffolding and are ignored.",
	}
	return json.MarshalIndent(out, "", " ")
}

package report

// The page. Self-contained by construction: no external stylesheet, script, font or image, so it
// can be opened or forwarded as a single file and never phones anywhere.
//
// Required order, and it is not cosmetic: coverage BEFORE results, because a reader must know the
// shape of the sweep before they read its output. Then findings severity-first. VERIFIED and
// SUSPECTED are distinguished VISUALLY, not by a caption, so the difference survives skimming.
const htmlTemplate = `<meta charset="utf-8">
<title>Slop sweep — {{.In.Repo}} @ {{.In.SHA}}</title>
<style>
:root{--bg:#fbfbfa;--panel:#fff;--ink:#1a1a1a;--muted:#6b6b6b;--line:#e2e0dc;--accent:#4a5d7e;
 --block:#a8332a;--block-bg:#fbeeec;--fix:#8a6212;--fix-bg:#fcf5e6;--note:#4a6b52;--note-bg:#eef4ef;
 --ver:#2f6b4f;--ver-bg:#eaf3ee;--sus:#7a6a3a;--sus-bg:#f7f3e6;
 --mono:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace}
@media (prefers-color-scheme:dark){:root{--bg:#151617;--panel:#1d1f21;--ink:#e8e6e3;--muted:#9a9691;
 --line:#32353a;--accent:#8fa8cc;--block:#e8776c;--block-bg:#2e1e1c;--fix:#d8ab5a;--fix-bg:#2b2519;
 --note:#8fbf9f;--note-bg:#1c2620;--ver:#7fc79f;--ver-bg:#1a2620;--sus:#cbb779;--sus-bg:#282416}}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--ink);font:15px/1.55 ui-sans-serif,-apple-system,"Segoe UI",Roboto,sans-serif}
.wrap{max-width:62rem;margin:0 auto;padding:2rem 1.25rem 5rem}
h1{font-size:1.5rem;margin:0 0 .15rem}
h2{font-size:1.05rem;margin:2.4rem 0 .8rem;padding-bottom:.35rem;border-bottom:1px solid var(--line);
 text-transform:uppercase;letter-spacing:.02em;color:var(--muted);font-weight:600}
.sub{color:var(--muted);font-size:.85rem;margin:0 0 1.4rem}
code,.mono{font-family:var(--mono);font-size:.86em}
.banner{background:var(--panel);border:1px solid var(--line);border-left:5px solid var(--accent);
 border-radius:8px;padding:1.1rem 1.25rem;margin:1.2rem 0}
.banner .v{font-size:1.35rem;font-weight:700}
.warn{background:var(--block-bg);border:1px solid var(--block);border-radius:8px;padding:.9rem 1.1rem;margin:1rem 0}
.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(11rem,1fr));gap:.6rem;margin:1rem 0}
.stat{background:var(--panel);border:1px solid var(--line);border-radius:7px;padding:.7rem .85rem}
.stat .k{font-size:.7rem;text-transform:uppercase;letter-spacing:.06em;color:var(--muted)}
.stat .val{font-size:1.25rem;font-weight:650;font-variant-numeric:tabular-nums;margin-top:.15rem}
.stat .d{font-size:.75rem;color:var(--muted)}
.tbl{overflow-x:auto;margin:.8rem 0}
table{border-collapse:collapse;width:100%;font-size:.88rem;min-width:30rem}
th,td{text-align:left;padding:.45rem .6rem;border-bottom:1px solid var(--line);vertical-align:top}
th{font-size:.7rem;text-transform:uppercase;letter-spacing:.05em;color:var(--muted)}
.f{border:1px solid var(--line);border-radius:8px;margin:.8rem 0;background:var(--panel);overflow:hidden}
.f.VERIFIED{border-left:5px solid var(--ver)}
.f.SUSPECTED{border-left:5px dashed var(--sus);
 background:repeating-linear-gradient(135deg,transparent,transparent 9px,var(--sus-bg) 9px,var(--sus-bg) 18px)}
.f>.hd{padding:.75rem 1rem .45rem;display:flex;flex-wrap:wrap;gap:.5rem;align-items:baseline}
.f>.hd .t{font-weight:650;flex:1 1 18rem;min-width:0}
.f>.bd{padding:0 1rem 1rem}
.loc{font-family:var(--mono);font-size:.78rem;color:var(--muted);display:block;margin:.1rem 0 .5rem}
dl{margin:.5rem 0 0;display:grid;grid-template-columns:max-content 1fr;gap:.3rem .9rem;font-size:.87rem}
dt{color:var(--muted);font-size:.7rem;text-transform:uppercase;letter-spacing:.05em;padding-top:.18rem}
dd{margin:0}
.pill{display:inline-block;font-size:.66rem;font-weight:700;letter-spacing:.06em;padding:.12rem .45rem;
 border-radius:3px;text-transform:uppercase;white-space:nowrap}
.block{background:var(--block-bg);color:var(--block);border:1px solid var(--block)}
.fix{background:var(--fix-bg);color:var(--fix);border:1px solid var(--fix)}
.note{background:var(--note-bg);color:var(--note);border:1px solid var(--note)}
.st-VERIFIED{background:var(--ver-bg);color:var(--ver);border:1px solid var(--ver)}
.st-SUSPECTED{background:var(--sus-bg);color:var(--sus);border:1px solid var(--sus)}
ul{margin:.5rem 0;padding-left:1.2rem}li{margin:.25rem 0}
.foot{margin-top:3rem;padding-top:1rem;border-top:1px solid var(--line);color:var(--muted);font-size:.8rem}
</style>
<div class="wrap">
<h1>Slop sweep — {{.In.Repo}}</h1>
<p class="sub"><span class="mono">{{.In.SHA}}</span> · tier {{.In.Tier}} · skill {{.In.SkillVersion}} · lexicon {{.In.LexiconVer}}</p>

<div class="banner">
  <div class="v">Accounting: {{.In.Accounting}}</div>
  <div class="sub" style="margin:.35rem 0 0">
    The auditor states <strong>{{.In.AttestedRepo}}</strong> source files read ·
    <strong>{{.In.AttestedPlan}}</strong> of the plan dispositioned{{if .In.Waived}} · {{.In.Waived}} waived (counted as unread){{end}}.
    <br><strong>This tool does not observe reading.</strong> Those figures are the auditor's own
    statement, reported as such — not a measurement of what was read.
  </div>
</div>
{{if .Tell}}<div class="warn"><strong>{{.Tell}}</strong></div>{{end}}
{{if .In.Remaining}}<div class="warn"><strong>Still open:</strong><ul>{{range .In.Remaining}}<li>{{.}}</li>{{end}}</ul></div>{{end}}

<h2>Coverage — what was and was not covered</h2>
<div class="grid">
  <div class="stat"><div class="k">Accounting</div><div class="val">{{.In.Accounting}}</div><div class="d">of items raised</div></div>
  <div class="stat"><div class="k">Stated read</div><div class="val">{{.In.AttestedRepo}}</div><div class="d">auditor's statement</div></div>
  <div class="stat"><div class="k">Verified</div><div class="val">{{.Verified}}</div><div class="d">cleared a pre-filing bar</div></div>
  <div class="stat"><div class="k">Suspected</div><div class="val">{{.Suspected}}</div><div class="d">excluded from the rate</div></div>
  <div class="stat"><div class="k">Rate</div><div class="val" style="font-size:.95rem;padding-top:.3rem">{{.Rate}}</div><div class="d">denominator {{.In.Denominator}}</div></div>
</div>
<div class="tbl"><table><thead><tr><th>Families run</th><th>Not run</th></tr></thead>
<tbody><tr><td>{{range .In.FamiliesRun}}{{.}} {{end}}</td><td>{{range .In.FamiliesNot}}{{.}} {{end}}</td></tr></tbody></table></div>
{{if .In.MapLimitations}}<p class="sub"><strong>Declared map limitations:</strong> {{range .In.MapLimitations}}{{.}} · {{end}}</p>{{end}}

<h2>Findings — severity first</h2>
{{if not .In.Findings}}<p>None recorded.</p>{{end}}
{{range .In.Findings}}
<div class="f {{.Status}}">
  <div class="hd">
    <span class="pill {{sevclass .Severity}}">{{.Severity}}</span>
    <span class="pill st-{{.Status}}">{{.Status}}</span>
    <span class="t">{{.Title}}</span>
  </div>
  <div class="bd">
    <span class="loc">{{.File}} · {{.Class}}{{if gt .Occurrences 1}} · {{.Occurrences}} occurrences{{end}}</span>
    <dl>
      {{if .Claim}}<dt>Claim</dt><dd>{{.Claim}}</dd>{{end}}
      {{if .Refutation}}<dt>Refutation</dt><dd>{{.Refutation}}</dd>{{end}}
      {{if .Bar}}<dt>Bar</dt><dd>{{.Bar}}</dd>{{end}}
      {{if .Evidence}}<dt>Evidence</dt><dd>{{.Evidence}}</dd>{{end}}
      {{if .Remediation}}<dt>Remediation</dt><dd>{{.Remediation}}</dd>{{end}}
    </dl>
  </div>
</div>
{{end}}

{{if .In.NearMisses}}<h2>Near-misses — refuted before filing</h2><ul>{{range .In.NearMisses}}<li>{{.}}</li>{{end}}</ul>{{end}}
{{if .In.CheckedClean}}<h2>Checked-clean, with the method</h2>
<div class="tbl"><table><thead><tr><th>Class</th><th>Method used</th></tr></thead><tbody>
{{range .In.CheckedClean}}<tr><td>{{.Class}}</td><td>{{.Method}}</td></tr>{{end}}</tbody></table></div>{{end}}

<div class="foot">
  <p>{{.In.Repo}} @ <span class="mono">{{.In.SHA}}</span> · skill {{.In.SkillVersion}} · lexicon {{.In.LexiconVer}} · denominator {{.In.Denominator}}</p>
  <p>Figures describing what was read are the auditor's statement. This tool reports; it does not verify the audit.</p>
  <p>Local file. Not published.</p>
</div>
</div>
`

# C4 Context Diagram

Shows slop-ferret in the context of the systems and actors it interacts with.

```mermaid
graph TB
    operator["Operator<br/><i>Runs the sweep</i>"]
    agent["Sweeping agent<br/><i>Reads code, judges findings,<br/>authors the report</i>"]

    sf["slop-ferret<br/><i>Enumerates what to read,<br/>reports coverage</i>"]

    magma["magma<br/><i>Parses the repo once,<br/>emits the call map</i>"]
    target["Target repository<br/><i>Read-only: git ls-files, git diff</i>"]
    claude["~/.claude<br/><i>Deployed skill +<br/>command entries</i>"]
    repo["github.com/robot-accomplice/slop-ferret<br/><i>Skill assets at a ref</i>"]

    operator -->|"Invokes via CLI"| sf
    operator -->|"Invokes /slop-ferret"| agent
    magma -->|"codemap-rows/1<br/>(_dead, _test-only)"| sf
    sf -->|"reads tracked paths"| target
    agent -->|"reads code, never writes"| target
    sf -->|"install / update<br/>writes skill + 2 commands"| claude
    claude -->|"SKILL.md, lexicon,<br/>allowed-tools"| agent
    repo -->|"update: HTTPS tarball<br/>at a resolved commit"| sf
    agent -->|"plan.json / discharge.json"| sf
    sf -->|"two coverage fractions<br/>+ a work queue"| agent
```

## Boundaries worth naming

**slop-ferret never writes to the target repository.** The sweep method it supports is
additive-only against the target by design, and the tool's only reads there are `git ls-files` and
`git diff --name-only`.

**The agent, not the tool, decides anything.** slop-ferret raises candidates and counts coverage;
whether a candidate is a real finding is a judgement call that stays with whoever is sweeping. The
tool cannot establish that a file was *read* — `read_paths` is self-reported and nothing here
corroborates it. Attestation is still worth requiring because it makes an omission a statement
someone made rather than a gap nobody owns.

**`~/.claude` is a security boundary, not just a config directory.** The command entries determine
which tools a sweep session is granted. A skill that cannot be invoked is a skill whose
`allowed-tools` never applies — which is how a sweep once ran holding two tools it was meant to be
denied.

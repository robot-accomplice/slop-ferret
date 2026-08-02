# Assets

## The mascot

`ferret-mascot-source.png` is the canonical artwork: commissioned, raster-only, 1254×1254, 8-bit
RGB, **no alpha**, opaque white background. **There is no SVG source and never was** — any vector
version in existence is a trace of this file, not a sibling of it.

This repository owns the mascot. roboticus.ai's project-card icon is a *derived* asset (head crop,
traced, colour stripped, tuned for 28px) and lives inline in that site's `ProjectIcons.tsx` because
its icon set is driven by `currentColor`. That is a documented derivation, not a second copy: if
the mascot is ever redrawn, the icon is regenerated from it rather than edited to match.

## Derivation

| file | what | how |
|---|---|---|
| `ferret-mascot-source.png` | canonical, untouched | as commissioned |
| `ferret-mascot.png` | light-theme hero | alpha recovered, cropped, 560px |
| `ferret-mascot-dark.png` | dark-theme hero | as above, ink recoloured |

The art is brown ink composited over white, so alpha is recovered per pixel from the red channel
(`a = (255 − r) / (255 − 0x6e)`), which has the widest ink/paper spread. Values below 8/255 are
floored to fully transparent — otherwise near-white paper texture keeps a residual alpha and the
image will not crop to its content.

## Why there are two

Measured, not assumed:

| | ink | vs `#ffffff` | vs GitHub dark `#0d1117` |
|---|---|---|---|
| light hero | `#6e3f1c` | **8.78** | 2.15 — fails |
| dark hero | `#b7692f` | 2.6 | **4.57** |

The source has no alpha, so used directly it renders as a **white box** on GitHub's dark theme, and
its brown fails WCAG AA against that background regardless. The README selects between the two with
`<picture media="(prefers-color-scheme: dark)">`.

**`currentColor` is not an option here.** The traced SVGs bake in no fill so they inherit colour
from CSS — which works inside a React component and does nothing inside a README `<img>`, where
there is no colour context and `currentColor` resolves to black. Hence two baked variants.

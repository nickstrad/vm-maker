---
name: draw-visual
description: Draw terminal-readable Unicode diagrams from Mermaid (preferred) or PlantUML sources and maintain regenerable Markdown embeds. Use for architecture, topology, flow, or sequence diagrams intended for plain-text reading, or to update existing draw-visual embeds. Not for raster images or graphical SVG diagrams.
argument-hint: "[what to draw, the Markdown file to embed it in, and where the source should live]"
context: fork
agent: visual-drawer
---

# Draw a visual

Request: $ARGUMENTS

Use this skill for diagrams that must read well in terminals and plain-text Markdown.
You write a diagram source, render it to Unicode text, and embed that text in Markdown.
The source is the truth; rendered text is never edited by hand.

If the requested diagram or target is unclear, ask for the missing information.

Resolve helper and reference paths relative to `${CLAUDE_SKILL_DIR}`; use its scripts for rendering and snapshots.

## Steps

1. Read the target Markdown file and any document the request names, so labels match its actor names.
2. Make sure the renderer exists: `scripts/setup.sh` (idempotent; add `--with-plantuml` only when you need PlantUML).
3. Choose the diagram type using the table below, then read `reference.md` for syntax that renders cleanly and for known failures.
4. Write the source beside the document, in a `diagrams/` folder unless the request says otherwise. Name it `<document-stem>-<purpose>.mmd`, or use a shared prefix when several documents embed one source.
5. Render it: `scripts/render.py path/to/source.mmd`. Fit the reader's available content width, accounting for margins. If unknown, start with 70 columns and check 80 too. Set `DRAW_VISUAL_MAX_WIDTH` for `--check`; the historical 110-column default is a renderer ceiling, not evidence that the diagram fits the reader's terminal.
6. Iterate on the source until it is clean: shorten labels, change direction, tune padding options, or split one picture into two. A wrong-looking diagram is worse than none; if it cannot be made clean, say so and propose a table or list instead.
7. Embed it. Put this marker and an empty fenced block where the diagram belongs, then run `render.py --update FILE.md`:

   ````markdown
   <!-- draw-visual: diagrams/3-topology.mmd -->
   ```text
   ```
   ````

8. Run `DRAW_VISUAL_MAX_WIDTH=70 scripts/render.py --check FILE.md` (adjust to the actual target). Capture the final reader-facing plain output when available, then run `bash scripts/snapshot.sh --input OUTPUT.md --out SNAPSHOT_DIR --columns 70,80`. See [snapshot-review.md](snapshot-review.md) for prerequisites, raw-text input, manifests, and limitations.
9. Open and visually inspect every generated PNG at readable scale. Check complete labels, spacing, connected arrows, wrapping, clipping, and reading order. Fix sources and regenerate both embeds and snapshots after changes. Generation and width checks alone do not finish validation. If images cannot be opened, report inspection as pending. Report source/embed paths, widths, reviewed viewport sizes, and limitations; clean temporary images after review when requested.

## Choosing a diagram type

| The visual shows | Use | Notes |
| --- | --- | --- |
| Actors exchanging messages over time | Mermaid `sequenceDiagram` | Best-looking output. Supports `autonumber`, notes, lost messages, `alt`, `loop`. |
| Components fanning out from a hub | Mermaid `graph LR` | At most about ten nodes. No subgraphs. Short edge labels or arrow numbers with a key below. |
| A chain or fan-out that exceeds the reader's width | Mermaid `graph TD` | Keeps labels intact in narrow terminals; inspect the longer vertical layout. |
| Anything containing a cycle or feedback loop | Mermaid `graph TD` | The back-edge returns up the side as a closed rectangle. `graph LR` cycles garble labels or crash the renderer. |
| Sequence needing PlantUML-only features | PlantUML sequence | Text mode supports sequences only, and crashes on `== divider ==`. |
| Timelines, tables of state, cut-point maps | Not this skill | Hand-written text or a Markdown table communicates better. |

## Conventions

- Follow the target project's instructions and diagram conventions.
- Use the actor names the document already uses; introduce human actors only when they belong in the requested diagram.
- Preserve the document's distinction between planned and implemented behavior.
- Keep render options in the source's first line (`%% draw-visual: -x 4 -y 1 -p 0`) so re-renders match.
- Change prose outside the embed or commit only when the user requests it.

## Renderer installation

The scripts install tools beneath `${DRAW_VISUAL_HOME:-$HOME/.local/share/draw-visual}` and do not change system packages. Mermaid rendering needs Go for initial installation; optional PlantUML setup downloads a private Linux x64 JRE and jar (about 200 MB). Initial installation needs network access. Python 3 runs `render.py`. Both skill variants use the same renderer installation.

`scripts/snapshot.sh` builds its pinned Go helper in the same private bin directory. It uses an installed monospaced font and produces PNG inspection artifacts from delivered text; the document's diagram remains text. The first build may download Go modules. These previews are not screenshots of the user's actual app.

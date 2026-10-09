# Inspecting rendered diagram snapshots

Use snapshots for new or changed text diagrams and when reviewing existing diagrams.
An embed freshness check cannot detect an unreadable layout. The 98-column Firecracker
boot map passed the old 110-column check but wrapped through boxes in the reader's
terminal. Fit the available content width and inspect images of delivered output.

## Commands

Run from this skill's directory, or use absolute helper paths:

```bash
DRAW_VISUAL_MAX_WIDTH=70 python3 scripts/render.py --check /path/to/document.md
bash scripts/snapshot.sh --input /path/to/document.md --out /tmp/diagram-review --columns 70,80
```

Prefer output from the actual presentation command, such as `tutor ... lesson --plain`,
when a catalog or renderer sits between source and reader. Markdown mode extracts every
fenced `text` block, preserving spaces and source line numbers. Mermaid markers identify
blocks when present; headings provide labels otherwise. Shell/SQL fences are excluded.
Source checks and served-output review are complementary.

For raw output from Mermaid or PlantUML:

```bash
python3 scripts/render.py /path/to/source.mmd > /tmp/diagram.txt
bash scripts/snapshot.sh --text --input /tmp/diagram.txt --out /tmp/diagram-review --columns 70,80
```

The helper emits one PNG per block and width, plus `manifest.json` with the input
SHA-256, block identity, content width, viewport width, and wrapped source line numbers.
Overflow returns nonzero **after** producing images for diagnosis. No text blocks or
malformed fences also fail. This is not an image-diff approval test. The manifest always
says review is pending; record completed inspection separately in task validation notes.

## Review and iterate

- Open every PNG with the image-viewing tool. Use original resolution or focused views
  when a long sequence is scaled too small to read.
- Trace arrows to their intended actors; check complete labels, note placement, padding,
  clipping, wrapped borders, and reading order. Narrowness alone does not prove clarity.
- Preserve labels when changing direction or splitting a diagram. Keep source options
  and embeds synchronized, then inspect fresh images after the final edit.
- Record inspected widths and per-diagram outcomes locally. Retain scripts and concise
  results, not permanent copies of disposable screenshots. Delete uploaded/temporary
  images when requested; preserve unrelated images and evidence still needed for review.

## Prerequisites and limits

The launcher builds `scripts/snapshot/` with pinned Go modules and the Go cache; first
use may need network access. It does not install system packages. The default font is
`/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf`; override with `--font /path/to/monospace.ttf`.
The font must contain the diagram's glyphs. Run `go test ./...` inside `scripts/snapshot/`
when changing parsing or width behavior.

PNGs simulate fixed-cell terminal wrapping using a 20-point font. They are **not**
screenshots of the reader's Mac, browser, Markdown styling, or terminal emulator. They
validate text geometry at selected widths, not application-specific zoom, margins,
font, or theme. Use actual application screenshots when those are the issue.
Tabs use eight-cell stops; ANSI CSI colors are removed. Box-drawing characters use one
cell and ordinary CJK characters two. Emoji shaping, bidirectional text, cursor-motion
sessions, and exact combining-glyph placement are outside this helper's scope. Prefer
plain output and inspect glyph fidelity.

Codex and Claude have independent skill installations. Keep this file, `reference.md`,
and shared scripts byte-identical; preserve each skill's routing/frontmatter. Both use
`DRAW_VISUAL_HOME` for private binaries.

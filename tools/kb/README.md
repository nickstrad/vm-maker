# kb (vendored)

A copy of the `kb` knowledge-store CLI (upstream: `~/Software/kb`, module
`github.com/nickstrad/kb`), vendored so vm-maker carries its **own** knowledge store: findings
about this repo live in `data/` and travel with the code, separate from the droplet-wide store at
`/root/Raw/knowledge`. Upstream docs are in [UPSTREAM-README.md](UPSTREAM-README.md); entry
format and workflow are in `skills/update-knowledge-store/SKILL.md`.

## Run it

Always go through the wrapper. It builds the binary on first use (needs Go + gcc; cgo) and forces
`KB_ROOT=tools/kb`, so it ignores any inherited `KB_ROOT` and never touches the global store.

```bash
tools/kb/bin/kb-local search "how many VMs can one command create" --caller claude
tools/kb/bin/kb-local list
tools/kb/bin/kb-local show vm-maker-invariants.md
tools/kb/bin/kb-local add /abs/path/to/draft.md     # drafts live outside data/
tools/kb/bin/kb-local doctor
```

Do not use the global `kb` on PATH here: without `KB_ROOT` it points at `/root/Raw/knowledge`.
Embeddings use local Ollama (`nomic-embed-text`) by default; set `KB_EMBEDDER=none` for
full-text only, or `OPENROUTER_API_KEY` for OpenRouter (then reindex with `--all`).

## Layout

| Path | Committed? |
| --- | --- |
| `data/*.md` | yes: the corpus |
| `.kb/kb.sqlite` | no: generated index plus search/feedback log |
| `.cache/kb` | no: built binary |

## Rebuild

```bash
tools/kb/bin/kb-local reindex --all            # recreate .kb/kb.sqlite from data/
KB_REBUILD=1 tools/kb/bin/kb-local version     # rebuild the binary after source changes
```

Reindexing a fresh clone restores search, but search history and feedback are local to each
checkout and are not recreated.

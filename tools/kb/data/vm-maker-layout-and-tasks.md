---
title: vm-maker repo layout and deno tasks
summary: Run deno task start/dev/test/check/compile from the repo root; src/ holds the code, architecture.md the design, tools/kb this knowledge store.
tags: [vm-maker, layout, deno, tasks]
updated: 2026-10-05
---

# vm-maker repo layout and deno tasks

Run from `/root/Software/vm-maker` (Deno is at `/root/.deno/bin`, not on PATH):

```bash
deno task start            # deno run src/main.ts  -> Hello, world!
deno task start Ada        # Hello, Ada!
deno task dev              # deno run --watch src/main.ts
deno task test             # deno test (unit + property tests)
deno task check            # deno fmt --check && deno lint && deno check src/
deno task compile          # single binary at bin/vm-maker
```

## Current layout

| Path | What it is |
| --- | --- |
| `deno.json`, `deno.lock` | Tasks, imports, compiler options |
| `src/main.ts`, `src/greet.ts`, `src/greet_test.ts` | Hello-world scaffold |
| `architecture.md` | Design: invariants, modules, Effect layers, testing strategy |
| `docs/cli.md`, `docs/diagrams/` | Candidate CLI shapes and diagram sources |
| `tools/kb/` | Vendored kb CLI and this repo's knowledge store |

## Planned `src/` layout (from architecture.md)

`cli/`, `config/`, `domain/`, `policy/` (pure resolve), `planner/` (pure plan), `executor/`
(bounded execution), `cloudinit/`, `labels/`, `reaper/`, `providers/{port.ts,hetzner,digitalocean,fake}`,
plus top-level `templates/` and `test/fixtures/`.

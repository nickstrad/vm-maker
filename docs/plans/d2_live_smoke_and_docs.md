# d2 · Live smoke test and docs alignment

**Wave d** · builder-fast (sonnet high / luna high: documentation sweep verified against the
code, plus one opt-in test) · depends on every c slice · owns `test/live/**`, `README.md`,
`docs/cli.md`, `docs/architecture.md` (status wording and the logging section only)

## What this slice gives us

Proof against real providers that the contract holds, on demand only: `deno task test:live`
creates one smallest VM per configured provider with the lab cloud-init, shows it, stops,
starts, and deletes it, and prints any id it failed to delete for manual cleanup. It also
brings the README and docs back in line with what runs: the README currently promises TTLs,
reapers, and managed-VM caps that v1 does not have.

## Architecture of the slice

<!-- draw-visual: diagrams/d2-smoke.mmd -->
```text
        ┌────────────┐     ┌────────┐     ┌───────────────┐
        │ smoke test │     │ runCli │     │ real provider │
        └──────┬─────┘     └────┬───┘     └───────┬───────┘
               │                │                 │
               │ 1. create smoke-ts --wait --user-data lab.yaml
               ├───────────────►│                 │
               │                │                 │
               │                │ 2. one createVm, polls
               │                ├────────────────►│
               │                │                 │
               │                │ 3. running      │
               │                │◄┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┤
               │                │                 │
               │ 4. show, stop --yes --wait, start --wait
               ├───────────────►│                 │
               │                │                 │
               │ 5. delete --yes --wait           │
               ├───────────────►│                 │
               │                │                 │
               │                │ 6. deleteVm, polls until absent
               │                ├────────────────►│
               │                │                 │
┌─────────────────────────────────────────────────────────────────┐
│ finally: delete any id still present, else print MANUAL CLEANUP │
└─────────────────────────────────────────────────────────────────┘
               │                │                 │
```

## Code plan

| File | Contents |
| --- | --- |
| `test/live/smoke_test.ts` | Skips unless `VM_MAKER_LIVE=1`. For each provider whose token variable is set: pick the smallest type from `catalog types --within-limits` for a region from env (`VM_MAKER_LIVE_REGION_<PROVIDER>`) or the provider's cheapest well-known region; run `create smoke-<timestamp> --wait`, `show`, `stop --yes --wait`, `start --wait`, `delete --yes --wait` through `runCli` with the real layers from d1 and `--output json`; assert each envelope. A `finally` block attempts `delete --yes` on any created id and, on failure, writes `MANUAL CLEANUP REQUIRED: <provider> <id>` to stderr and fails the test. Uses `cloud-init/lab.yaml` as `--user-data` without rendering (placeholders stay empty, which the file tolerates). |
| `test/live/README.md` | How to run, what it costs (one small VM per provider for a few minutes), which env variables it reads, and the manual cleanup note. |
| `README.md` | Rewrite **Why**, **Status**, and **Getting started** to match cli.md: no TTL, reaper, count cap, or templates; list the commands; show the permission flags from d1 and the `config` sample; link `cloud-init/README.md` (track p; do not edit it); state that `deno task test` never contacts a provider. |
| `docs/cli.md` | Add `VM_MAKER_LOG_LEVEL` to **Shared options and output** next to `--verbose`, and one sentence that diagnostics are JSONL lines on stderr. No other contract change. |
| `docs/architecture.md` | Add a short **Observability** paragraph (JSONL stderr logging, span path in errors) and update the opening sentence that says the implementation is a hello-world scaffold. Keep diagrams untouched. |

Decisions already made; do not reopen:

- The live test is never part of `deno task test`, `deno task check`, or any CI default.
- It creates at most one VM per provider per run and deletes it in cleanup.
- Cleanup failure is a test failure with ids printed; nothing is retried blindly.

## Test plan

- **Offline guard:** `deno task test` must still pass with `test/live/` present (the skip
  path), and the live file must not be matched by the default task's include pattern or
  must self-skip without `VM_MAKER_LIVE`.
- **Live run (manual, documented in the status file):** paste the redacted JSON envelopes
  from one successful run per provider under **Notes for peers** in
  `docs/plans/status/d2.md`, with ids scrubbed.
- **Docs review:** every command in cli.md's command surface appears in the README; the
  README's "Status" paragraph names the waves that are merged; `docs/plans/README.md` wave
  table is updated to mark d as merged by the coordinator, not by this slice.
- **Reviewer checks:** `grep -n "ttl\|reap\|TTL" README.md` is empty; the draw-visual
  embeds in architecture.md still pass `render.py --check`.

## Hand-off

None. After d2 merges, v1 is complete; resize and other future commands start a new plan
set.

# c2 · stop, start, delete

**Wave c** · agent `implementer-opus-high` · depends on b1, b4, b6 · owns
`src/commands/lifecycle.ts`, `src/confirm/**`, `src/cli/handlers/stop.ts`,
`src/cli/handlers/start.ts`, `src/cli/handlers/delete.ts`, `src/cli/render/receipt.ts`

## What this slice gives us

One-ID lifecycle mutations with the safety rules from
[cli.md](../cli.md#stop-start-and-delete) and
[architecture.md](../architecture.md#lifecycle-operations): fetch current state, refuse
transitional or unknown power states as conflicts, succeed without a mutation when already
in the requested state, preview on `--dry-run` without prompting, confirm stop and delete
unless `--yes`, re-fetch after the prompt and refuse if state changed, send exactly one
mutation, and optionally wait. Non-TTY without `--yes` exits 7.

## Architecture of the slice

<!-- draw-visual: diagrams/c2-lifecycle.mmd -->
```text
┌────────────────────────────┐   ┌──────────────┐   ┌─────────────┐
│handlers stop, start, delete├──►│render receipt├──►│envelope (b1)│
└──────────────┬─────────────┘   └──────────────┘   └─────────────┘
               │
               │                 ┌──────────────┐   ┌─────────────┐
               └────────────────►│ runLifecycle ├──►│decide [pure]│
                                 └───────┬──────┘   └─────────────┘
                                         │
                                         │          ┌─────────────┐
                                         ├─────────►│ Confirmation│
                                         │          └─────────────┘
                                         │
                                         │          ┌─────────────┐
                                         ├─────────►│ ProviderPort│
                                         │          └─────────────┘
                                         │
                                         │          ┌─────────────┐
                                         └─────────►│ waitFor (b6)│
                                                    └─────────────┘
```

The decision in the middle is pure and shared by all three commands.

<!-- draw-visual: diagrams/c2-decision.mmd -->
```text
┌──────────────────┐
│    current Vm    │
└─────────┬────────┘
          ▼
┌──────────────────┐
│  status stable?  │
└─────────┬────────┘
          │
          ├─────────────────────────────┐
          │                             │
         no                            yes
          │                             │
          ▼                             ▼
┌──────────────────┐    ┌──────────────────────────────┐
│ Conflict, exit 3 │    │      already in target?      │
└──────────────────┘    └───────────────┬──────────────┘
                                        │
          ┌─────────────────────────────┤
          │                             │
         yes                           no
          │                             │
          ▼                             ▼
┌──────────────────┐    ┌──────────────────────────────┐
│   noop, exit 0   │    │           dry-run?           │
└──────────────────┘    └───────────────┬──────────────┘
                                        │
          ┌─────────────────────────────┤
          │                             │
         yes                           no
          │                             │
          ▼                             ▼
┌──────────────────┐    ┌──────────────────────────────┐
│preview, no prompt│    │confirm, re-fetch, mutate once│
└──────────────────┘    └──────────────────────────────┘
```

## Code plan

| File | Contents |
| --- | --- |
| `src/commands/lifecycle.ts` | Pure `decide(action: stop \| start \| delete, vm: Vm) -> Noop \| Mutate \| Conflict`: `stop` on `off` and `start` on `running` are `Noop`; `transitioning` or `unknown` is `Conflict` with the raw status in the message; `delete` on any stable state is `Mutate`. Pure `preview(action, vm) -> ActionPreview` (provider, id, name, action, raw status, attached resources, billing note for delete). Pure `unchanged(before: Vm, after: Vm) -> boolean` comparing status and raw status. `runLifecycle(request) -> Effect<Receipt, AppError>`: resolve provider, `getVm`, `decide`; dry-run returns the preview; `Noop` returns a `noop` receipt; otherwise confirm when required, re-fetch, check `unchanged` (else `Conflict`), send the one mutation, then `waitFor` when `--wait` with the matching condition (`off`, `running`, `absent`). Steps wrapped with `Effect.fn("lifecycle.<step>")`. |
| `src/confirm/service.ts` | `Confirmation` service: `confirm(preview) -> Effect<boolean, Aborted>`. Pure `confirmationRequired({ action, yes, dryRun }) -> boolean` (stop and delete, not start; never with `--yes` or `--dry-run`). |
| `src/confirm/tty.ts` | `ConfirmationTty` layer using `effect/cli` `Prompt.confirm` on stderr; when stdin is not a TTY it fails `Aborted` with the hint `pass --yes to skip the prompt`. |
| `src/confirm/fake.ts` | `ConfirmationFake(answers: boolean[])` layer recording every preview shown. |
| `src/cli/handlers/stop.ts`, `start.ts`, `delete.ts` | Replace stubs: build the `LifecycleRequest`, run, render. |
| `src/cli/render/receipt.ts` | Text for previews (`would stop hetzner 51234567 (scratch)`), noop (`already off`), accepted receipts with action id, and the delete note about independently billed resources. |

Decisions already made; do not reopen:

- Delete refuses on `transitioning` too, so a VM mid-create or mid-shutdown is never
  deleted on a stale view.
- The prompt text includes provider, id, name, and action, and for delete the attached
  resource references when the provider returned any.
- Confirmation declined is `Aborted` (exit 7) and sends nothing; a changed state after the
  prompt is `Conflict` (exit 3) and sends nothing.
- `--wait` on a `noop` receipt returns immediately with the current VM.

## Test plan

Fake provider and fake confirmation layers; `TestClock` for `--wait`.

- **MC/DC on `decide`:** conditions `isStop`, `isStart`, `isDelete`, `statusRunning`,
  `statusOff`, `statusStable` (transitioning/unknown false). Table enumerating stop×{running,
  off, transitioning, unknown}, start×same, delete×same; expected `Mutate, Noop, Conflict,
  Conflict` for stop, `Noop, Mutate, Conflict, Conflict` for start, `Mutate, Mutate,
  Conflict, Conflict` for delete.
- **MC/DC on `confirmationRequired`:** conditions `isStopOrDelete`, `yes`, `dryRun`. Rows:
  stop plain (required); start plain (not); stop with `--yes` (not); stop with `--dry-run`
  (not).
- **Examples:** dry-run stop records zero mutations and never calls `confirm`; declined
  confirmation records zero mutations and exits 7; non-TTY without `--yes` exits 7 with the
  hint; state changing between prompt and re-fetch (fake `settleNow` inside the fake
  confirmation callback) exits 3 with zero mutations; stop on an already-off VM exits 0 with
  `noop` and zero mutations; delete on a VM larger than the limits proceeds; delete with
  `lost` fault exits 5 with `outcome-unknown`, vm id retained, and the hint to run `show`;
  missing id exits 6; `--wait` after stop returns the `off` VM; `--wait` timeout exits 9
  and the receipt keeps vm and action ids.
- **Property:** for any `Vm` arbitrary and action, `decide` returns `Conflict` exactly when
  status is `transitioning` or `unknown`, and `Mutate` never when `decide` would be `Noop`.
- **Reviewer checks:** exactly one port mutation call per successful command run
  (`FakeHandle.mutations().length === 1`); `src/confirm/tty.ts` writes prompts to stderr.

## Hand-off

- c3 reuses `Confirmation` only if create ever needs it (it does not in v1) and shares
  `render/receipt.ts` for accepted receipts.
- d2 live smoke test uses `--yes` paths only.

# b6 · Bounded waiting

**Wave b** · builder-strong (opus high / sol high: Schedule and TestClock deadline semantics) ·
depends on a1, b4 (tests only; use a1's port type for code) · owns `src/wait/**`

## What this slice gives us

The `--wait` behavior for every mutation: poll the provider's action status and the resulting
VM condition with a finite Effect `Schedule`, stop on success, on a provider-reported action
error, or when the deadline expires (default 3 minutes, `--timeout` up to 10 minutes). Poll
errors consume the same deadline. Timeout never deletes, resubmits, or rolls back anything;
it returns `WaitTimeout` with the VM and action ids so `show` can inspect later.

## Architecture of the slice

<!-- draw-visual: diagrams/b6-wait.mmd -->
```text
┌─────────┐     ┌─────────┐     ┌──────┐
│ command │     │ waitFor │     │ port │
└────┬────┘     └────┬────┘     └───┬──┘
     │               │              │
     │ 1. receipt, condition, deadline
     ├──────────────►│              │
  ┌─[loop every 2s until deadline]────────┐
  │  │               │              │     │
  │  │               │ 2. getAction(id)   │
  │  │               ├─────────────►│     │
  │  │               │              │     │
  │  │               │ 3. ActionStatus    │
  │  │               │◄┈┈┈┈┈┈┈┈┈┈┈┈┈┤     │
  │  │               │              │     │
  │  │               │ 4. getVm(id) │     │
  │  │               ├─────────────►│     │
  │  │               │              │     │
  │  │               │ 5. Vm or NotFound  │
  │  │               │◄┈┈┈┈┈┈┈┈┈┈┈┈┈┤     │
  │  │               │              │     │
  │┌────────────────────────────────────┐ │
  ││ evaluate: settled, pending, failed │ │
  │└────────────────────────────────────┘ │
  │  │               │              │     │
  └───────────────────────────────────────┘
   ┌─[alt settled]─────────────────────┐
   │ │               │              │  │
   │ │ 6. last Vm    │              │  │
   │ │◄┈┈┈┈┈┈┈┈┈┈┈┈┈┈┤              │  │
   ├┈[deadline]┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┤
   │ │               │              │  │
   │ │ 7. WaitTimeout with ids, exit 9 │
   │ │×┈┈┈┈┈┈┈┈┈┈┈┈┈┈┤              │  │
   │ │               │              │  │
   └───────────────────────────────────┘
     │               │              │
```

## Code plan

| File | Contents |
| --- | --- |
| `src/wait/condition.ts` | `WaitCondition` union: `{ kind: "status", vmId, target: running \| off }`, `{ kind: "absent", vmId }`, `{ kind: "created", vmId }` (status becomes `running` or `off`, anything stable). Pure `evaluate(condition, observation) -> settled \| pending \| failed` where `observation = { action?: ActionStatus, vm?: Vm, vmMissing: boolean }`. Action state `error` is `failed`; `absent` settles on `vmMissing`. |
| `src/wait/schedule.ts` | Pure `pollSchedule(deadline: Duration) -> Schedule`: fixed 2-second spacing, capped by `deadline`, and a hard cap on poll count of `ceil(deadline / 2s) + 1` so even a misbehaving clock terminates. Pure `validateTimeout(requested?: Duration) -> Result<Duration, Usage>` applying default 3 minutes and maximum 10 minutes. |
| `src/wait/poll.ts` | `waitFor(port, receipt, condition, deadline) -> Effect<Vm \| undefined, WaitTimeout \| AppError>`: each tick calls `getAction` when the receipt has an action id, then `getVm` (mapping `NotFound` to `vmMissing: true`), evaluates, and repeats on `pending`. Transient `Transport` errors from a poll are logged at debug and count as `pending`; `Auth` and `ProviderRejected` abort immediately. Runs under `Effect.fn("wait.poll")` with `annotateLogs({ vmId, actionId })` and a debug line per tick with `elapsedMs`. |
| `src/wait/mod.ts` | Barrel. |

Decisions already made; do not reopen:

- Spacing is constant, not exponential; the provider APIs are cheap to poll and the
  deadline is short.
- `noop` receipts (already in the requested state) skip waiting entirely; the command layer
  decides that, `waitFor` is never called with one.
- The returned `Vm` is the last observation, so the command can render final state without
  another read; for `absent` it returns `undefined`.

## Test plan

All timing tests use `TestClock` from `effect/testing`; no real sleeps.

- **MC/DC on `evaluate` for `status`:** conditions `actionErrored`, `vmPresent`,
  `statusIsTarget`. Rows: baseline (action ok, present, target) settled; action errored
  gives failed regardless; vm missing gives failed for `status`; status not target gives
  pending.
- **MC/DC on `validateTimeout`:** conditions `given`, `withinMax`. Rows: absent gives 3 min;
  given and within gives itself; given over 10 min gives `Usage`.
- **Property:** for any deadline between 1 second and 10 minutes, `pollSchedule` emits at
  most `ceil(deadline / 2s) + 1` ticks and the last tick's elapsed time is ≤ deadline.
- **Layer tests with the fake provider (b4):** shutdown settles after two polls and the
  returned `Vm` is `off`; delete settles when `getVm` is `NotFound`; a fake whose action
  never leaves `running` yields `WaitTimeout` after exactly the deadline with `vmId` and
  `actionId` set and zero mutations recorded after the receipt; a `Transport` fault on the
  first poll is absorbed and the second poll settles; an `Auth` fault aborts without
  consuming the deadline.
- **Reviewer checks:** no `Date.now`, `setTimeout`, or `Effect.sleep` with a literal
  duration outside `schedule.ts`.

## Hand-off

- c2 and c3 call `validateTimeout` during request decoding and `waitFor` after a receipt
  when `--wait` is set.

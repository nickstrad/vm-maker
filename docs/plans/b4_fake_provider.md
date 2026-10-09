# b4 · Fake provider

**Wave b** · builder-fast (sonnet high / luna high: in-memory fake over a fixed port; the tests
are the spec) · depends on a1 · owns `src/providers/fake/**`

## What this slice gives us

An in-memory implementation of `ProviderPort` that every command test in wave c runs
against. Tests seed it with VMs (including ones no vm-maker ever created), catalogs, and
scripted failures; it records every mutation so a test can assert "dry-run sent zero
mutations" or "create submitted exactly one request". It is test infrastructure, not a CLI
mode, and keeps no state file.

## Architecture of the slice

<!-- draw-visual: diagrams/b4-fake.mmd -->
```text
┌──────────────────────────────┐
│    command tests (c wave)    │
└───────────────┬──────────────┘
                ▼
┌──────────────────────────────┐
│FakeSeed: vms, catalog, faults│
└───────────────┬──────────────┘
                ▼
┌──────────────────────────────┐
│      FakeProvider.layer      ├────────────────────┐
└───────────────┬──────────────┘                    │
                ▼                                   ▼
┌──────────────────────────────┐   ┌────────────────────────────────┐
│      ProviderPort (a1)       │   │FakeHandle: mutations, settleNow│
└───────────────┬──────────────┘   └────────────────────────────────┘
                ▼
┌──────────────────────────────┐
│      transitions [pure]      │
└───────────────┬──────────────┘
                ▼
┌──────────────────────────────┐
│          Ref state           │
└──────────────────────────────┘
```

## Code plan

| File | Contents |
| --- | --- |
| `src/providers/fake/state.ts` | `FakeSeed = { provider, vms: Vm[], catalog: Catalog, actionsPerPoll?: number, faults?: Fault[] }`. `Fault` union: `{ on: "getVm", id, error: AppError }`, `{ on: "listVms", incomplete: IncompleteRead }`, `{ on: "createVm" \| "shutdownVm" \| "powerOnVm" \| "deleteVm", id?, mode: "lost" \| "reject", error?: AppError }`, `{ on: "getAction", id, sequence: ActionStatus["state"][] }`. State lives in a `Ref` so concurrent fibers are safe. |
| `src/providers/fake/transitions.ts` | Pure functions over `Vm[]`: `applyShutdown(vms, id)` sets status `transitioning` with raw `stopping`, `applyPowerOn`, `applyDelete` removes, `applyCreate(vms, resolved, nextId)` appends a `transitioning` VM with raw `initializing`. Pure `settle(vms)` moves every `transitioning` VM to its target stable state; the fake calls it after `actionsPerPoll` polls of `getAction` (default 2) so `wait/` tests see real progression. |
| `src/providers/fake/port.ts` | `FakeProvider.layer(seed) -> Layer<ProviderPort \| FakeHandle>`. `FakeHandle` service exposes `mutations(): MutationCall[]` (operation, args, timestamp), `vms(): Vm[]`, `seedFault(fault)`, `settleNow()`. `listVms` applies region filter and the `incomplete` fault; `getVm` on a missing id fails `NotFound`; mutations with a `lost` fault record the call, apply the transition, and fail `OutcomeUnknown` (the provider did accept it); `reject` records nothing and fails with the given error. `encodeCreate` returns a deterministic redacted preview. |
| `src/providers/fake/arbitraries.ts` | Append to `test/support/arbitraries.ts` a `fakeSeed` arbitrary (consistent catalog and VMs sharing region ids and type ids); keep it in this file and re-export from the support file by appending one line. |
| `src/providers/fake/mod.ts` | Barrel. |

Decisions already made; do not reopen:

- Two fakes with different `provider` ids can be provided at once (c1 uses this for
  `list --provider all`); the layer takes the provider id from the seed.
- A `lost` mutation still mutates the fake state. That is the whole point of
  `OutcomeUnknown`: the user must look before retrying.
- The fake never sleeps; progression is counted in polls, so `TestClock` is unnecessary
  here and `wait/` tests control time themselves.

## Test plan

- **Examples:** seeded unlabelled VM appears in `listVms` and `getVm`; region filter
  excludes others; `deleteVm` then `getVm` is `NotFound`; `shutdownVm` leaves the VM
  `transitioning` and two `getAction` polls settle it to `off`; `lost` fault records one
  mutation and fails `OutcomeUnknown` with the VM still changed; `reject` fault records no
  mutation; `incomplete` fault returns rows plus the `IncompleteRead`.
- **Property:** for any `fakeSeed`, every `Vm` returned by `listVms` decodes against the
  `Vm` schema; `applyDelete` is idempotent; `settle(settle(vms)) === settle(vms)`.
- **Reviewer checks:** no `Date.now()` or `setTimeout` in `src/providers/fake`; the layer
  provides both `ProviderPort` and `FakeHandle`.

## Hand-off

- c1, c2, c3 build their tests on `FakeProvider.layer` and assert on `FakeHandle.mutations()`.
- b6 uses the `actionsPerPoll` progression to test polling against a realistic port.

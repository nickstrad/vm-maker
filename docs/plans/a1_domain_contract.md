# a1 · Domain contract

**Wave a** · builder-deep (fable high / astra high: the contract every later slice imports; a
wrong shape is expensive to change) · depends on nothing · owns `src/domain/**`,
`src/providers/port.ts`, `test/support/arbitraries.ts`

## What this slice gives us

Every later slice imports the same vocabulary: VM identity, normalized VM and catalog models,
the command request and result types, the typed error union, the exit code mapping, and the
provider port interface. With this merged, wave b can run six slices in parallel without
anyone inventing a conflicting type. Nothing user-facing runs yet, but `deno task test`
proves that models decode provider-shaped values, unit normalization is exact, and every
error maps to the exit code [cli.md](../cli.md#shared-options-and-output) promises.

## Architecture of the slice

Everything here is pure. `domain/` has no imports from other `src/` folders;
`providers/port.ts` imports only `domain/`.

<!-- draw-visual: diagrams/a1-modules.mmd -->
```text
┌───────────────────┐   ┌──────────┐
│     ids, units    │ ┌─┤  errors  │
└─────────┬─────────┘ │ └─────┬────┘
          ▼           │       ▼
┌───────────────────┐ │ ┌──────────┐
│    vm, catalog    │ │ │exit codes│
└─────────┬─────────┘ │ └──────────┘
          ▼           │
┌───────────────────┐ │
│inventory, mutation│ │
└─────────┬─────────┘ │
          ▼           │
┌───────────────────┐ │
│      requests     │ │
└─────────┬─────────┘ │
          ▼───────────┘
┌───────────────────┐
│ providers/port.ts │
└───────────────────┘
```

Later waves consume the contract as shown; the dashed targets do not exist yet.

<!-- draw-visual: diagrams/a1-consumers.mmd -->
```text
┌──────────────────────┐   ┌────────────────────┐
│domain/ + port.ts [a1]├─┬►│     cli/ [b1]      │
└───────────┬──────────┘ │ └────────────────────┘
            │            │
            │            │ ┌────────────────────┐
            ├────────────┼►│     http/ [b3]     │
            │            │ └────────────────────┘
            │            │
            │            │ ┌────────────────────┐
            ├────────────┼►│providers/fake/ [b4]│
            │            │ └────────────────────┘
            │            │
            │            │ ┌────────────────────┐
            ├────────────┼►│    policy/ [b5]    │
            │            │ └────────────────────┘
            │            │
            │            │ ┌────────────────────┐
            └────────────┼►│     wait/ [b6]     │
                         │ └────────────────────┘
                         └────────────▼
                           ┌────────────────────┐
                           │ adapters [c5, c6]  │
                           └────────────────────┘
```

## Code plan

Create these files. Use `Schema` classes so every model decodes and encodes; no plain
interfaces for data that crosses the port.

| File | Contents |
| --- | --- |
| `src/domain/ids.ts` | `ProviderId = Schema.Literals(["hetzner", "digitalocean"])`, `ProviderSelector` adds `"all"`, `VmId` (branded non-empty string), `ActionId`, `VmRef = { provider, id }`. |
| `src/domain/units.ts` | `Gb` branded non-negative number. Pure `gbFromMib(mib)` returning `mib / 1024` exactly (no rounding), `gbFromGb` identity. |
| `src/domain/vm.ts` | `PowerStatus = running \| off \| transitioning \| unknown`; `Vm` with provider, id, name, typeId, region, status, rawStatus, memoryGb, diskGb, ipv4, ipv6, image, createdAt, metadata (labels/tags as `Record<string,string>` or string array), attached resource references (`{ kind, id, name? }[]`). Optional fields are `Option` or `undefined` decided once here: use `Schema.optional` and `undefined`, never `null`. |
| `src/domain/catalog.ts` | `Arch = x86 \| arm`; `VmType` (id, name, vcpu, memoryGb, diskGb, arch, availableRegions: string[]); `Region` (id, name); `Image` (id, name, arch, availableRegions?); `Catalog = { types, regions, images, incomplete: IncompleteRead[] }`. |
| `src/domain/inventory.ts` | `Inventory = { rows: Vm[], incomplete: IncompleteRead[] }`; `IncompleteRead = { provider, reason: pages \| time \| cursor, pagesRead }`. |
| `src/domain/mutation.ts` | `MutationReceipt = { provider, vmId?, actionId?, status: accepted \| completed \| noop }`; `ActionStatus = { id, state: running \| success \| error, message? }`. `noop` is the already-in-state case. |
| `src/domain/requests.ts` | Decoded command inputs: `ListQuery`, `CatalogQuery`, `CreateRequest` (name, provider, sizing as a union `{ kind: "type", typeId } \| { kind: "minimums", vcpu, memoryGb, diskGb? }`, region?, image?, sshKeys, userData?: `{ bytes: Uint8Array, byteLength, sha256 }`, dryRun, wait, timeout?), `LifecycleRequest`. `ResolvedCreate` = CreateRequest with the chosen `VmType`, final region, image, and ssh keys. |
| `src/domain/limits.ts` | `Limits = { maxVcpu, maxMemoryGb, maxDiskGb }` positive finite; `DEFAULT_LIMITS = { 8, 8, 128 }`. Operational constants: `HTTP_TIMEOUT = 30s`, `READ_RETRY_BUDGET = 90s`, `READ_RETRIES = 2`, `MAX_PAGES = 100`, `PAGINATION_BUDGET = 3min`, `WAIT_DEFAULT = 3min`, `WAIT_MAX = 10min`. |
| `src/domain/errors.ts` | One `Schema.TaggedError` per variant, all sharing optional `provider`, `vmId`, `actionId`, `requestId`, `httpStatus`, `hint`, `cause` fields. Variants and codes: `Usage` (`usage`), `ConfigInvalid` (`config-invalid`), `Conflict` (`conflict`), `LimitExceeded` (`limit-exceeded`, with `limit`, `requested`, `maximum`), `Auth` (`auth`), `ProviderRejected` (`provider-rejected`), `Transport` (`transport`), `OutcomeUnknown` (`outcome-unknown`), `IncompleteInventory` (`incomplete-inventory`), `NotFound` (`not-found`), `Aborted` (`aborted`), `WaitTimeout` (`wait-timeout`), `Internal` (`internal`). Export `AppError` union and `isAppError`. |
| `src/domain/exit.ts` | Pure `exitCodeFor(e: AppError): ExitCode` with the table from cli.md: usage/config-invalid 2, conflict 3, limit-exceeded 4, auth/provider-rejected/transport/outcome-unknown/incomplete-inventory 5, not-found 6, aborted 7, wait-timeout 9, internal 1. Exhaustive `Match` so a new variant fails type-checking. |
| `src/domain/mod.ts` | Barrel. |
| `src/providers/port.ts` | `ProviderPort` service via `Context.Service` with the eight operations from architecture.md, each returning `Effect<_, AppError>`: `listVms(query) -> Inventory`, `getVm(id) -> Vm`, `getCatalog(query) -> Catalog`, `createVm(resolved) -> MutationReceipt`, `shutdownVm(id)`, `powerOnVm(id)`, `deleteVm(id)`, `getAction(id) -> ActionStatus`. Also `encodeCreate(resolved) -> RedactedPreview` so dry-run and real submission share one payload (user-data shown as byte length and hash). |
| `test/support/arbitraries.ts` | fast-check arbitraries for `VmType`, `Vm`, `Catalog`, `Limits`, `PowerStatus`, and `AppError`. Other slices extend this file only by appending exports. |

Decisions already made; do not reopen:

- IDs are opaque strings on both providers even though Hetzner uses integers.
- `memoryGb` and `diskGb` are plan capacity in GB; DigitalOcean MiB is divided by 1024 and
  kept fractional (`0.5` for 512 MiB).
- `PowerStatus.unknown` is a real value, not an error. Adapters map unfamiliar raw statuses
  to it and keep `rawStatus`.
- The port has no `waitFor` method; polling lives in `wait/` on top of `getAction`/`getVm`.
- `Internal` wraps any defect; it carries the original `cause` for `--verbose` rendering.

## Test plan

- **Exit mapping (MC/DC not needed, exhaustive instead):** one test per error variant
  asserting the code, plus a property test that `exitCodeFor` never returns `0` and always
  returns one of the documented codes for any `AppError` arbitrary.
- **Units (property):** `gbFromMib(n * 1024) === n` for non-negative integers; `gbFromMib`
  is monotonic; `gbFromMib(512) === 0.5`.
- **Schemas (examples):** each model decodes a representative literal and round-trips
  through encode/decode; `Vm` with every optional field absent decodes; `null` is rejected.
- **Error data (examples):** each error constructs with only `message`, exposes its `code`,
  and `isAppError` rejects a plain `Error`.
- **Reviewer checks:** `deno task check` clean; no file in `src/domain` imports outside
  `effect` and `src/domain`; `AppError` is a closed union (adding a variant without updating
  `exitCodeFor` fails `deno check`).

## Hand-off

- b1 renders `AppError` values and uses `exitCodeFor`.
- b3 constructs `Auth`, `ProviderRejected`, `Transport`, `OutcomeUnknown`,
  `IncompleteInventory` with `requestId` and `httpStatus`.
- b4 implements `ProviderPort` in memory; c5 and c6 implement it over HTTP.
- b5 consumes `VmType`, `Catalog`, `Limits`, `CreateRequest` and returns `ResolvedCreate`.

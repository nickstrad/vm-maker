# c3 · create

**Wave c** · builder-deep (fable high / astra high: integrates five modules, carries most exit
codes, and the redaction invariant must hold end to end) · depends on b1, b2, b4, b5, b6 · owns
`src/commands/create.ts`, `src/cli/handlers/create.ts`, `src/cli/render/create.ts`

## What this slice gives us

`vm-maker create NAME` end to end against the fake provider, following the six-step
[creation sequence](../architecture.md#creation-policy): decode flags and config, fetch the
live catalog, resolve one compatible type within limits with b5, read and check user-data,
encode one payload through the port, print the redacted preview on `--dry-run` or submit
once and optionally wait. Exit codes 2, 3, 4, 5, and 9 all have a path here.

## Architecture of the slice

<!-- draw-visual: diagrams/c3-create.mmd -->
```text
┌──────────────────────────────────────┐
│            handler create            │
└───────────────────┬──────────────────┘
                    ▼
┌──────────────────────────────────────┐
│            create command            │
└───────────────────┬──────────────────┘
                    ▼
┌──────────────────────────────────────┐
│    loadPrefs: ConfigService (b2)     │
└───────────────────┬──────────────────┘
                    ▼
┌──────────────────────────────────────┐
│    fetchCatalog: port.getCatalog     │
└───────────────────┬──────────────────┘
                    ▼
┌──────────────────────────────────────┐
│     resolve: resolveCreate (b5)      │
└───────────────────┬──────────────────┘
                    ▼
┌──────────────────────────────────────┐
│   readUserData: checkUserData (b5)   │
└───────────────────┬──────────────────┘
                    ▼
┌──────────────────────────────────────┐
│      encode: port.encodeCreate       │
└───────────────────┬──────────────────┘
                    ▼
┌──────────────────────────────────────┐
│submit: port.createVm once, or preview│
└───────────────────┬──────────────────┘
                    ▼
┌──────────────────────────────────────┐
│          wait: waitFor (b6)          │
└──────────────────────────────────────┘
```

## Code plan

| File | Contents |
| --- | --- |
| `src/commands/create.ts` | `create(request) -> Effect<CreateOutcome, AppError>` with `CreateOutcome = { kind: "preview", resolved, preview } \| { kind: "receipt", resolved, receipt, vm? }`. Steps, each an `Effect.fn("create.<step>")`: `selectProvider`; `loadPrefs` (region, image, sshKeys, limits from `ConfigService`); `fetchCatalog` via the port; `resolve` via b5 `resolveCreate`; `readUserData` (only when `--user-data` given: `Deno.readFile` the explicit path, then b5 `checkUserData` with the provider's byte limit); `encode` via `port.encodeCreate(resolved)`; `submit` via `port.createVm` when not dry-run; `wait` via b6 with condition `created` when `--wait`. Validate `--timeout` with b6 `validateTimeout` before any network call. |
| `src/cli/handlers/create.ts` | Replace the stub: decode into `CreateRequest`, run, render. Reads the user-data file here or in the command, but never logs its contents. |
| `src/cli/render/create.ts` | Text for the preview (`would create hetzner scratch: type cx22 (2 vCPU, 4 GB RAM, 40 GB disk) in fsn1 with ubuntu-24.04, ssh keys [..], user-data 1234 bytes sha256:abcd…`) and for receipts (vm id, action id, and `waiting` summary). JSON `data` for preview is `{ resolved, request: RedactedPreview }`. |

Decisions already made; do not reopen:

- The preview printed by `--dry-run` is the port's `encodeCreate` output, not a separate
  rendering, so what you see is what gets sent.
- User-data appears everywhere only as `{ byteLength, sha256 }`.
- One `createVm` call per invocation; `OutcomeUnknown` from the port is returned as is,
  with the hint to run `list` before retrying and the explicit warning that a retry can
  create a second VM.
- `--wait` deadline failures do not delete the VM; the receipt with ids is still printed in
  the error envelope's `partialData`.

## Test plan

Fake provider seeded with a catalog that has x86 and arm types, a type over the disk
ceiling, and a type unavailable in the chosen region.

- **MC/DC on step gating:** decision `dryRun` versus submit, and `wait && !dryRun` for
  polling. Rows: dry-run (zero mutations, preview printed, catalog fetched once); submit
  without wait (one mutation, no polls); submit with wait (one mutation, polls until
  `created`); dry-run with wait (zero mutations, zero polls, no error).
- **Examples:** `--memory 16` exits 4 before the catalog is fetched (assert `getCatalog`
  not called); explicit `--type` unavailable in region exits 3 with the compatible list;
  minimums satisfied only by a type whose disk is 160 GB exits 4 naming `maxDiskGb`; region
  missing from flags and config exits 2 with the hint; user-data file over the provider
  limit exits 2; `lost` fault exits 5 with `outcome-unknown` and the second-VM warning;
  `--wait` timeout exits 9 with the vm id in the envelope; limits raised in a test config
  allow the 160 GB type; `--timeout 11m` exits 2 before any port call.
- **Property:** for any fake seed and any minimums request that b5 can resolve, `create
  --dry-run` and `create` resolve the same type (determinism across the two invocations)
  and the submitted payload equals the dry-run preview's payload minus redaction.
- **Reviewer checks:** `grep -r "userData" src/commands/create.ts src/cli/render/create.ts`
  shows only `byteLength` and `sha256` reaching output; exactly one `createVm` call in
  the submit tests.

## Hand-off

- c5 and c6 implement `encodeCreate` and `createVm`; their contract tests assert the
  preview equals the request body with user-data replaced.
- d2 live smoke runs `create --wait` then `delete --yes --wait`.

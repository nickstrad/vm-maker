# c1 · list, show, catalog

**Wave c** · agent `implementer-sonnet-high` · depends on b1, b4, b5 · owns
`src/commands/list.ts`, `src/commands/show.ts`, `src/commands/catalog.ts`,
`src/commands/registry.ts`, `src/cli/handlers/list.ts`, `src/cli/handlers/show.ts`,
`src/cli/handlers/catalog*.ts`, `src/cli/render/inventory.ts`, `src/cli/render/catalog.ts`

## What this slice gives us

The read-only half of the CLI, end to end against the fake provider: `list` with region
filter and `--provider all` partial results, `show ID` with every detail column from
[cli.md](../cli.md#discover-and-inspect), and the three `catalog` subcommands including
`--within-limits`. Any existing lab VM becomes inspectable the moment real adapters land,
because these commands depend only on the port.

## Architecture of the slice

<!-- draw-visual: diagrams/c1-reads.mmd -->
```text
┌───────────────────────────────┐
│     cli handlers + render     │
└───────────────┬───────────────┘
                ▼
┌───────────────────────────────┐
│  commands list, show, catalog ├──────────────────┐
└───────────────┬───────────────┘                  │
                ▼                                  ▼
┌───────────────────────────────┐   ┌─────────────────────────────┐
│        ProviderRegistry       │   │limits (b2 config, b5 policy)│
└───────────────┬───────────────┘   └─────────────────────────────┘
                ▼
┌───────────────────────────────┐
│ProviderPort (b4 fake, d1 live)│
└───────────────────────────────┘
```

`list --provider all` is two independent reads whose outcomes are combined without one
hiding the other.

<!-- draw-visual: diagrams/c1-list-all.mmd -->
```text
   ┌──────┐     ┌─────────┐     ┌──────────────┐     ┌──────────┐
   │ list │     │ hetzner │     │ digitalocean │     │ envelope │
   └───┬──┘     └────┬────┘     └───────┬──────┘     └─────┬────┘
       │             │                  │                  │
       │ 1. listVms  │                  │                  │
       ├────────────►│                  │                  │
       │             │                  │                  │
       │ 2. listVms  │                  │                  │
       ├───────────────────────────────►│                  │
       │             │                  │                  │
       │ 3. rows     │                  │                  │
       │◄┈┈┈┈┈┈┈┈┈┈┈┈┤                  │                  │
       │             │                  │                  │
       │ 4. Auth error                  │                  │
       │×┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┤                  │
       │             │                  │                  │
┌───────────────────────────┐           │                  │
│ combine rows and failures │           │                  │
└───────────────────────────┘           │                  │
       │             │                  │                  │
       │ 5. ok false, rows, errors [digitalocean], exit 5  │
       ├──────────────────────────────────────────────────►│
       │             │                  │                  │
```

## Code plan

| File | Contents |
| --- | --- |
| `src/commands/registry.ts` | `ProviderRegistry` service: `forProvider(id: ProviderId) -> Effect<ProviderPort, Auth>` (fails `Auth` with a hint naming the token env variable when no credentials are configured). `ProviderRegistry.fromPorts(map)` test constructor over fake layers. Pure `selectProviders(selector, defaultProvider?) -> Result<ProviderId[], Usage>`: explicit flag wins, `all` expands to both, missing flag and missing default is `Usage` with a hint. |
| `src/commands/list.ts` | `list(query) -> Effect<ListOutcome, AppError>`. For one provider: `listVms`, normalized rows sorted by provider then name then id, `incomplete` entries turned into `IncompleteInventory` failure that carries `partialData` rows. For `all`: run both reads concurrently with `Effect.all({ concurrency: 2, mode: "either" })`-style collection; if both succeed and are complete, success; otherwise fail with the first error, `partialData` holding successful rows, and `errors` listing every failed provider. Wrapped in `Effect.fn("list")`. |
| `src/commands/show.ts` | `show(ref) -> Effect<Vm, AppError>`: resolve provider, `getVm`, `NotFound` passes through unchanged. |
| `src/commands/catalog.ts` | `catalogTypes(query)` applying region, arch, and `--within-limits` (via b5 `checkTypeAgainstLimits` with `ConfigService` limits); `catalogRegions()`; `catalogImages(query)` with region and arch filters. Incomplete catalog reads fail `IncompleteInventory` with partial data like `list`. |
| `src/cli/handlers/*.ts` | Replace the b1 stubs: decode the invocation into the command query, run the command with the registry, return `Outcome` with the renderer. |
| `src/cli/render/inventory.ts` | Pure text table for the documented columns `PROVIDER ID NAME TYPE REGION STATUS RAM_GB DISK_GB IPV4 IPV6`, `-` for missing values, fixed-width columns computed from content; `show` detail block with image, created, labels, attached resources, raw status. |
| `src/cli/render/catalog.ts` | Pure tables for types (`ID NAME VCPU RAM_GB DISK_GB ARCH REGIONS`), regions, images. |

Decisions already made; do not reopen:

- Credentials are resolved only for selected providers; `list --provider hetzner` must
  never touch the DigitalOcean token (the registry is lazy per provider).
- Partial inventory exits 5 with `ok: false`, `data` holding the successful rows, and one
  error per failed provider, exactly as cli.md states.
- `RAM_GB` prints fractional values without trailing zeros (`0.5`, `8`).
- Sorting is stable and defined here, not left to provider order.

## Test plan

Fake providers from b4, two of them for `all` tests.

- **MC/DC on `selectProviders`:** conditions `flagGiven`, `flagIsAll`, `defaultConfigured`,
  `commandIsList`. Rows: flag single; flag all on list (both); flag all on show (`Usage`);
  no flag with default (default); no flag no default (`Usage`).
- **MC/DC on list combination:** conditions `hetznerOk`, `digitaloceanOk`, `anyIncomplete`.
  Rows: both ok complete (ok true); one failed (ok false, rows from the other, one error
  naming the failed provider); both failed (ok false, no rows, two errors); both ok one
  incomplete (ok false, all rows, `incomplete-inventory` error).
- **Property:** for any seeded inventory, the text table has one line per row plus a header,
  every column is aligned, and no cell is empty (missing values print `-`); JSON `data`
  round-trips through the `Vm` schema.
- **Examples:** a seeded VM with no labels is listed and shown; two VMs with the same name
  are both listed and `show` by id returns the right one; a VM larger than every default
  limit is listed and shown; `catalog types --within-limits` excludes a type whose bundled
  disk is over the ceiling even when its RAM fits; `show` of a missing id exits 6; the
  DigitalOcean token is never requested when only Hetzner is selected (registry test).
- **Reviewer checks:** `deno task start list --provider hetzner` against no credentials
  prints an `auth` error with the env variable hint and exits 5.

## Hand-off

- d1 implements the production `ProviderRegistry` over real adapters and tokens.
- c2 and c3 reuse `selectProviders` and `registry.forProvider`.

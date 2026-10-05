# vm-maker architecture

vm-maker is a CLI that creates, reads, updates and deletes virtual machines at cloud providers
(Hetzner Cloud and DigitalOcean first). It calls each provider's HTTP API directly instead of
wrapping `hcloud` or `doctl`. That gives us one typed request model, one error model, and the
ability to swap every provider for an in-memory fake in tests.

The tool is built so that **it cannot do something expensive by accident**. Every design decision
below serves one of these invariants.

## Invariants

| # | Invariant | Enforced by |
| --- | --- | --- |
| I1 | **One VM per invocation by default.** Creating more requires `limits.maxCount > 1` in config *and* an explicit `--count`. Both are capped by a compiled-in hard bound. | Policy engine, `Count` branded type |
| I2 | **No unbounded loops.** Every retry, poll and pagination uses a finite `Schedule` (attempt count *and* wall-clock cap). There is no `while (true)`, `Effect.forever`, or unbounded `Effect.repeat` in the codebase. | `Bounded` schedules module, lint rule, review |
| I3 | **Hard upper bounds on resources:** vCPU, memory, disk, TTL, count, and total live vm-maker VMs per provider. | `Limits` schema; policy engine |
| I4 | **Policies never produce an invalid or incompatible config.** A plan is built only from a provider *catalog* (types × regions × images × arch) and is checked again before execution. | Policy engine is total: `Spec → Plan \| PolicyViolation[]` |
| I5 | **Nothing is sent before validation.** All parsing and policy checks happen before the first mutating API call. `--dry-run` prints exactly the requests that would be sent. | Planner/Executor split |
| I6 | **No duplicate creation on retry.** Each create carries a `vm-maker/request-id` label; the executor checks for it before retrying a create. | Executor |
| I7 | **Provider state is the source of truth.** There is no local database to drift. Ownership, TTL and spec hashes live in provider labels/tags. | Label codec |

## Module overview

<!-- draw-visual: diagrams/architecture-module-create.mmd -->
```text
┌──────────────┐  ┌────────────┐  ┌────────────────┐
│   cli [io]   │  │catalog [io]│  │cloudinit [pure]│
└───────┬──────┘  └──────┬─────┘  └────────┬───────┘
        ▼                │                 │
┌──────────────┐         │                 │
│config [pure] │         │                 │
└───────┬──────┘         │                 │
        ▼                │                 │
┌──────────────┐         │                 │
│policy [pure] │◄────────┘        ┌────────┘
└───────┬──────┘                  │
        ▼                         │
┌──────────────┐                  │
│planner [pure]│◄─────────────────┘
└───────┬──────┘
        ▼
┌──────────────┐
│executor [io] │
└──────────────┘
```

<!-- draw-visual: diagrams/architecture-module-reap.mmd -->
```text
┌────────────────────────────┐   ┌──────────────┐   ┌─────────────┐
│        reaper [io]         ├──►│planner [pure]├──►│executor [io]│
└────────────────────────────┘   └──────────────┘   └─────────────┘

┌────────────────────────────┐
│domain [pure], shared by all│
└────────────────────────────┘
```

<!-- draw-visual: diagrams/architecture-module-port.mmd -->
```text
┌────────┐   ┌─────────────┐   ┌─────────────────────┐
│executor├──►│Provider port├──►│   hetzner + labels  │
└────────┘   └──────┬──────┘   └─────────────────────┘
                    │
                    │          ┌─────────────────────┐
                    ├─────────►│digitalocean + labels│
                    │          └─────────────────────┘
                    │
                    │          ┌─────────────────────┐
                    └─────────►│   fake, in-memory   │
                               └─────────────────────┘
```

<!-- draw-visual: diagrams/architecture-module-http.mmd -->
```text
┌─────────────────────┐             ┌───────────┐
│   hetzner + labels  ├─HttpClient─►│Hetzner API│
└─────────────────────┘             └───────────┘

┌─────────────────────┐             ┌───────────┐
│digitalocean + labels├─HttpClient─►│   DO API  │
└─────────────────────┘             └───────────┘
```

The code is organised as *ports and adapters*. Everything left of the `Provider` port is pure or
depends only on Effect services, so it can be tested without a network.

| Module | Responsibility | Pure? |
| --- | --- | --- |
| `cli/` | Parse argv with `effect/cli`, render output (human table or `--output json`), map errors to exit codes. | no (I/O edge) |
| `config/` | Load `vm-maker.config.{yaml,json}` + env vars, decode with Schema, merge with defaults, reject any value above a hard cap (exit 2). | yes after read |
| `domain/` | Branded types and schemas: `VmSpec`, `ResourceSpec`, `Ttl`, `Count`, `Region`, `Limits`, `Plan`, error ADTs. | yes |
| `catalog/` | Fetches and caches each provider's server types, locations, images and prices, normalised to one `Catalog` shape. | port + adapters |
| `policy/` | `resolve(spec, catalog, limits, liveCount) → Either<PolicyViolation[], ResolvedSpec>`. Picks the smallest compatible server type for a resource spec. | **yes** |
| `cloudinit/` | Render templates into `#cloud-config` user-data, validate shape and provider size limits, inject vm-maker metadata. | yes |
| `planner/` | Turn a `ResolvedSpec` into a `Plan`: an ordered, finite list of `ProviderRequest`s. | yes |
| `executor/` | Run a plan against the `Provider` port with bounded retry/poll, idempotency checks and structured logs. | effectful |
| `providers/` | `hetzner/`, `digitalocean/`, `fake/`. Each implements `Provider` and `CatalogSource` over `effect/http` `HttpClient`. | adapters |
| `reaper/` | `reap`: list vm-maker-owned VMs, select those past `expires-at`, plan bounded deletes. | planner is pure |
| `labels/` | Encode/decode vm-maker metadata to provider labels (Hetzner) and tags (DigitalOcean, which only has flat tags). | yes |

## Effect service graph

Each boundary is an Effect service (`Context.Tag`) provided by a `Layer`. The production and test
programs differ only in which layers they provide:

<!-- draw-visual: diagrams/architecture-layers-requires.mmd -->
```text
┌───────────────────────┐
│createVm(args) requires├──┬──────┐
└───────────┬───────────┘  └──────┼─────────────┬────────┐
            ▼                     ▼             ▼        ▼
┌───────────────────────┐  ┌─────────────┐  ┌──────┐  ┌─────┐
│        Provider       │  │CatalogSource│  │Config│  │Clock│
└───────────────────────┘  └─────────────┘  └──────┘  └─────┘
```

<!-- draw-visual: diagrams/architecture-layers-prod.mmd -->
```text
┌───────────────────────┐
│    production Layer   ├───────────────┐
└───────────┬───────────┘               │
            ▼                           ▼
┌───────────────────────┐   ┌───────────────────────┐
│      HetznerLive      │   │    DigitalOceanLive   │
└───────────┬───────────┘   └───────────┬───────────┘
            ▼                           ▼
┌───────────────────────┐   ┌───────────────────────┐
│live HttpClient + Clock│   │live HttpClient + Clock│
└───────────────────────┘   └───────────────────────┘
```

<!-- draw-visual: diagrams/architecture-layers-test.mmd -->
```text
┌─────────────────────┐
│      test Layer     │
└──────────┬──────────┘
           │
           ├───────────────────────┐
           │                       │
         unit                  contract
           │                       │
           ▼                       ▼
┌─────────────────────┐   ┌─────────────────┐
│   FakeProviderTest  │   │ RecordedHttpTest│
└──────────┬──────────┘   └────────┬────────┘
           ▼                       ▼
┌─────────────────────┐   ┌─────────────────┐
│in-memory + TestClock│   │HttpClient replay│
└─────────────────────┘   └─────────────────┘
```

```ts
// The same program runs everywhere; only the Layer differs.
const program = createVm(args) // Effect<VmCreated, CreateError, Provider | Catalog | Config | Clock>

program.pipe(Effect.provide(HetznerLive))      // production
program.pipe(Effect.provide(FakeProviderTest)) // unit + property tests
program.pipe(Effect.provide(RecordedHttpTest)) // contract tests against recorded API fixtures
```

`Clock` comes from Effect's `TestClock` in tests, so TTL expiry and polling timeouts are
deterministic and instant.

## Creating a VM

<!-- draw-visual: diagrams/architecture-create-validate.mmd -->
```text
┌──────┐     ┌─────┐     ┌────────┐     ┌─────────┐     ┌────────┐
│ User │     │ CLI │     │ Config │     │ Catalog │     │ Policy │
└───┬──┘     └──┬──┘     └────┬───┘     └────┬────┘     └────┬───┘
    │           │             │              │               │
    │ 1. create │             │              │               │
    ├──────────►│             │              │               │
    │           │             │              │               │
    │           │ 2. decode   │              │               │
    │           ├────────────►│              │               │
    │           │             │              │               │
    │           │ 3. VmSpec   │              │               │
    │           │◄┈┈┈┈┈┈┈┈┈┈┈┈┤              │               │
    │           │             │              │               │
    │           │ 4. fetch    │              │               │
    │           ├───────────────────────────►│               │
    │           │             │              │               │
    │           │             ┌──────────────────────────────┐
    │           │             │ GET types, locations, images │
    │           │             └──────────────────────────────┘
    │           │             │              │               │
    │           │             │      ┌────────────────┐      │
    │           │             │      │ count live VMs │      │
    │           │             │      └────────────────┘      │
    │           │             │              │               │
    │           │ 5. Catalog, live count     │               │
    │           │◄┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┤               │
    │           │             │              │               │
    │           │ 6. resolve  │              │               │
    │           ├───────────────────────────────────────────►│
    │           │             │              │               │
    │           │             │              │            ┌──────┐
    │           │             │              │            │ pure │
    │           │             │              │            └──────┘
  ┌─[alt violations]───────────────────────────────────────────┐
  │ │           │             │              │               │ │
  │ │           │ 7. PolicyViolation[]       │               │ │
  │ │           │◄┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┤ │
  │ │           │             │              │               │ │
  │ │ 8. exit 3 │             │              │               │ │
  │ │◄┈┈┈┈┈┈┈┈┈┈┤             │              │               │ │
  ├┈[ok]┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┤
  │ │           │             │              │               │ │
  │ │           │ 9. ResolvedSpec            │               │ │
  │ │           │◄┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┤ │
  │ │           │             │              │               │ │
  └────────────────────────────────────────────────────────────┘
    │           │             │              │               │
```

<!-- draw-visual: diagrams/architecture-create-plan.mmd -->
```text
┌──────┐     ┌─────┐     ┌─────────┐
│ User │     │ CLI │     │ Planner │
└───┬──┘     └──┬──┘     └────┬────┘
    │           │             │
    │           │ 1. ResolvedSpec
    │           ├────────────►│
    │           │             │
    │           │ 2. Plan     │
    │           │◄┈┈┈┈┈┈┈┈┈┈┈┈┤
  ┌─[opt dry-run]───────────────┐
  │ │           │             │ │
  │ │ 3. print plan, exit 0   │ │
  │ │◄┈┈┈┈┈┈┈┈┈┈┤             │ │
  │ │           │             │ │
  └─────────────────────────────┘
    │           │             │
```

<!-- draw-visual: diagrams/architecture-create-execute.mmd -->
```text
┌──────┐     ┌─────┐     ┌──────────┐     ┌──────────┐
│ User │     │ CLI │     │ Executor │     │ Provider │
└───┬──┘     └──┬──┘     └─────┬────┘     └─────┬────┘
    │           │              │                │
    │           │ 1. run Plan  │                │
    │           ├─────────────►│                │
    │           │              │                │
    │           │              │ 2. request-id? │
    │           │              ├───────────────►│
    │           │              │                │
    │           │             ┌──────────────────┐
    │           │             │  skip if found   │
    │           │             └──────────────────┘
    │           │              │                │
    │           │              │ 3. POST create │
    │           │              ├───────────────►│
    │           │              │                │
    │           │             ┌───────────────────┐
    │           │             │ labels, user-data │
    │           │             └───────────────────┘
    │           │            ┌─[loop every 2s, max 3m]─┐
    │           │            │ │                │      │
    │           │            │ │ 4. GET status  │      │
    │           │            │ ├───────────────►│      │
    │           │            │ │                │      │
    │           │            └─────────────────────────┘
  ┌─[alt running]─────────────────────────────────┐
  │ │           │              │                │ │
  │ │           │ 5. VmCreated │                │ │
  │ │           │◄┈┈┈┈┈┈┈┈┈┈┈┈┈┤                │ │
  │ │           │              │                │ │
  │ │ 6. VM id, IP             │                │ │
  │ │◄┈┈┈┈┈┈┈┈┈┈┤              │                │ │
  ├┈[timeout]┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┤
  │ │           │              │                │ │
  │ │           │ 7. ProvisionTimeout           │ │
  │ │           │◄┈┈┈┈┈┈┈┈┈┈┈┈┈┤                │ │
  │ │           │              │                │ │
  │ │ 8. VM id, exit 9         │                │ │
  │ │◄┈┈┈┈┈┈┈┈┈┈┤              │                │ │
  │ │           │              │                │ │
  └───────────────────────────────────────────────┘
    │           │              │                │
```

1. **Parse.** argv and config are decoded into a `VmSpec`. A malformed input fails with a
   `DecodeError` naming the field. Nothing has touched the network yet.
2. **Catalog.** The provider catalog is fetched (read-only, cached briefly) along with the count
   of live vm-maker VMs.
3. **Policy.** `policy.resolve` is a pure function that returns every violation, not just the
   first, so the user fixes everything in one pass.
4. **Plan.** The planner emits a finite `Plan`. `--dry-run` stops here and prints it.
5. **Execute.** The executor sends the create request, then polls status on a bounded schedule
   (for example, every 2 s for at most 3 min). If the deadline passes, it fails with
   `ProvisionTimeout` and prints the VM id. It never deletes or retries on its own.

### Policy pipeline

<!-- draw-visual: diagrams/architecture-policy-pipeline.mmd -->
```text
┌──────────────────────────────────┐
│            raw input             │
└─────────────────┬────────────────┘
                  ▼
┌──────────────────────────────────┐
│     decode to branded types      │
└─────────────────┬────────────────┘
                  │
                  ├──────────────────────────────────────┐
                  │                                      │
                  │                                    fail
                  │                                      │
                  ▼                                      ▼
┌──────────────────────────────────┐            ┌─────────────────┐
│ limits: request <= config <= cap │            │   DecodeError   │
└─────────────────┬────────────────┘            └─────────────────┘
                  ▼
┌──────────────────────────────────┐
│     count check vs live VMs      │
└─────────────────┬────────────────┘
                  │
                  ├──────────────────────────────────────┐
                  │                                      │
                  │                                    fail
                  │                                      │
                  ▼                                      ▼
┌──────────────────────────────────┐            ┌─────────────────┐
│catalog: type, region, image, arch│            │  LimitExceeded  │
└─────────────────┬────────────────┘            └─────────────────┘
                  │
                  ├──────────────────────────────────────┐
                  │                                      │
                  │                                  violation
                  │                                      │
                  ▼                                      ▼
┌──────────────────────────────────┐            ┌─────────────────┐
│  select cheapest, ties by name   │            │PolicyViolation[]│
└─────────────────┬────────────────┘            └─────────────────┘
                  ▼                                      ▲
┌──────────────────────────────────┐                     │
│     cloud-init: schema, size     ├──────violation──────┘
└─────────────────┬────────────────┘
                  ▼
┌──────────────────────────────────┐
│           ResolvedSpec           │
└──────────────────────────────────┘
```

The policy engine is the core of invariants I1, I3 and I4. Its rules:

- **Parse, don't validate.** Values become branded types (`VCpu`, `MemoryGiB`, `Ttl`, `Count`) only
  through smart constructors that enforce bounds, so an out-of-range value cannot be represented
  past the edge.
- **Catalog-driven compatibility.** "Valid" means *a tuple that exists in the catalog*:
  `(serverType, location, image)` with matching architecture (x86 vs arm) and availability, not
  "each field looks plausible".
- **Deterministic selection.** A resource spec (`--cpu 2 --mem 4`) resolves to the cheapest type
  satisfying all minimums. Ties break on name. The same inputs always produce the same plan.
- **Limits are layered.** hard cap ≥ config limit ≥ request. Config may lower a cap but never
  raise it: a config value above a hard cap fails config loading (exit 2) rather than being
  silently clamped. A request above the config limit fails with `LimitExceeded` (exit 4).
- **Total.** For every input the engine returns exactly one of `ResolvedSpec` or a non-empty
  `PolicyViolation[]`. It never throws. This is stated as a fast-check property.

Hard caps are compiled in. The first eight rows match [cli.md §0.2](cli.md#02-hard-caps-compiled-in-config-may-lower-them-never-raise-them),
which is the user-facing contract; the last two are internal execution bounds.

| Bound | Default config value | Hard cap |
| --- | --- | --- |
| VMs per invocation (`count`) | 1 | 5 |
| vCPU per VM | 8 | 32 |
| Memory per VM | 16 GB | 128 GB |
| Disk per VM | 160 GB | 1000 GB |
| TTL | 7d max (default TTL 8h) | 30d |
| Active vm-maker VMs per provider | 5 | 20 |
| Deletes per `reap` run | 10 | 20 |
| HTTP retries per request | 2 | 3 |
| Poll duration (`--wait`) | 3 min | 10 min |
| Pages read per list call | 10 | 20 |

## VM lifecycle and TTL

<!-- draw-visual: diagrams/architecture-lifecycle-create.mmd -->
```text
┌────────────────────────┐
│        Planned         │
└────────────┬───────────┘
             ▼
┌────────────────────────┐
│        Creating        │
└────────────┬───────────┘
             │
             ├───────────────────────────┐
             │                           │
             │                       deadline
             │                           │
             ▼                           ▼
┌────────────────────────┐          ┌────────┐
│Running, has expires-at │◄────┐    │TimedOut│
└────────────┬───────────┘     │    └────────┘
             │                 │
          extend            bounded
             ▼                 │
┌────────────────────────┐     │
│Extended, new expires-at├─────┘
└────────────────────────┘
```

<!-- draw-visual: diagrams/architecture-lifecycle-expire.mmd -->
```text
┌────────────────────────┐
│        Running         ├────────delete──────────┐
└────────────┬───────────┘                        │
             │                                    │
     now > expires-at                             │
             ▼                                    ▼
┌────────────────────────┐         ┌────────────────────────────┐
│        Expired         ├──reap──►│Deleted, reap caps N per run│
└────────────┬───────────┘         └────────────────────────────┘
             │                                    ▲
   optional in-VM timer                           │
             ▼                                    │
┌────────────────────────┐                        │
│PoweredOff, still billed├─────────reap───────────┘
└────────────────────────┘
```

The TTL is stored at creation as an absolute expiry in UTC epoch seconds: the label
`vm-maker/expires-at=<epoch>` on Hetzner, and the tag `vm-maker:expires-at:<epoch>` on DigitalOcean,
whose tags cannot contain `/` (see [cli.md §0.3](cli.md#03-ownership-and-ttl-labels)). vm-maker never
touches a VM without its `managed` label or tag.
Enforcement uses two mechanisms, neither of which loops:

1. **`vm-maker reap`** lists owned VMs, selects the expired ones, and deletes at most
   `limits.reapMax` of them per run. It is meant to run from cron or a systemd timer, so the
   scheduling loop lives outside the tool. `reap --dry-run` lists what would be deleted.
2. **In-VM fallback (optional).** cloud-init installs a systemd timer that powers the VM off at
   `expires-at`. On both providers a powered-off VM **is still billed**, so this guards against a
   forgotten workload, not cost. Deletion stays with `reap` so that no provider token ever lives
   on the VM.

`vm-maker extend <id> --ttl 2h` rewrites the label, bounded by the TTL ceiling measured from now.

## cloud-init

User-data is a first-class part of the spec, not an afterthought:

- **Sources:** `--cloud-init file.yaml` or a named template (`--template docker-host`) from
  `templates/`. Templates are typed functions `(vars) => CloudConfig`, not string interpolation.
- **Validation before send:** the result must start with `#cloud-config`, decode against a
  `CloudConfig` schema covering the subset we support (users, ssh keys, packages, write_files,
  runcmd), and fit the provider's size limit (Hetzner 32 KiB, DigitalOcean 64 KiB).
- **Injected metadata:** vm-maker appends its own `write_files` entry (`/etc/vm-maker.json` with
  the request id, spec hash and expiry) and, if requested, the TTL power-off timer.
- **Readiness:** `create --wait-ready` polls (bounded) for the VM to report that cloud-init
  finished, over SSH with `cloud-init status --wait`, or via a phone-home URL later.
- **Secrets:** rendered user-data is kept out of logs and `--dry-run` output unless `--show-secrets`
  is passed. Provider metadata endpoints expose user-data to anything on the VM, so templates
  should fetch secrets at boot rather than embed them.

## Errors

All failures are tagged errors (`Data.TaggedError`) in a closed union, so the CLI can map each one
to a stable exit code and a single-line message, and tests can assert on the tag.

<!-- draw-visual: diagrams/architecture-errors-union.mmd -->
```text
┌───────────┐   ┌────────────────────────┐
│CreateError├──►│  DecodeError, exit 2   │
└─────┬─────┘   └────────────────────────┘
      │         ┌────────────────────────┐
      ├────────►│PolicyViolation, exit 3 │
      │         └────────────────────────┘
      │         ┌────────────────────────┐
      ├────────►│ LimitExceeded, exit 4  │
      │         └────────────────────────┘
      │         ┌────────────────────────┐
      ├────────►│ ProviderError, exit 5  │
      │         └────────────────────────┘
      │         ┌────────────────────────┐
      └────────►│ProvisionTimeout, exit 9│
                └────────────────────────┘
```

<!-- draw-visual: diagrams/architecture-errors-policy.mmd -->
```text
┌───────────────┐   ┌───────────────────────┐
│PolicyViolation├──►│IncompatibleCombination│
└───────┬───────┘   └───────────────────────┘
        │           ┌───────────────────────┐
        ├──────────►│      OutOfBounds      │
        │           └───────────────────────┘
        │           ┌───────────────────────┐
        ├──────────►│     UnknownRegion     │
        │           └───────────────────────┘
        │           ┌───────────────────────┐
        ├──────────►│      UnknownType      │
        │           └───────────────────────┘
        │           ┌───────────────────────┐
        └──────────►│      UnknownImage     │
                    └───────────────────────┘
```

<!-- draw-visual: diagrams/architecture-errors-limit.mmd -->
```text
┌─────────────┐   ┌───────────┐
│LimitExceeded├──►│ CountLimit│
└──────┬──────┘   └───────────┘
       │          ┌───────────┐
       └─────────►│LiveVmQuota│
                  └───────────┘
```

<!-- draw-visual: diagrams/architecture-errors-provider.mmd -->
```text
┌─────────────┐   ┌───────────┐
│ProviderError├──►│    Auth   │
└──────┬──────┘   └───────────┘
       │          ┌───────────┐
       ├─────────►│RateLimited│
       │          └───────────┘
       │          ┌───────────┐
       ├─────────►│  NotFound │
       │          └───────────┘
       │          ┌───────────┐
       └─────────►│ServerError│
                  └───────────┘
```

Exit codes are a public contract shared with [cli.md §0.4](cli.md#04-exit-codes-stable-documented-tested):

| Exit | Name | Error family | Meaning |
| --- | --- | --- | --- |
| 0 | `OK` | — | success, including a dry run that would succeed |
| 1 | `INTERNAL` | defect | a bug; never an expected failure |
| 2 | `USAGE` | `DecodeError` | bad flags, config or cloud-init input, or a config value above a hard cap |
| 3 | `INVALID` | `PolicyViolation` | well formed but not compatible (type × region × image × arch) |
| 4 | `LIMIT` | `LimitExceeded` | count, size, TTL or active-VM cap would be exceeded |
| 5 | `PROVIDER` | `ProviderError` | auth, rate limit or 5xx after bounded retries |
| 6 | `NOT_FOUND` | `ProviderError.NotFound` | the VM does not exist, or is not managed by vm-maker |
| 7 | `ABORTED` | `Aborted` | confirmation declined, or needed in a non-TTY without `--yes` |
| 8 | `DRIFT` | `PlanDrift` | a saved plan no longer matches the catalog or inventory |
| 9 | `TIMEOUT` | `ProvisionTimeout` | the VM exists but did not become ready before the `--wait` deadline |

## Testing strategy

Testing is a first-class property of the design, not a later phase. The architecture already does
most of the work: a pure core, service ports, `TestClock`, and a fake provider.

<!-- draw-visual: diagrams/architecture-testing-fast.mmd -->
```text
┌───────────┐   ┌─────────────────┐   ┌───────────────────┐
│default run├──►│  most: property ├──►│pure core, no Layer│
└─────┬─────┘   └─────────────────┘   └───────────────────┘
      │         ┌─────────────────┐   ┌───────────────────┐
      ├────────►│many: model-based├──►│ fc.commands + fake│
      │         └─────────────────┘   └───────────────────┘
      │         ┌─────────────────┐   ┌───────────────────┐
      └────────►│       unit      ├──►│    core + fake    │
                └─────────────────┘   └───────────────────┘
```

<!-- draw-visual: diagrams/architecture-testing-slow.mmd -->
```text
┌─────┐   ┌───────────────┐   ┌─────────────────┐
│edges├──►│    contract   ├──►│recorded fixtures│
└──┬──┘   └───────────────┘   └─────────────────┘
   │      ┌───────────────┐   ┌─────────────────┐
   └─────►│few: live smoke├──►│ VM_MAKER_LIVE=1 │
          └───────────────┘   └─────────────────┘
```

| Layer | Tool | What it proves |
| --- | --- | --- |
| **Property tests** (most tests) | fast-check | Policy totality; every `ResolvedSpec` satisfies all bounds and exists in the catalog; selection is deterministic and minimal; label encode/decode round-trips; cloud-init renders stay under size limits. |
| **Model-based tests** | `fc.commands` + fake provider | Random sequences of create/extend/delete/reap never exceed live-VM limits, never leave orphans, and reap removes exactly the expired set. |
| **Unit tests** | `Deno.test` + `@std/assert` | Specific edge cases and regressions, each pinned with the fast-check seed that found it. |
| **Contract tests** | recorded HTTP fixtures | Adapters encode requests and decode real response shapes (including error bodies) correctly. Fixtures are scrubbed of tokens and ids. |
| **Live smoke tests** | opt-in: `VM_MAKER_LIVE=1` | Create the smallest VM with a 10 min TTL, wait for cloud-init, delete. A `finally` reap deletes anything left by a failed run. Never runs in default `deno test`. |

Generators live next to the schemas they produce (`domain/arbitraries.ts`), so each new field gets
an arbitrary in the same change. `deno task test` runs everything except live tests in seconds.

## Proposed layout

```text
src/
  main.ts               # wires layers, runs the CLI
  cli/                  # effect/cli commands, output, exit codes
  config/               # config schema, loader, ceilings
  domain/               # branded types, schemas, errors, arbitraries
  policy/               # pure resolve()
  planner/              # pure plan()
  executor/             # bounded execution
  cloudinit/            # templates, renderer, validator
  labels/               # provider label/tag codec
  reaper/
  providers/
    port.ts             # Provider + CatalogSource services
    hetzner/            # HTTP adapter + response schemas
    digitalocean/
    fake/               # in-memory provider used by tests
templates/              # cloud-init templates
test/fixtures/          # recorded provider responses
```

## Technology decisions

| Concern | Choice | Why |
| --- | --- | --- |
| Runtime | Deno 2 | Built-in TS, test runner, fmt/lint, permissions (`--allow-net=api.hetzner.cloud,api.digitalocean.com`) and `deno compile` to a single binary. |
| Effects, errors, DI | `effect` 4 | Typed errors, `Layer`-based dependency injection, `Schedule` for bounded retries, `TestClock`. |
| CLI parsing | `effect/cli` | Part of the `effect` package since v4; typed options, generated help. |
| HTTP | `effect/http` `HttpClient` (part of `effect` since v4) | Composable retries and timeouts; easy to swap for recorded fixtures. |
| Schemas | **Effect Schema** (recommended) over zod | Already part of `effect`; decodes straight into branded types, and `effect/JsonSchema` can export the config schema for editor completion, which removes the main reason to add zod. Effect 4 no longer bundles fast-check, and its own `Arbitrary` module is marked unstable, so generators are written with fast-check directly next to each schema. |
| Property testing | `fast-check` | Generators, shrinking, model-based testing, reproducible seeds. |

## Open questions

- Should `reap` also run opportunistically at the start of `create` (bounded, opt-in)?
- Provider token storage: env vars only, or also the OS keyring?
- Should SSH keys be uploaded per VM or referenced by name from the provider account?
- Do we need a `snapshot` verb in v1, or only CRUD?

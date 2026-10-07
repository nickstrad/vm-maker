# vm-maker architecture

This document derives the proposed architecture from [the CLI contract](cli.md). The current
implementation is a hello-world scaffold. V1 is a stateless API client for lab VMs on
Hetzner Cloud and DigitalOcean.

## State and lifetime

The provider owns VM state. Each invocation fetches current inventory or the selected VM,
performs one requested operation, and exits. Any VM accessible to the credentials can be
managed regardless of which tool created it. Identity is `(provider, opaque ID)`.

Only API credentials are needed for rediscovery. Optional config contains preferences and
editable creation limits. No inventory, VM IDs, action history, catalog cache, or desired
state is persisted locally. Callers may capture command output, but no later command depends
on it.

V1 has no TTL, expiry metadata, reaper, scheduler, ownership gate, spec hash, or saved-plan
workflow. VMs exist until explicitly deleted through this CLI, another API client, or the UI.

## Command dependencies

| CLI command | Reads | Decision | Mutation |
| --- | --- | --- | --- |
| `list` | Paginated inventory from one or both providers | Normalize/filter results, expose incomplete reads | None |
| `show ID` | Current VM and available resource references | Normalize details | None |
| `create NAME` | Types, regions, images, referenced SSH keys as needed | Resolve one compatible shape within config limits | Create one VM |
| `stop ID` | Current VM | Stable state check and confirmation | Graceful shutdown |
| `start ID` | Current VM | Stable state check | Power on |
| `delete ID` | Current VM and resource references as available | Confirm exact target and deletion scope | Ordinary single-VM delete |
| `catalog ...` | Relevant paginated catalog | Normalize/filter, optionally apply limits | None |
| `config show/check` | Optional config file | Decode and merge defaults | None |
| `version` | Build metadata | Render | None |

Only creation requires sizing policy and catalog compatibility. Other mutations require
current VM state, never a catalog lookup or an ownership label. Config's size ceilings
cannot block discovery or lifecycle actions on larger existing machines.

## Modules

| Module | Responsibility |
| --- | --- |
| `cli/` | Parse commands, provider selection, prompts, stdout envelope, stderr diagnostics, exit codes. |
| `config/` | Read optional TOML, merge defaults, validate finite positive limits, resolve token env names. |
| `domain/` | VM identity, normalized VM/catalog models, command requests/results, typed errors. |
| `policy/` | Pure creation validation and deterministic type selection. |
| `commands/` | Per-command orchestration: live reads, pure decisions, confirmation, mutation, optional wait. |
| `providers/port.ts` | Small typed API for inventory, detail, catalog, creation, lifecycle actions, and action status. |
| `providers/hetzner/` | Direct HTTP adapter and provider response schemas. |
| `providers/digitalocean/` | Direct HTTP adapter and provider response schemas. |
| `providers/fake/` | In-memory implementation with seeded fixtures for tests. |
| `http/` | Authentication, redaction, bounded read retries, pagination, request timeouts. |
| `wait/` | Bounded polling of action status and resulting VM state. |

<!-- draw-visual: diagrams/architecture-modules-commands.mmd -->
```text
┌──────────────────────────┐   ┌──────────────┐   ┌─────────────────┐
│        cli/ [io]         ├──►│commands/ [io]├──►│    wait/ [io]   │
└──────────────────────────┘   └───────┬──────┘   └────────┬────────┘
                                       │                   ▼
┌──────────────────────────┐           │          ┌─────────────────┐
│domain/ [pure] used by all│           ├─────────►│providers/port.ts│
└──────────────────────────┘           │          └─────────────────┘
                                       │
                                       │          ┌─────────────────┐
                                       ├─────────►│   config/ [io]  │
                                       │          └─────────────────┘
                                       │
                                       │          ┌─────────────────┐
                                       └─────────►│  policy/ [pure] │
                                                  └─────────────────┘
```

<!-- draw-visual: diagrams/architecture-modules-providers.mmd -->
```text
┌─────────────────┐       ┌────────────────────────────┐  ┌──────────┐
│providers/port.ts├─impl─►│  providers/hetzner/ [io]   ├─►│http/ [io]│
└────────┬────────┘       └────────────────────────────┘  └──────────┘
         │                                                      ▲
         │                ┌────────────────────────────┐        │
         ├──────────impl─►│providers/digitalocean/ [io]├────────┘
         │                └────────────────────────────┘
         │
         │                ┌────────────────────────────┐
         └──────────impl─►│   providers/fake/ [pure]   │
                          └────────────────────────────┘
```

Plain arrows mean "depends on". The `impl` arrows point from the port to the modules that
implement it.

Creation needs no separate planner and executor. The resolver returns a validated
`ResolvedCreate`; the adapter encodes it once, and that encoded payload is both the redacted
dry-run preview and the request sent in a real invocation. Lifecycle previews describe the
action on the fetched VM. Previews live only in memory.

## Provider port and models

Use Effect services and Layers for the provider, config, HTTP client, confirmation, and
clock. Pure functions operate on decoded values. Production selects one real adapter per
provider; tests substitute a fake provider, fake confirmation, and TestClock.

| Service | Production Layer | Test Layer |
| --- | --- | --- |
| Provider port | Hetzner or DigitalOcean adapter | fake provider |
| HttpClient | live client | unused by the fake |
| Config | optional TOML merged with defaults | same decoder over test values |
| Confirmation | TTY prompt | fake confirmation |
| Clock | live clock | TestClock |

The port offers these operations:

```text
listVms(query) -> Inventory          # rows plus incomplete-read flag
getVm(id) -> Vm
getCatalog(query) -> Catalog
createVm(resolvedCreate) -> MutationReceipt
shutdownVm(id) -> MutationReceipt
powerOnVm(id) -> MutationReceipt
deleteVm(id) -> MutationReceipt
getAction(id) -> ActionStatus        # if provider gives an action ID
```

`Vm` includes provider, opaque ID, name, type, region, normalized status, raw provider status,
RAM, bundled disk, addresses, image/creation time when available, native metadata, and attached
resource references when available. Normalize status to `running`, `off`, `transitioning`,
or `unknown`; retain the raw status to avoid losing provider distinctions.

`MutationReceipt` includes provider, VM ID when known, action ID when available, and an
accepted/completed indication. Polling checks the action and the resulting VM condition;
deletion completes when the VM is confirmed absent. A receipt is command output, not local
state. A subsequent `show` reads the provider afresh.

Adapters retain native labels/tags as metadata. They do not require managed labels, infer
identity from names, or automatically add metadata. The ordinary VM deletion endpoint keeps
each provider's own associated-resource behavior; recursively deleting independent resources
is outside the port. Show resource references so users can inspect anything that remains.

Provider API references: [Hetzner Cloud](https://docs.hetzner.cloud/reference/cloud) and
[DigitalOcean Droplets](https://docs.digitalocean.com/products/droplets/reference/api/droplets/).

## Creation policy

Default configuration:

| Setting | Default | Meaning |
| --- | ---: | --- |
| `maxVcpu` | 8 | Maximum selected type's vCPU count. |
| `maxMemoryGb` | 8 | Maximum selected type's plan RAM. |
| `maxDiskGb` | 128 | Maximum selected type's bundled root storage. |

These are positive finite config values, raised by editing config rather than rebuilding.
There are no compiled-in RAM or disk caps.

Creation follows this sequence:

1. Decode flags and config; reject malformed input and requested minimums above limits.
2. Fetch the current catalog and resolve provider defaults for region, image, and SSH keys.
3. Filter types to those available in the region and compatible with the image architecture.
   An explicit type must be in that set. For resource minimums, pick the smallest candidate
   that meets them, ordered by RAM, vCPU, disk, then type ID.
4. Validate the selected type's RAM, vCPU, and bundled disk against ceilings. Disk is checked
   even when no storage minimum was supplied. Never attach a volume to satisfy a storage
   minimum.
5. Read optional user-data, validate provider byte limits, and encode one creation payload.
6. For dry-run, return the redacted preview. Otherwise submit once and return the receipt;
   optionally wait for provider completion within the deadline.

<!-- draw-visual: diagrams/architecture-create-flow.mmd -->
```text
  ┌─────┐     ┌──────────┐     ┌────────┐     ┌──────┐     ┌──────┐
  │ cli │     │ commands │     │ policy │     │ port │     │ wait │
  └──┬──┘     └─────┬────┘     └────┬───┘     └───┬──┘     └───┬──┘
     │              │               │             │            │
     │ 1. decoded request           │             │            │
     ├─────────────►│               │             │            │
     │              │               │             │            │
    ┌────────────────┐              │             │            │
    │ flags + config │              │             │            │
    └────────────────┘              │             │            │
     │              │               │             │            │
     │              │ 2. getCatalog │             │            │
     │              ├────────────────────────────►│            │
     │              │               │             │            │
     │              │ 3. catalog, defaults        │            │
     │              │◄┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┤            │
     │              │               │             │            │
     │              │ 4. resolve type             │            │
     │              ├──────────────►│             │            │
     │              │               │             │            │
     │              │ 5. smallest fit             │            │
     │              │◄┈┈┈┈┈┈┈┈┈┈┈┈┈┈┤             │            │
     │              │               │             │            │
     │              │ 6. check ceilings           │            │
     │              ├──────────────►│             │            │
     │              │               │             │            │
     │              │ 7. ResolvedCreate           │            │
     │              │◄┈┈┈┈┈┈┈┈┈┈┈┈┈┈┤             │            │
     │              │               │             │            │
     │             ┌───────────────────────────────┐           │
     │             │  adapter encodes one payload  │           │
     │             └───────────────────────────────┘           │
 ┌─[alt dry-run]───────────────────────────────────────────────────┐
 │   │              │               │             │            │   │
 │   │ 8. redacted preview          │             │            │   │
 │   │◄┈┈┈┈┈┈┈┈┈┈┈┈┈┤               │             │            │   │
 │   │              │               │             │            │   │
 │  ┌────────────────┐              │             │            │   │
 │  │  no mutation   │              │             │            │   │
 │  └────────────────┘              │             │            │   │
 ├┈[submit]┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┤
 │   │              │               │             │            │   │
 │   │              │ 9. createVm   │             │            │   │
 │   │              ├────────────────────────────►│            │   │
 │   │              │               │             │            │   │
 │   │              │ 10. MutationReceipt         │            │   │
 │   │              │◄┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┤            │   │
 │   │            ┌─[opt --wait]─────────────────────────────────┐ │
 │   │            │ │               │             │            │ │ │
 │   │            │ │ 11. poll, deadline          │            │ │ │
 │   │            │ ├─────────────────────────────────────────►│ │ │
 │   │            │ │               │             │            │ │ │
 │   │            │ │ 12. completed │             │            │ │ │
 │   │            │ │◄┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┤ │ │
 │   │            │ │               │             │            │ │ │
 │   │            └──────────────────────────────────────────────┘ │
 │   │              │               │             │            │   │
 │   │ 13. receipt  │               │             │            │   │
 │   │◄┈┈┈┈┈┈┈┈┈┈┈┈┈┤               │             │            │   │
 │   │              │               │             │            │   │
 └─────────────────────────────────────────────────────────────────┘
     │              │               │             │            │
```

Unit conventions match [CLI limits](cli.md#limits-and-optional-configuration): normalize
DigitalOcean MiB RAM by dividing by 1024, use Hetzner's GB RAM, and use advertised GB for disk.
Provider fixtures keep raw values. Guest free space is not part of capacity validation.

No account-wide VM count is inferred from inventory. One invocation creates one VM;
concurrent invocations and external scripts are outside any global count guarantee.

## Lifecycle operations

For each explicit `(provider, ID)`, fetch current detail. Missing IDs map to `NotFound`.
Dry-run returns the action preview without prompting or mutating. Stop/delete request
confirmation of provider, ID, name, and action unless `--yes` is present. Re-fetch after a
prompt before sending and refuse if relevant state changed. This reduces stale decisions
without pretending that reads and writes form an atomic transaction.

<!-- draw-visual: diagrams/architecture-lifecycle-mutation.mmd -->
```text
  ┌─────┐     ┌──────────┐     ┌──────────────┐     ┌──────┐
  │ cli │     │ commands │     │ confirmation │     │ port │
  └──┬──┘     └─────┬────┘     └───────┬──────┘     └───┬──┘
     │              │                  │                │
     │ 1. stop or delete ID            │                │
     ├─────────────►│                  │                │
     │              │                  │                │
     │              │ 2. getVm         │                │
     │              ├──────────────────────────────────►│
     │              │                  │                │
     │              │ 3. Vm / NotFound │                │
     │              │◄┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┤
 ┌─[alt dry-run]────────────────────────────────────────────┐
 │   │              │                  │                │   │
 │   │ 4. action preview               │                │   │
 │   │◄┈┈┈┈┈┈┈┈┈┈┈┈┈┤                  │                │   │
 ├┈[act]┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┤
 │   │            ┌─[opt no --yes]────────────────────────┐ │
 │   │            │ │                  │                │ │ │
 │   │            │ │ 5. provider, ID, name, action     │ │ │
 │   │            │ ├─────────────────►│                │ │ │
 │   │            │ │                  │                │ │ │
 │   │            │ │ 6. confirmed     │                │ │ │
 │   │            │ │◄┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┤                │ │ │
 │   │            │ │                  │                │ │ │
 │   │            └───────────────────────────────────────┘ │
 │   │              │                  │                │   │
 │   │              │ 7. getVm again   │                │   │
 │   │              ├──────────────────────────────────►│   │
 │   │              │                  │                │   │
 │   │              │ 8. current Vm    │                │   │
 │   │              │◄┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┤   │
 │ ┌─[alt state changed]──────────────────────────────────┐ │
 │ │ │              │                  │                │ │ │
 │ │ │ 9. Conflict  │                  │                │ │ │
 │ │ │◄┈┈┈┈┈┈┈┈┈┈┈┈┈┤                  │                │ │ │
 │ ├┈[unchanged]┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┤ │
 │ │ │              │                  │                │ │ │
 │ │ │              │ 10. shutdownVm or deleteVm        │ │ │
 │ │ │              ├──────────────────────────────────►│ │ │
 │ │ │              │                  │                │ │ │
 │ │ │              │ 11. MutationReceipt               │ │ │
 │ │ │              │◄┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┤ │ │
 │ │ │              │                  │                │ │ │
 │ │ │ 12. receipt  │                  │                │ │ │
 │ │ │◄┈┈┈┈┈┈┈┈┈┈┈┈┈┤                  │                │ │ │
 │ │ │              │                  │                │ │ │
 │ └──────────────────────────────────────────────────────┘ │
 │   │              │                  │                │   │
 └──────────────────────────────────────────────────────────┘
     │              │                  │                │
```

Start on a running VM and stop on an off VM succeed without a mutation. A transitioning or
unknown power state is refused as a conflict. Stop requests graceful shutdown only; no hidden
fallback to hard power-off. Delete uses exactly one ID and never a tag/name selector.

Stop preserves the disk but not running processes or memory, and VM billing continues on both
providers. Sources: [Hetzner billing FAQ](https://docs.hetzner.com/cloud/billing/faq/) and
[DigitalOcean pricing](https://docs.digitalocean.com/products/droplets/details/pricing/).
Delete releases the VM; independent resources can remain billable according to provider rules.

## HTTP bounds and ambiguous outcomes

Every HTTP request has a 30-second timeout. Read-only requests may retry twice with bounded
backoff inside a 90-second total budget; respect rate-limit hints only within that budget.
Pagination has a 100-page and 3-minute limit per provider and rejects repeated cursors.
Reaching a bound returns an explicit incomplete result. These operational bounds are
centralized constants, independent of the sizing limits in config.

Mutations are sent once: there is no blind retry of a create or lifecycle request. A lost
response may mean the provider accepted the request. Return a typed `OutcomeUnknown` error
(exit 5), retain any known IDs, and direct the user to `list`/`show` before trying again.
Neither provider offers a create idempotency guarantee, and names or labels cannot establish
one, so a second invocation after a lost response can create a second VM.

<!-- draw-visual: diagrams/architecture-outcome-unknown.mmd -->
```text
┌──────────────────────────────────────┐
│          Mutation sent once          │
└───────────────────┬──────────────────┘
                    ▼
┌──────────────────────────────────────┐
│            Response lost             │
└───────────────────┬──────────────────┘
                    ▼
┌──────────────────────────────────────┐
│OutcomeUnknown, exit 5, known IDs kept│
└───────────────────┬──────────────────┘
                    ▼
┌──────────────────────────────────────┐
│        User runs list or show        │
└───────────────────┬──────────────────┘
                    │
                    ├─────────────────────────────────┐
                    │                                 │
                 exists                            absent
                    │                                 │
                    ▼                                 ▼
┌──────────────────────────────────────┐    ┌──────────────────┐
│           Keep existing VM           │    │Retry the mutation│
└──────────────────────────────────────┘    └──────────────────┘
```

`--wait` uses a finite Effect Schedule with a default 3-minute deadline, a configurable
`--timeout` up to 10 minutes, and a finite maximum number of polls. Poll errors consume the
same deadline. Failure or timeout does not trigger deletion, resubmission, or rollback.
An accepted VM remains discoverable through provider inventory even if this process exits.

`list --provider all` performs the two independent reads and reports each outcome. One
failure cannot hide the other provider's results or produce a successful empty inventory.
Credentials are read only for selected providers and never printed.

## Errors and verification

Error variants distinguish usage, incompatibility/state conflict, size limits, authentication,
provider rejection, transport failure, unknown mutation outcome, incomplete inventory,
not-found, aborted confirmation, and wait timeout. The CLI maps them to the stable exit codes
in [cli.md](cli.md#shared-options-and-output). JSON stdout uses the same versioned envelope
for text-equivalent successes, errors, and partial inventory.

<!-- draw-visual: diagrams/architecture-errors-provider.mmd -->
```text
┌──────────────────────┐   ┌───────────────────┐
│Provider error, exit 5├──►│        Auth       │
└───────────┬──────────┘   └───────────────────┘
            │              ┌───────────────────┐
            ├─────────────►│  ProviderRejected │
            │              └───────────────────┘
            │              ┌───────────────────┐
            ├─────────────►│     Transport     │
            │              └───────────────────┘
            │              ┌───────────────────┐
            ├─────────────►│   OutcomeUnknown  │
            │              └───────────────────┘
            │              ┌───────────────────┐
            └─────────────►│IncompleteInventory│
                           └───────────────────┘
```

Planned tests:

- Property tests: every resolved type is compatible, meets all requested minimums, respects
  configured ceilings, and is deterministically selected. Include bundled disk over the cap
  and config ceilings larger than defaults.
- Command tests: seeded unlabelled VMs are visible and actionable; duplicate names cannot
  redirect an ID target; larger VMs can be inspected/stopped/started/deleted; dry-run sends
  zero mutations; one create submits one request.
- Failure tests: confirmation cancellation, state changes during prompts, missing IDs,
  transitional states, lost mutation responses without retries, bounded pagination/polling,
  and partial multi-provider inventory.
- Adapter contract tests: scrubbed real response fixtures verify request encoding, unit
  normalization, native metadata, action receipts, and provider error decoding.
- Opt-in live smoke tests: create one small VM and explicitly delete it in cleanup. No default
  test contacts a cloud provider. Cleanup failures report IDs for manual deletion; no TTL
  or reaper is assumed.

The fake provider is internal test infrastructure, not a public CLI mode, and needs no state
file. Tests seed its inventory directly, including externally created VMs, then exercise the
same command services as production.

## Proposed layout and implementation order

```text
src/
  main.ts
  cli/
  config/
  domain/
  policy/
  commands/
  providers/
    port.ts
    hetzner/
    digitalocean/
    fake/
  http/
  wait/
test/fixtures/
```

Use Deno, TypeScript, Effect services and Schema, and fast-check. Call provider HTTP APIs
directly. Config is TOML. No local storage layer is needed; Deno permissions cover the
selected API hosts, token environment variables, the optional config file, and explicitly
supplied user-data files.

Implement list/show and catalog reads first so any existing lab VM is usable immediately.
Then implement one-ID stop/start/delete with confirmation and bounded waiting. Add create
with configurable sizing policy and user-data. Build both adapters against the same contract
and keep README status aligned with what actually runs. Future resize or configuration-edit
commands can reuse the live-read model without introducing a state database.

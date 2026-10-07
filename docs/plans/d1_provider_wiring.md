# d1 · Production provider wiring

**Wave d** · agent `implementer-sonnet-high` · depends on every c slice · owns
`src/providers/live.ts`, `src/main.ts`, `deno.json` (tasks only)

## What this slice gives us

The real thing: `vm-maker list --provider hetzner` reads a real project when
`HCLOUD_TOKEN` is set, `--provider all` reads both accounts, and every other command runs
against the selected adapter. Tokens are resolved lazily from the env variable names in
config, read only for selected providers, held as `Redacted`, and never printed. Deno
permissions are narrowed to the two API hosts, the token variables, the config file, and
explicit user-data paths.

## Architecture of the slice

<!-- draw-visual: diagrams/d1-wiring.mmd -->
```text
┌───────┐   ┌────────────────────┐
│main.ts├──►│    runCli (b1)     │
└───┬───┘   └────────────────────┘
    │
    │       ┌────────────────────┐
    ├──────►│ProviderRegistryLive│
    │       └────────────────────┘
    │
    │       ┌────────────────────┐
    └──────►│ConfirmationTty (c2)│
            └────────────────────┘
```

<!-- draw-visual: diagrams/d1-registry.mmd -->
```text
┌────────────────────┐   ┌───────────────────────────────┐
│ProviderRegistryLive├──►│  ConfigService tokenEnv (b2)  │
└──────────┬─────────┘   └───────────────────────────────┘
           │
           │             ┌───────────────────────────────┐
           ├────────────►│    Config.redacted from env   │
           │             └───────────────────────────────┘
           │
           │             ┌───────────────────────────────┐
           ├────────────►│   HetznerProvider.layer (c5)  │
           │             └───────────────────────────────┘
           │
           │             ┌───────────────────────────────┐
           └────────────►│DigitalOceanProvider.layer (c6)│
                         └───────────────────────────────┘
```

## Code plan

| File | Contents |
| --- | --- |
| `src/providers/live.ts` | `ProviderRegistryLive` layer implementing c1's `ProviderRegistry`: `forProvider(id)` reads the provider's `tokenEnv` name from `ConfigService`, loads it with Effect `Config.redacted(name)` from the ambient `ConfigProvider`, fails `Auth` with hint `set <NAME>` when missing or empty, and memoizes one adapter layer per provider for the invocation (`Layer.memoize` or a `Ref` of built ports) so `all` builds each once. Production `ConfigProvider` is `ConfigProvider.fromEnv()`; tests use `ConfigProvider.fromMap`. |
| `src/main.ts` | Compose layers: logger (a2), `ConfigLive` from `--config` (b2), `ProviderRegistryLive`, `ConfirmationTty` (c2), live `Clock`; call b1 `runCli`. |
| `deno.json` | `start` and `compile` tasks carry exactly `--allow-net=api.hetzner.cloud,api.digitalocean.com --allow-env=HCLOUD_TOKEN,DIGITALOCEAN_TOKEN,VM_MAKER_LOG_LEVEL,HOME --allow-read`. If config names a different `tokenEnv`, the user widens `--allow-env` at launch; document that in d2. |

Decisions already made; do not reopen:

- No token is read until a command actually asks the registry for that provider.
- Memoization lives per process invocation; nothing is cached across runs.
- Permissions are explicit allow-lists, never `-A`.

## Test plan

- **MC/DC on token resolution:** conditions `envNamePresentInConfig` (always true via
  defaults, so test the override path), `valueSet`, `valueNonEmpty`. Rows: set and
  non-empty gives a port; missing gives `Auth` with the hint naming the variable; empty
  string gives the same `Auth`.
- **Examples with `ConfigProvider.fromMap`:** selecting Hetzner only never reads
  `DIGITALOCEAN_TOKEN` (use a provider map that throws or records on unexpected keys);
  `all` with one token missing yields one port and one `Auth`; the two adapters are built
  once each across two `forProvider` calls.
- **Reviewer checks:** `deno task compile` succeeds and `bin/vm-maker version` runs;
  `bin/vm-maker list --provider hetzner` with no token exits 5 and prints the hint; the
  `--allow-env` list in `deno.json` has no wildcard.

## Hand-off

- d2 runs the live smoke test against this wiring and documents the permission flags.

# b2 · Config

**Wave b** · agent `implementer-sonnet-high` · depends on a1, a2 · owns `src/config/**`

## What this slice gives us

Optional TOML preferences become one validated `EffectiveConfig` value with a recorded source
for every field, exactly as [cli.md](../cli.md#limits-and-optional-configuration) describes:
defaults, `./vm-maker.toml`, `~/.config/vm-maker/config.toml`, or `--config PATH`. Unknown
keys fail, limits must be positive and finite, token environment variable names are
resolved but token values are never read here. `config show` and `config check` in c4 only
render what this slice returns.

## Architecture of the slice

<!-- draw-visual: diagrams/b2-config.mmd -->
```text
┌──────────────────────────┐
│      locate [pure]       │
└─────────────┬────────────┘
              ▼
┌──────────────────────────┐
│        read file         │
└─────────────┬────────────┘
              ▼
┌──────────────────────────┐
│     @std/toml parse      │
└─────────────┬────────────┘
              ▼
┌──────────────────────────┐
│  Schema decode, strict   │
└─────────────┬────────────┘
              ▼
┌──────────────────────────┐
│  merge defaults [pure]   │
└─────────────┬────────────┘
              ▼
┌──────────────────────────┐
│EffectiveConfig + sources ├────────────────┐
└─────────────┬────────────┘                │
              ▼                             ▼
┌──────────────────────────┐   ┌─────────────────────────┐
│showView, checkReport (c4)│   │create prefs, limits (c3)│
└──────────────────────────┘   └─────────────────────────┘
```

Everything after "read file" is pure: decode, merge, validate, and annotate sources.

## Code plan

| File | Contents |
| --- | --- |
| `src/config/schema.ts` | `ConfigFile` Schema mirroring the TOML in cli.md: `defaultProvider?`, `limits? { maxVcpu?, maxMemoryGb?, maxDiskGb? }`, `providers? { hetzner? ProviderPrefs, digitalocean? ProviderPrefs }` with `ProviderPrefs = { tokenEnv?, region?, image?, sshKeys?: string[] }`. Strict: extra keys at any level fail decoding with a `ConfigInvalid` naming the key path. Limits check positive finite. |
| `src/config/defaults.ts` | `DEFAULT_CONFIG`: limits from `domain/limits`, `tokenEnv` `HCLOUD_TOKEN` and `DIGITALOCEAN_TOKEN`, no default provider, region/image/sshKeys unset. |
| `src/config/merge.ts` | Pure `merge(file: ConfigFile \| undefined) -> EffectiveConfig` where every leaf is `{ value, source: default \| file }` and the config carries `path?: string`. |
| `src/config/locate.ts` | Pure `candidatePaths({ explicit?, cwd, home }) -> { path, required }[]`: explicit path (required), else `./vm-maker.toml` then `~/.config/vm-maker/config.toml` (optional). |
| `src/config/load.ts` | `ConfigService` via `Context.Service` exposing `effective: EffectiveConfig`. `ConfigLive = Layer.effect(...)` that takes `{ explicitPath?, cwd, home }`, reads the first existing candidate with `Deno.readTextFile`, parses with `@std/toml`, decodes, merges. Missing explicit path is `ConfigInvalid` with hint; TOML syntax error is `ConfigInvalid` with line info from the parser. `ConfigTest(values)` layer wraps a literal `ConfigFile`. |
| `src/config/check.ts` | Pure `checkReport(effective) -> { ok: true, warnings: string[] }` used by `config check`: warns when a provider has no region or image (create will need flags), never fails on missing tokens since values are not read. |
| `src/config/render.ts` | Pure `showView(effective) -> ConfigShowData` with provider prefs, limits, and sources; token env names included, token values structurally impossible. |
| `src/config/mod.ts` | Barrel. |

Decisions already made; do not reopen:

- Token values are never read in this module. Resolution of env names to `Redacted` values
  happens in d1 using Effect `Config` with a `ConfigProvider`, so tests never need
  `--allow-env`.
- Removed expiry settings (`ttl`, `expiry`, anything under `[reaper]`) fail as unknown keys
  with a hint saying v1 has no expiry.
- Flags override provider prefs in the command layer; this module never sees flags.

## Test plan

- **MC/DC on `candidatePaths`:** conditions `explicitGiven`, `cwdFileExists`,
  `homeFileExists` applied in `load.ts` selection; rows: none (defaults, `path` undefined);
  explicit present (used, required); explicit missing (ConfigInvalid); cwd only; home only;
  both (cwd wins).
- **Property:** for any `Limits` arbitrary with positive finite values, encoding to TOML and
  loading yields the same numbers with `source: file`; any limit ≤ 0, `NaN`, or `Infinity`
  fails as `ConfigInvalid` naming the field.
- **Examples:** the full sample TOML from cli.md decodes; an unknown key at top level, under
  `[limits]`, and under `[providers.hetzner]` each fail with the key path in the message;
  `ttl = "4h"` fails with the expiry hint; `showView` output has no key named `token`.
- **Reviewer checks:** `ConfigLive` tests use a temp directory under the scratchpad with
  `--allow-read=.` only; no test reads `HOME` from the real environment.

## Hand-off

- c3 reads provider region/image/sshKeys and limits from `ConfigService`.
- c4 renders `showView` and `checkReport`.
- d1 resolves `tokenEnv` names to token values.

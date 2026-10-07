# c4 · config show, config check

**Wave c** · agent `implementer-sonnet-high` · depends on b1, b2 · owns
`src/commands/config.ts`, `src/cli/handlers/configShow.ts`,
`src/cli/handlers/configCheck.ts`, `src/cli/render/config.ts`

## What this slice gives us

Offline inspection of effective preferences: `config show` prints every setting with its
source (`default` or the file path), token environment variable names without values, and
`config check` validates a file without credentials and exits 2 on any problem with the key
path in the message. Both work with no config file at all.

## Architecture of the slice

<!-- draw-visual: diagrams/c4-config.mmd -->
```text
┌───────────────────────────┐
│handlers config show, check├─────────────┐
└─────────────┬─────────────┘             │
              ▼                           ▼
┌───────────────────────────┐   ┌───────────────────┐
│      commands/config      │   │render config table│
└─────────────┬─────────────┘   └─────────┬─────────┘
              ▼                           ▼
┌───────────────────────────┐   ┌───────────────────┐
│     ConfigService (b2)    │   │   envelope (b1)   │
└─────────────┬─────────────┘   └───────────────────┘
              ▼
┌───────────────────────────┐
│   showView, checkReport   │
└───────────────────────────┘
```

## Code plan

| File | Contents |
| --- | --- |
| `src/commands/config.ts` | `configShow() -> Effect<ConfigShowData, AppError>` returning b2 `showView`; `configCheck() -> Effect<CheckReport, ConfigInvalid>` returning b2 `checkReport`. Both read `ConfigService`; the layer is built by b1's `runCli` from `--config` so an explicit missing path fails here as `ConfigInvalid`. |
| `src/cli/handlers/configShow.ts`, `configCheck.ts` | Replace stubs. |
| `src/cli/render/config.ts` | Text: a `SETTING VALUE SOURCE` table for `defaultProvider`, limits, and each provider's `tokenEnv`, `region`, `image`, `sshKeys`; `check` prints `ok` plus one `warning:` line per b2 warning. |

Decisions already made; do not reopen:

- `config check` never reads environment variables and never says whether a token is set.
- Unknown keys are errors, not warnings, and the message includes the TOML key path.

## Test plan

- **Examples:** no file gives all sources `default`; the sample TOML from cli.md shows
  `source: ./vm-maker.toml` for every file-provided key and `default` for the rest; `--config
  missing.toml` exits 2 with the path in the message; a file with `[reaper]` exits 2 with
  the expiry hint; `config show --output json` has no key named `token` anywhere (walk the
  JSON).
- **Property:** for any `Limits` arbitrary written to a temp TOML, `config show` reports the
  same numbers with `source: file`.
- **Reviewer checks:** tests create files only under the scratchpad directory and run with
  `--allow-read=.`; `grep -r "Deno.env" src/commands/config.ts src/cli/render/config.ts`
  is empty.

## Hand-off

- d2 documents these commands in the README quick start.

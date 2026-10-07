# Implementation plans

Each file in this folder is one vertical slice of work that a single agent can take from
branch to merged pull request. Source of truth for behavior is [cli.md](../cli.md) and
[architecture.md](../architecture.md); a plan never overrides them. If a plan and those
documents disagree, the documents win and the plan gets fixed.

## Naming and ordering

Plans are named `<letter><number>_<slug>.md`.

- The **letter** is a wave. Every slice in a wave can be worked on at the same time, because
  the slices own disjoint paths and depend only on earlier waves. Wave `b` starts when every
  `a` slice is merged, and so on.
- The **number** is only an identifier inside the wave. `b3` does not depend on `b2`.

| Wave | Slices | What the wave delivers |
| --- | --- | --- |
| a | [a1](a1_domain_contract.md), [a2](a2_observability_tooling.md) | The shared contract: domain models, typed errors, exit codes, provider port, JSONL logging, error trace rendering, test tooling. |
| b | [b1](b1_cli_spine.md), [b2](b2_config.md), [b3](b3_http.md), [b4](b4_fake_provider.md), [b5](b5_policy.md), [b6](b6_wait.md) | Every module in the architecture, each tested in isolation. `vm-maker version` runs end to end. |
| c | [c1](c1_list_show_catalog.md), [c2](c2_lifecycle.md), [c3](c3_create.md), [c4](c4_config_commands.md), [c5](c5_hetzner_adapter.md), [c6](c6_digitalocean_adapter.md) | Every command works against the fake provider; both real adapters pass contract tests on scrubbed fixtures. |
| d | [d1](d1_provider_wiring.md), [d2](d2_live_smoke_and_docs.md) | Real providers wired in, opt-in live smoke test, README and docs aligned with what runs. |

<!-- draw-visual: diagrams/readme-waves.mmd -->
```text
┌──────────────────────────┐
│wave a: contract, logging │
└─────────────┬────────────┘
              ▼
┌──────────────────────────┐
│wave b: modules, cli spine│
└─────────────┬────────────┘
              ▼
┌──────────────────────────┐
│wave c: commands, adapters│
└─────────────┬────────────┘
              ▼
┌──────────────────────────┐
│wave d: wiring, live smoke│
└──────────────────────────┘
```

Agents working a slice follow [AGENTS.md](AGENTS.md) (also reachable as `CLAUDE.md`), which
defines the `/tmp/vm-maker-plans/<wave>/<id>.md` state files used to resume work and to see
what peers in the same wave are doing.

## Plan format

Every plan has the same sections, in this order:

1. **Header line**: wave, agent, dependencies, owned paths.
2. **What this slice gives us**: the capability a user or a later slice gets, in plain words.
3. **Architecture of the slice**: one or two diagrams showing the new modules and every
   existing module they call or are called by. Diagrams are draw-visual embeds whose sources
   live in `docs/plans/diagrams/`.
4. **Code plan**: files to create, the public functions and services with their signatures,
   and the decisions the implementer must not reopen.
5. **Test plan**: which logic gets MC/DC tables, which gets property tests, which fakes are
   used, and the acceptance checks the reviewer runs.
6. **Hand-off**: the seams later waves plug into.

## Agents and review

| Agent | Use it for |
| --- | --- |
| `implementer-opus-high` | Slices with subtle semantics: contracts, policy, retries, polling, adapters, lifecycle. |
| `implementer-sonnet-high` | Slices where the plan already fixes the interfaces: config, fakes, renderers, docs. |
| `reviewer-opus-high` | Optional pre-review of a diff against the plan's acceptance criteria. |

The orchestrator (the session that dispatches the agents) does the final pass on every slice
before merge: it runs `deno task check` and `deno task test`, reads the diff against the
plan's acceptance criteria, and rejects anything that touches paths the slice does not own.

## Merge protocol

1. Branch from `main` as `slice/<id>` (for example `slice/b3`).
2. Touch only the paths the plan lists under **owns**, plus that slice's test files. A slice
   may add an import to `deno.json` only when its plan says so, and must keep the existing
   entries untouched.
3. Keep `deno task check` and `deno task test` green on every commit.
4. Open a pull request titled `<id>: <slug>` with the plan's acceptance checklist in the body.
5. After the orchestrator's final pass the branch merges with a merge commit, never a
   squash, so the slice's commits stay readable. The next wave rebases on `main`.

## Testing conventions

The app has no local state, so most tests are plain unit and property tests over pure
functions, with Effect layers supplying fakes for the few effectful seams.

- **No network, ever, in default tests.** `deno task test` runs without `--allow-net`. A
  real API call fails with a permission error instead of reaching a provider. Credentials are
  never read from the real environment in tests; configuration comes from an in-memory
  `ConfigProvider`.
- **MC/DC on named decisions.** Each plan lists the boolean decisions that get Modified
  Condition/Decision Coverage. For a decision with conditions `c1..cn`, write a table in a
  comment above the tests with `n + 1` rows: a baseline row plus one row per condition where
  flipping only that condition flips the outcome. One `Deno.test` per row, named after the
  row. Decisions not listed in a plan get ordinary example tests. There is no percentage
  coverage gate; `deno task coverage` prints branch coverage as information.
- **Property tests with fast-check** for the logic the plans flag as most important: type
  resolution, ceilings, unit normalization, pagination and retry bounds, envelope round trips.
  Use the shared arbitraries in `test/support/arbitraries.ts`. Keep `fc.assert` defaults
  (100 runs); fast-check prints the failing seed and shrunk counterexample on failure.
- **Fakes through Effect layers.** The provider port, confirmation, clock, HTTP client, and
  config provider all have test layers. Prefer `TestClock` from `effect/testing` to real
  sleeps. Never mock by monkey-patching modules.
- **Fixtures** for adapters are scrubbed real responses under `test/fixtures/<provider>/`.
  IDs, names, IPs, and tokens are replaced with obviously fake values.
- **Don't over-test.** Renderers and wiring get a handful of example tests. Pure decision
  logic gets the MC/DC and property treatment. No test asserts on log line wording.

## Logging and error conventions

These apply to every slice and are implemented by [a1](a1_domain_contract.md) and
[a2](a2_observability_tooling.md).

- **Errors are data.** Every failure is a `Schema.TaggedError` from `src/domain/errors.ts`
  with a stable `code`, a human `message`, and optional `provider`, `vmId`, `actionId`,
  `requestId`, `httpStatus`, and `hint` fields. Modules never throw and never return
  `unknown`; unexpected defects become `Internal` at the CLI boundary.
- **Every step has a span.** Command steps and port calls are wrapped with `Effect.fn("name")`
  or `Effect.withSpan`. The CLI prints the span path with each error so a reader sees where
  it failed, for example `create > catalog.fetch > http.get`. With `--verbose` the full
  `Cause` is pretty-printed to stderr.
- **Logs are JSONL on stderr.** One JSON object per line with `ts`, `level`, `msg`, `span`,
  and any annotations (`command`, `provider`, `vmId`, `actionId`, `requestId`,
  `durationMs`). Default minimum level is `Warn`; `--verbose` lowers it to `Debug`; the
  environment variable `VM_MAKER_LOG_LEVEL` (`trace|debug|info|warn|error|none`) overrides
  both. Tokens, `Authorization` headers, and user-data bytes never appear in logs or errors.
- **Exit codes** come from one pure function `exitCodeFor(error)` and match
  [cli.md](../cli.md#shared-options-and-output).

## Effect 4 notes

The repo pins `effect@4.0.0`. Its API differs from the Effect 3 documentation that most
search results describe. Verify against the installed source before using an API:
`~/.cache/deno/npm/registry.npmjs.org/effect/4.0.0/src/`.

Verified module paths used by the plans:

| Need | Import |
| --- | --- |
| Core | `effect`: `Effect`, `Layer`, `Context.Service`, `Schema`, `Data`, `Result`, `Schedule`, `Logger`, `LogLevel`, `References.MinimumLogLevel`, `Redacted`, `Config`, `ConfigProvider`, `Match`, `Cause`, `Duration` |
| CLI | `effect/cli`: `Command`, `Flag`, `Argument`, `Prompt`, `CliError` |
| HTTP | `effect/http`: `HttpClient`, `FetchHttpClient`, `HttpClientRequest`, `HttpClientResponse`, `HttpClientError` |
| Testing | `effect/testing`: `TestClock`, `TestConsole` |
| TOML | `jsr:@std/toml` (added by a2) |

`Result` replaces `Either`; `Context.Service` replaces `Context.Tag`; platform modules live
inside the `effect` package rather than `@effect/platform`.

## Environment

Deno 2.9.5 is installed at `~/.deno/bin/deno`, which is not on `PATH` in agent shells. Run
`export PATH="$HOME/.deno/bin:$PATH"` first. Dependencies are already cached, so `deno task
test` works offline.

# b1 · CLI spine

**Wave b** · agent `implementer-opus-high` · depends on a1, a2 · owns `src/cli/**`,
`src/main.ts`

## What this slice gives us

A runnable `vm-maker` binary that parses the whole command surface from
[cli.md](../cli.md#command-surface), validates every flag combination, prints the versioned
JSON or text envelope on stdout, renders typed errors with their span path on stderr, and
exits with the documented code. `vm-maker version` works end to end. Every other command
parses and validates, then returns a clearly labelled `Internal("not implemented")` until
wave c fills in its handler file. Wave c slices therefore only edit their own handler file.

## Architecture of the slice

<!-- draw-visual: diagrams/b1-spine.mmd -->
```text
┌────┐  ┌────────────┐  ┌──────────┐  ┌──────┐  ┌──────────────────┐
│argv├─►│parse [pure]├─►│Invocation├─►│runCli├─►│handlers (c wave) │
└────┘  └────────────┘  └──────────┘  └───┬──┘  └──────────────────┘
                                          │
                                          │     ┌──────────────────┐
                                          ├────►│ envelope stdout  │
                                          │     └──────────────────┘
                                          │
                                          │     ┌──────────────────┐
                                          ├────►│report stderr (a2)│
                                          │     └──────────────────┘
                                          │
                                          │     ┌──────────────────┐
                                          └────►│ domain/exit (a1) │
                                                └──────────────────┘
```

Parsing is pure: `argv` becomes an `Invocation` value or a `Usage` error with no I/O.
Execution is the only effectful step and is delegated to one handler per command.

<!-- draw-visual: diagrams/b1-run-sequence.mmd -->
```text
┌──────┐     ┌───────┐     ┌────────┐     ┌─────────┐     ┌────────┐
│ main │     │ parse │     │ runCli │     │ handler │     │ output │
└───┬──┘     └───┬───┘     └────┬───┘     └────┬────┘     └────┬───┘
    │            │              │              │               │
    │ 1. argv    │              │              │               │
    ├───────────►│              │              │               │
    │            │              │              │               │
    │            │ 2. Invocation or Usage      │               │
    │            ├┈┈┈┈┈┈┈┈┈┈┈┈┈►│              │               │
    │            │              │              │               │
    │            │              │ 3. run(invocation)           │
    │            │              ├─────────────►│               │
    │            │            ┌─[alt success]────────────────────┐
    │            │            │ │              │               │ │
    │            │            │ │ 4. data + renderText         │ │
    │            │            │ │◄┈┈┈┈┈┈┈┈┈┈┈┈┈┤               │ │
    │            │            │ │              │               │ │
    │            │            │ │ 5. envelope, exit 0          │ │
    │            │            │ ├─────────────────────────────►│ │
    │            │            ├┈[AppError]┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┤
    │            │            │ │              │               │ │
    │            │            │ │ 6. typed error               │ │
    │            │            │ │×┈┈┈┈┈┈┈┈┈┈┈┈┈┤               │ │
    │            │            │ │              │               │ │
    │            │            │ │ 7. report, envelope, exit>0  │ │
    │            │            │ ├─────────────────────────────►│ │
    │            │            │ │              │               │ │
    │            │            └──────────────────────────────────┘
    │            │              │              │               │
```

## Code plan

| File | Contents |
| --- | --- |
| `src/cli/invocation.ts` | `Invocation` as a tagged union, one member per command (`Version`, `List`, `Show`, `Create`, `Stop`, `Start`, `Delete`, `CatalogTypes`, `CatalogRegions`, `CatalogImages`, `ConfigShow`, `ConfigCheck`), each carrying its decoded `domain/requests` value plus `Shared = { provider?: ProviderSelector, configPath?, output: text \| json, verbose, dryRun?, yes?, wait?, timeout?: Duration }`. |
| `src/cli/parse.ts` | Build the command tree with `effect/cli` `Command`, `Flag`, `Argument`. Pure `parse(argv) -> Result<Invocation, Usage>`. Post-parse validation rules, each producing a `Usage` with a specific message and `hint`: `--timeout` requires `--wait`; `--timeout` ≤ 10 minutes; `--type` xor (`--vcpu` and `--memory`); `--disk` only with minimums; `--provider all` only on `list`; `--yes` only on stop/delete; `--dry-run` only on create/start/stop/delete; positive integers for `--vcpu`, positive numbers for `--memory`/`--disk`; duration syntax `30s`, `2m`, `1h`. Provider selection falls back to config in c-wave; the parser only records what the flags said. |
| `src/cli/envelope.ts` | `Envelope<T> = { schemaVersion: 1, ok, data: T \| null, errors: ErrorJson[] }`. Pure `successEnvelope(data)`, `failureEnvelope(errors, partialData?)`. JSON printer uses stable key order; text printer delegates to a per-command `TextRenderer<T>` passed by the handler. |
| `src/cli/handlers/<command>.ts` | Exactly these files: `version.ts`, `list.ts`, `show.ts`, `create.ts`, `stop.ts`, `start.ts`, `delete.ts`, `catalogTypes.ts`, `catalogRegions.ts`, `catalogImages.ts`, `configShow.ts`, `configCheck.ts`. Each exports `run(invocation) -> Effect<Outcome, AppError, Requirements>` where `Outcome = { data, renderText: () => string }`. `version.ts` is real: reads build metadata (`deno.json` version and git short SHA injected at compile time or `dev`). All others return `new Internal({ message: "not implemented: list" })`. |
| `src/cli/handlers/mod.ts` | Static table from `Invocation._tag` to handler; no dynamic registration. |
| `src/cli/run.ts` | `runCli(argv, io) -> Effect<ExitCode>`: parse, build the logger layer from a2 with `resolveLevel`, provide the layer, call the handler inside `withCommandSpan`, map success to envelope + exit 0, map `AppError` through `renderErrorReport` and `exitCodeFor`, catch defects into `Internal`. `io` is `{ stdout, stderr, isTty, env }` so tests inject everything. Partial results (c1) use `ok: false` with `data` populated, so the handler can fail with an error that carries `partialData`; support that path now via an `AppError` field `partialData?: unknown`. If a1 did not add it, add it here in `src/cli/partial.ts` as a wrapper error instead of editing `domain/`. |
| `src/main.ts` | `Deno.exitCode = await runCli(Deno.args, liveIo)`. Nothing else. |

Decisions already made; do not reopen:

- Prompts and diagnostics never touch stdout. `--output json` guarantees exactly one JSON
  object on stdout, even on failure.
- A handler never prints; it returns data plus a text renderer.
- Exit code comes from `exitCodeFor`; the spine has no other code table.
- `version` output in text mode is one line `vm-maker <version> (<sha>)`; JSON `data` is
  `{ version, commit }`.

## Test plan

- **MC/DC on parse validation:** decisions `timeoutRequiresWait` (conditions: timeoutGiven,
  waitGiven), `sizingShape` (typeGiven, vcpuGiven, memoryGiven, diskGiven; valid only for
  type-alone or vcpu+memory with optional disk), `providerAllAllowed` (selectorIsAll,
  commandIsList), `timeoutBound` (timeoutGiven, timeoutWithinMax). One test per row with
  the exact `Usage.message` asserted.
- **Property:** for any valid `Invocation` from an arbitrary, rendering it back to argv and
  parsing yields an equal `Invocation` (round trip). Any argv containing an unknown flag
  fails with `Usage` and exit 2.
- **Envelope (examples):** success, failure, and partial shapes serialize with the keys
  `schemaVersion, ok, data, errors` in that order; failure stderr contains `error[code]` and
  an `at:` line; `--verbose` adds the Cause.
- **End to end:** `runCli(["version"])` exits 0 and prints the envelope; `runCli(["list"])`
  exits 1 with `internal` and message mentioning `not implemented`; `runCli(["bogus"])`
  exits 2; a thrown defect inside a handler becomes `internal` with the original message in
  verbose output.
- **Reviewer checks:** `deno task start version` prints the line; `deno task compile`
  produces a binary that does the same.

## Hand-off

- c1, c2, c3, c4 replace the bodies of their handler files and add text renderers; they
  do not touch `parse.ts` except to fix a bug they discover (call it out in the PR).
- d1 provides the production `ProviderRegistry` layer to `runCli` in `main.ts`.

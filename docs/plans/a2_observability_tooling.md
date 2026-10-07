# a2 · Observability and test tooling

**Wave a** · agent `implementer-opus-high` · depends on nothing · owns `src/log/**`,
`src/main.ts`, `src/hello/**` (delete), `deno.json`, `test/support/fc.ts`,
`test/support/layers.ts`

## What this slice gives us

A JSONL logger with levels, token redaction, and an error report renderer that prints the
span path and typed fields for any failure. Plus the project tooling every later slice relies
on: a `deno task test` that cannot reach the network, a coverage task, a live-test task
that is opt-in, and shared fast-check settings. After this merges, any slice can log with
`Effect.log*`, annotate with `Effect.annotateLogs`, and get the agreed stderr format for free.

## Architecture of the slice

`src/log/` depends only on `effect`. The error report renderer is generic over `Cause` and a
small structural `{ _tag, code, message, ...fields }` shape, so it does not import `domain/`
and can merge in parallel with a1.

<!-- draw-visual: diagrams/a2-log-pipeline.mmd -->
```text
┌────────────────────────┐   ┌──────────────────────┐
│Effect.log, annotateLogs│   │   AppError + Cause   │
└────────────┬───────────┘   └───────────┬──────────┘
             ▼                           ▼
┌────────────────────────┐   ┌──────────────────────┐
│      level filter      │ ┌─┤  renderErrorReport   │
└────────────┬───────────┘ │ └───────────┬──────────┘
             ▼             │             ▼
┌────────────────────────┐ │ ┌──────────────────────┐
│         redact         │ │ │envelope errors[] (b1)│
└────────────┬───────────┘ │ └──────────────────────┘
             ▼             │
┌────────────────────────┐ │
│       JSONL line       │ │
└────────────┬───────────┘ │
             ▼─────────────┘
┌────────────────────────┐
│         stderr         │
└────────────────────────┘
```

## Code plan

| File | Contents |
| --- | --- |
| `src/log/level.ts` | `LogLevelName = trace \| debug \| info \| warn \| error \| none`; pure `resolveLevel({ verbose, env }) -> LogLevel`: env `VM_MAKER_LOG_LEVEL` wins when valid, else `--verbose` gives `Debug`, else `Warn`. Invalid env value falls back and emits one warn line. |
| `src/log/redact.ts` | Pure `redactString(s)` replacing anything that looks like a bearer token, `Authorization: ...`, or a provider token shape (Hetzner 64-char hex-like, DigitalOcean `dop_v1_...`) with `[redacted]`; `redactRecord(obj)` applies it to string leaves and drops keys named `token`, `authorization`, `userData`. Deterministic, idempotent. |
| `src/log/jsonl.ts` | `JsonlLogger.layer(options: { level, sink: (line: string) => void })` built on `Logger.make` + `Logger.layer` and `References.MinimumLogLevel`. Each line is `{ ts, level, msg, span: string[], ...annotations }` after `redactRecord`. Production sink writes to stderr via `Deno.stderr.writeSync` with a trailing newline; tests pass an array-collecting sink. |
| `src/log/report.ts` | `renderErrorReport(error, cause, opts: { verbose }) -> { text: string, json: ErrorJson }`. `text` is the stderr block: `error[code]: message`, one indented line with the non-empty id fields, `at: span > path`, and `hint:` when present. With `verbose`, append `Cause.pretty`. `ErrorJson` is `{ code, message, provider?, vmId?, actionId?, requestId?, trace: string[] }`. Span path is read from the Cause's span annotations (`Effect.fn` and `withSpan` put them there); verify the Effect 4 accessor in `Cause.ts`/`Tracer.ts` before coding. |
| `src/log/mod.ts` | Barrel plus `withCommandSpan(name)` helper that combines `Effect.withSpan(name)` and `Effect.annotateLogs({ command: name })`. |
| `src/main.ts` | Replace the hello program with a stub that builds the logger layer from `resolveLevel` and prints `vm-maker: not implemented yet` to stderr with exit 1. b1 replaces the body. |
| `deno.json` | Tasks: `test` = `deno test --allow-read=. --no-prompt` (no net, no env); `test:live` = `deno test --allow-net=api.hetzner.cloud,api.digitalocean.com --allow-env --allow-read=. test/live/`; `coverage` = `deno test --allow-read=. --no-prompt --coverage=coverage && deno coverage coverage --include=src/`; `check` unchanged; `start`/`compile` gain `--allow-net=api.hetzner.cloud,api.digitalocean.com --allow-env --allow-read`. Add import `"@std/toml": "jsr:@std/toml@^1"` and `"@std/cli": "jsr:@std/cli@^1"` now so wave b touches no shared file. Remove the `docs/` exclude only if it blocks nothing. |
| `test/support/fc.ts` | Re-export `fc` and a `property(name, arb, predicate)` helper that wraps `Deno.test` + `fc.assert` with `verbose: true` so seeds print on failure. |
| `test/support/layers.ts` | `collectLogs()` returning `{ layer, lines }` for asserting on structured fields; `withTestClock(effect)` helper over `effect/testing` `TestClock`. |
| `src/hello/**` | Delete, with its tests. |

Decisions already made; do not reopen:

- Logs go to stderr only. stdout is reserved for the output envelope.
- Default level `Warn`, so a normal successful run prints nothing to stderr.
- Redaction is applied in the logger, not at call sites, so a forgotten call site cannot
  leak a token. Call sites still never put tokens or user-data in log arguments.
- `VM_MAKER_LOG_LEVEL` is read once at startup by b1 through Effect `Config`, not via
  `Deno.env` inside `src/log`.

## Test plan

- **MC/DC on `resolveLevel`:** conditions are `envPresent`, `envValid`, `verbose`. Table
  rows: baseline (none) gives `Warn`; `verbose` alone gives `Debug`; `envPresent && envValid`
  overrides `verbose`; `envPresent && !envValid` behaves like absent and logs one warning.
- **Redaction (property):** for any string `s` and token-shaped `t` from an arbitrary,
  `redactString(s + t + s)` contains no `t`; `redactString` is idempotent; strings without
  token shapes are unchanged.
- **JSONL (examples):** every emitted line parses as JSON with the required keys; a message
  below the minimum level is not emitted; annotations and span names appear; a line whose
  annotation holds `Authorization: Bearer x` is redacted.
- **Error report (examples):** text block contains code, message, id fields, and span path
  in that order; `verbose: false` omits the Cause; JSON `trace` equals the span path.
- **Reviewer checks:** `deno task test` passes with no `--allow-net` or `--allow-env`; a test
  that calls `fetch("https://example.com")` fails with a permission error (keep that test as
  a guard named `test/support/no_network_test.ts`); `src/hello` is gone.

## Hand-off

- b1 builds the logger layer with `resolveLevel` and calls `renderErrorReport` to produce
  the stderr block and the JSON envelope `errors[]` entries.
- Every slice wraps its steps with `Effect.fn("<module>.<step>")` so `at:` paths are useful.

---
title: vm-maker stack and Deno location
summary: vm-maker uses Deno 2.9.5 at /root/.deno/bin/deno (not on PATH) with effect, Effect Schema and fast-check.
tags: [vm-maker, deno, effect, fast-check, toolchain]
updated: 2026-10-05
verified: 2026-10-05 — /root/.deno/bin/deno --version on the droplet reported 2.9.5
---

# vm-maker stack and Deno location

Deno is installed at `/root/.deno/bin/deno` and is **not on PATH**, so a bare `deno` fails with
"command not found". Call it by full path or prepend it:

```bash
export PATH=/root/.deno/bin:$PATH
deno --version   # deno 2.9.5
```

## Libraries

| Concern | Choice |
| --- | --- |
| Runtime, tests, fmt/lint, compile | Deno 2 |
| Effects, typed errors, DI, bounded retries | `effect` 4 (`npm:effect@^4.0.0`; resolves 4.0.0 until 4.0.1 clears Deno's 24h minimum-dependency-age policy) |
| Schemas / parsing | Effect Schema (`effect` 4: `Schema.Trim.check(Schema.isNonEmpty(), …)`, `decodeUnknownEffect`, `Result` replaces `Either`). Effect 4 no longer bundles fast-check; tests build generators with fast-check directly |
| Property tests | `fast-check` (`npm:fast-check@^4`) |
| Assertions | `@std/assert` (`jsr:@std/assert@^1`) |
| Planned | `effect/cli` for argv, `effect/http` `HttpClient` for HTTP (merged into the `effect` package in v4) |

`deno.json` sets `strict`, `exactOptionalPropertyTypes` and `noUncheckedIndexedAccess`, and
`fmt.lineWidth` 100.

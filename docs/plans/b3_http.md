# b3 · HTTP core

**Wave b** · builder-deep (fable high / astra high: retry, pagination, and lost-response
semantics are the thing under test) · depends on a1, a2 · owns `src/http/**`

## What this slice gives us

One bounded, redacting HTTP layer that both adapters build on, implementing the
[operational bounds](../architecture.md#http-bounds-and-ambiguous-outcomes): 30-second
request timeout, read-only retries (two, with backoff, inside a 90-second budget, honoring
`Retry-After` only within that budget), mutations sent exactly once with lost responses
surfaced as `OutcomeUnknown`, and a pagination driver that stops at 100 pages, 3 minutes, or
a repeated cursor and reports the read as incomplete. Status codes become typed errors with
`requestId` and `httpStatus` attached. Adapters in wave c only describe requests and decode
bodies.

## Architecture of the slice

<!-- draw-visual: diagrams/b3-http.mmd -->
```text
┌──────────────────────────────────────┐
│           adapter (c5, c6)           ├───────────┐
└───────────────────┬──────────────────┘           │
                    ▼                              ▼
┌──────────────────────────────────────┐   ┌───────────────┐
│ApiClient read / mutate (effect/http) │◄──┤  collectPages │
└───────────────────┬──────────────────┘   └───────┬───────┘
                    ▼                              ▼
┌──────────────────────────────────────┐   ┌───────────────┐
│retryDecision, classifyResponse [pure]│   │nextPage [pure]│
└───────────────────┬──────────────────┘   └───────────────┘
                    ▼
┌──────────────────────────────────────┐
│          domain/errors (a1)          │
└──────────────────────────────────────┘
```

The retry and pagination decisions are pure functions over small records; the layer only
sequences them with `Schedule` and the clock.

<!-- draw-visual: diagrams/b3-read-retry.mmd -->
```text
    ┌─────────┐     ┌───────────┐     ┌──────────────┐
    │ adapter │     │ ApiClient │     │ provider API │
    └────┬────┘     └─────┬─────┘     └───────┬──────┘
         │                │                   │
         │ 1. read(request)                   │
         ├───────────────►│                   │
         │                │                   │
         │                │ 2. GET, 30s timeout
         │                ├──────────────────►│
         │                │                   │
         │                │ 3. 503 or timeout │
         │                │×┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┤
         │                │                   │
       ┌─────────────────────────────────────┐│
       │ retryDecision: attempt 1, in budget ││
       └─────────────────────────────────────┘│
         │                │                   │
         │                │ 4. GET after 1s   │
         │                ├──────────────────►│
         │                │                   │
         │                │ 5. 200 body       │
         │                │◄┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┤
         │                │                   │
         │ 6. decoded value                   │
         │◄┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┤                   │
         │                │                   │
┌──────────────────────────────────────────────────────┐
│ mutate() never retries: lost reply is OutcomeUnknown │
└──────────────────────────────────────────────────────┘
         │                │                   │
```

## Code plan

| File | Contents |
| --- | --- |
| `src/http/classify.ts` | Pure `classifyResponse({ status, requestId?, bodyText }) -> Ok \| AppError`: 401/403 `Auth`; 404 `NotFound` (adapter decides whether it means VM-not-found); 409/422/400 `ProviderRejected` with the provider's message when the body is JSON with `error.message` or `message`; 429 `Transport` flagged `retryable` with `retryAfter`; 5xx `Transport` retryable; others `ProviderRejected`. Never reads headers named `Authorization`. |
| `src/http/retry.ts` | Pure `retryDecision({ kind: read \| mutation, attempt, error, elapsed, retryAfter? }) -> { retry: false } \| { retry: true, delay }`. Reads retry when `attempt < 2 && error.retryable && elapsed + delay <= 90s`; backoff `1s, 3s`, replaced by `retryAfter` when given and still within budget; mutations never retry. |
| `src/http/paginate.ts` | Pure `nextPage(state, page) -> { continue: true, cursor } \| { done } \| { incomplete: reason }` with state `{ pages, seenCursors, startedAt, now }`; limits `MAX_PAGES`, `PAGINATION_BUDGET`, repeated cursor. `collectPages(fetchPage)` drives it in Effect and returns `{ items, incomplete?: IncompleteRead }`. Cursor is an opaque string so both Hetzner page numbers and DigitalOcean `links.pages.next` fit. |
| `src/http/client.ts` | `ApiClient` service via `Context.Service`: `read(request) -> Effect<Decoded, AppError>` and `mutate(request) -> Effect<Decoded, AppError>`, where `request = { method, url, body?, schema, provider }`. Built on `effect/http` `HttpClient` with `Effect.timeout(30s)`, bearer header from a `Redacted<string>` token, `Accept: application/json`, JSON body encoding, `classifyResponse`, Schema decode of the body into `ProviderRejected` on mismatch (with the decode issue in `cause`). `mutate` wraps timeout, connection reset, and malformed-but-2xx responses into `OutcomeUnknown` with any `requestId`; `read` wraps the same into retryable `Transport`. Each call runs in `Effect.fn("http.<method>")` with `annotateLogs({ provider, requestId })`. |
| `src/http/layers.ts` | `ApiClientLive({ provider, baseUrl, token })` over `FetchHttpClient.layer`; `ApiClientTest(handler)` over an `HttpClient` built from a function `(request) => response` so tests script status codes, bodies, delays, and dropped connections without a socket. |
| `src/http/mod.ts` | Barrel. |

Decisions already made; do not reopen:

- The token is a `Redacted<string>`; the only place it is unwrapped is the header builder,
  and the request value logged is the URL and method, never headers.
- `read` versus `mutate` is chosen by the caller, not inferred from the HTTP method, so a
  `POST` used for a read-only search can still retry.
- Pagination never retries a page beyond what `read` already does; a page failure after
  partial progress returns `IncompleteInventory` carrying the rows read so far.
- Elapsed time comes from the Effect `Clock`, so `TestClock` controls it.

## Test plan

- **MC/DC on `retryDecision`:** conditions `isRead`, `attemptBelowMax`, `errorRetryable`,
  `withinBudget`. Five rows: baseline all true (retry with backoff), then each condition
  flipped alone (no retry). Extra rows: `retryAfter` within budget replaces backoff;
  `retryAfter` beyond budget means no retry.
- **MC/DC on `nextPage`:** conditions `hasNextCursor`, `pagesBelowMax`, `withinTime`,
  `cursorUnseen`. Baseline continues; each flip yields `done` (no cursor) or `incomplete`
  with the matching reason.
- **Property:** for any sequence of page responses, `collectPages` returns at most 100
  pages, never visits a cursor twice, and the concatenated items equal the prefix of the
  input it visited; `classifyResponse` maps every status in 100..599 to exactly one outcome
  and never throws for any body string.
- **Layer tests with `ApiClientTest` and `TestClock`:** a read hitting 503 then 200 succeeds
  after one retry and the clock advanced by 1s; a read hitting 503 three times fails
  `Transport` with `attempts: 3`; a mutation hitting a timeout fails `OutcomeUnknown`
  without a second request (assert the handler was called once); 401 yields `Auth` and the
  error text contains no token; a 2xx body that fails schema decoding on a mutation yields
  `OutcomeUnknown`.
- **Reviewer checks:** no test uses `--allow-net`; grep shows `Redacted.value` is called in
  exactly one place in `src/http`.

## Hand-off

- c5 and c6 construct `ApiClientLive` with their base URLs and describe requests with
  schemas; they call `collectPages` for list and catalog endpoints.
- d1 supplies the `Redacted` token from config env names.

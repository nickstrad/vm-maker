# c5 · Hetzner Cloud adapter

**Wave c** · builder-strong (opus high / sol high: real API quirks behind one port) · depends
on b3 · owns `src/providers/hetzner/**`, `test/fixtures/hetzner/**`

## What this slice gives us

A `ProviderPort` implementation over the
[Hetzner Cloud API](https://docs.hetzner.cloud/reference/cloud) that passes contract tests on
scrubbed real responses: inventory and detail with unit normalization and native labels,
catalog of server types, locations, and images with architecture, one encoded create
payload shared by dry-run and submission, graceful shutdown, power on, delete, and action
status. No command code changes; the port is the contract.

## Architecture of the slice

<!-- draw-visual: diagrams/c5-hetzner.mmd -->
```text
┌─────────────────────────────────────┐   ┌─────────────────┐
│          ProviderPort (a1)          │   │ fixtures (tests)│
└──────────────────┬──────────────────┘   └────────┬────────┘
                   ▼                               │
┌─────────────────────────────────────┐            │
│           hetzner adapter           ├────────────┤
└──────────────────┬──────────────────┘            │
                   ▼                               ▼
┌─────────────────────────────────────┐   ┌─────────────────┐
│paths, encodeCreate, normalize [pure]│   │  ApiClient (b3) │
└──────────────────┬──────────────────┘   └────────┬────────┘
                   ▼                               ▼
┌─────────────────────────────────────┐   ┌─────────────────┐
│               schemas               │   │api.hetzner.cloud│
└─────────────────────────────────────┘   └─────────────────┘
```

## Code plan

| File | Contents |
| --- | --- |
| `src/providers/hetzner/schemas.ts` | Effect `Schema` for the response subsets used: `Server` (id, name, status, server_type, datacenter.location.name, public_net.ipv4.ip, public_net.ipv6.ip, image, created, labels, volumes), `ServerType` (id, name, cores, memory, disk, architecture, deprecation, prices[].location for availability), `Location`, `Image` (id, name, architecture, type, status), `Action` (id, status, error), `Error` (code, message), `Meta.pagination` (page, next_page). Unknown fields are allowed. |
| `src/providers/hetzner/normalize.ts` | Pure `toVm(server) -> Vm` (ids as strings, status map: `running` → running, `off` → off, `initializing \| starting \| stopping \| rebuilding \| migrating \| deleting` → transitioning, else unknown; memory GB used directly, disk GB directly; labels into metadata; volumes as attached references), `toVmType`, `toRegion`, `toImage`, `toActionStatus`, `toReceipt`. |
| `src/providers/hetzner/encode.ts` | Pure `encodeCreate(resolved) -> { body: HetznerCreateBody, preview: RedactedPreview }`: `name`, `server_type` name, `location`, `image`, `ssh_keys`, `user_data` string, `start_after_create: true`; preview replaces `user_data` with `{ byteLength, sha256 }`. |
| `src/providers/hetzner/paths.ts` | Pure request builders: `GET /v1/servers?page=&per_page=50`, `GET /v1/servers/{id}`, `GET /v1/server_types`, `GET /v1/locations`, `GET /v1/images?type=system&architecture=`, `POST /v1/servers`, `POST /v1/servers/{id}/actions/shutdown`, `POST /v1/servers/{id}/actions/poweron`, `DELETE /v1/servers/{id}`, `GET /v1/actions/{id}`. Cursor for b3 pagination is the `next_page` number as a string. |
| `src/providers/hetzner/adapter.ts` | `HetznerProvider.layer({ token: Redacted<string> })` built on b3 `ApiClientLive` with base URL `https://api.hetzner.cloud`; `getVm` maps a 404 to `NotFound` with the vm id; `deleteVm` returns the `action` id from the delete response; `getAction` for the receipt's action. Each operation runs in `Effect.fn("hetzner.<op>")` with `annotateLogs({ provider: "hetzner" })`. |
| `src/providers/hetzner/mod.ts` | Barrel. |
| `test/fixtures/hetzner/*.json` | Scrubbed responses: `servers.page1.json`, `servers.page2.json`, `server.json` (with labels and a volume), `server_types.json` (include a deprecated type and an arm type), `locations.json`, `images.json`, `action.running.json`, `action.success.json`, `action.error.json`, `error.unauthorized.json`, `error.uniqueness.json`. |

Decisions already made; do not reopen:

- Type availability by location comes from `prices[].location` (and excludes deprecated
  types when `deprecation` is set and in the past). Document the field used in a comment.
- Hetzner `memory` is already GB; no conversion. `disk` is GB.
- `shutdown` is the graceful action; `poweroff` is never used.
- Delete uses `DELETE /v1/servers/{id}` only; no label or name selectors anywhere.

## Test plan

All through b3's `ApiClientTest` with fixture bodies; no network.

- **Contract tests (examples):** `listVms` across two fixture pages returns both pages'
  servers with `incomplete` empty; `getVm` normalizes units, status, labels, and volume
  references; a 404 on `getVm` is `NotFound` with the id; `getCatalog` yields types with
  `arch` and `availableRegions`, deprecated type excluded, arm type `arch: "arm"`;
  `createVm` sends exactly the body returned by `encodeCreate` and the preview equals the
  body with `user_data` replaced; `shutdownVm`/`powerOnVm`/`deleteVm` hit the documented
  paths and return action ids; `getAction` maps `running`/`success`/`error`; 401 fixture
  yields `Auth`; uniqueness error fixture yields `ProviderRejected` with Hetzner's message.
- **Property:** `toVm` never throws for any `Server` generated from the schema's arbitrary;
  raw status strings outside the known set map to `unknown` and keep `rawStatus`.
- **Reviewer checks:** fixture files contain no real ids, IPs, or tokens (grep for `Bearer`
  and real-looking ids); every port method is implemented; no `fetch` import in this folder.

## Hand-off

- d1 constructs `HetznerProvider.layer` from the resolved token.
- d2 live smoke exercises create, show, stop, start, delete on one small type.

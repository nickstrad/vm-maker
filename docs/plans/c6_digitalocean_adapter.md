# c6 · DigitalOcean adapter

**Wave c** · agent `implementer-opus-high` · depends on b3 · owns
`src/providers/digitalocean/**`, `test/fixtures/digitalocean/**`

## What this slice gives us

A `ProviderPort` implementation over the
[DigitalOcean Droplets API](https://docs.digitalocean.com/products/droplets/reference/api/droplets/)
that passes the same contract tests as Hetzner on scrubbed fixtures: droplet inventory with
MiB-to-GB normalization and tags, sizes, regions, and images with architecture, one encoded
create payload shared by dry-run and submission, graceful shutdown, power on, delete, and
action status via the droplet actions endpoint.

## Architecture of the slice

<!-- draw-visual: diagrams/c6-digitalocean.mmd -->
```text
┌─────────────────────────────────────┐   ┌────────────────────┐
│          ProviderPort (a1)          │   │  fixtures (tests)  │
└──────────────────┬──────────────────┘   └──────────┬─────────┘
                   ▼                                 │
┌─────────────────────────────────────┐              │
│         digitalocean adapter        ├──────────────┤
└──────────────────┬──────────────────┘              │
                   ▼                                 ▼
┌─────────────────────────────────────┐   ┌────────────────────┐
│paths, encodeCreate, normalize [pure]│   │   ApiClient (b3)   │
└──────────────────┬──────────────────┘   └──────────┬─────────┘
                   ▼                                 ▼
┌─────────────────────────────────────┐   ┌────────────────────┐
│               schemas               │   │api.digitalocean.com│
└─────────────────────────────────────┘   └────────────────────┘
```

## Code plan

| File | Contents |
| --- | --- |
| `src/providers/digitalocean/schemas.ts` | Effect `Schema` for the subsets used: `Droplet` (id, name, status, size_slug, region.slug, networks.v4[] with type `public`, networks.v6[], image.slug/name, created_at, tags, volume_ids, memory, disk, vcpus), `Size` (slug, vcpus, memory, disk, regions[], available), `Region` (slug, name, available), `Image` (id, slug, name, distribution, regions[]), `Action` (id, status, type), `Error` (id, message, request_id), `links.pages.next`, `meta.total`. Unknown fields allowed. |
| `src/providers/digitalocean/normalize.ts` | Pure `toVm(droplet) -> Vm` (ids as strings, status map: `active` → running, `off` → off, `new \| archive` → transitioning, else unknown; `memoryGb = gbFromMib(memory)`, `diskGb = disk`; public v4 and first v6; tags into metadata; `volume_ids` as attached references), `toVmType` from `Size` (arch is `x86` for every current DigitalOcean size; document that assumption and keep a single place to change it), `toRegion`, `toImage`, `toActionStatus`, `toReceipt`. |
| `src/providers/digitalocean/encode.ts` | Pure `encodeCreate(resolved) -> { body, preview }`: `name`, `region`, `size` slug, `image` slug or numeric id, `ssh_keys` (ids or fingerprints as given), `user_data`, `tags: []`; preview replaces `user_data` with `{ byteLength, sha256 }`. |
| `src/providers/digitalocean/paths.ts` | Pure request builders: `GET /v2/droplets?per_page=200&page=`, `GET /v2/droplets/{id}`, `GET /v2/sizes?per_page=200`, `GET /v2/regions`, `GET /v2/images?type=distribution&per_page=200`, `POST /v2/droplets`, `POST /v2/droplets/{id}/actions` with `{ type: "shutdown" }` or `{ type: "power_on" }`, `DELETE /v2/droplets/{id}`, `GET /v2/droplets/{id}/actions/{actionId}`. Cursor for b3 pagination is the `links.pages.next` URL string. |
| `src/providers/digitalocean/adapter.ts` | `DigitalOceanProvider.layer({ token })` on b3 `ApiClientLive` with base URL `https://api.digitalocean.com`; `getVm` maps 404 to `NotFound`; `deleteVm` returns a receipt without an action id (DigitalOcean returns 204), so `wait` relies on the `absent` condition; `getAction` needs the droplet id, so the receipt carries both and the adapter keeps a small `actionId -> vmId` encoding (`"<dropletId>:<actionId>"`) that stays inside this folder. Each operation runs in `Effect.fn("digitalocean.<op>")` with `annotateLogs({ provider: "digitalocean" })`. |
| `src/providers/digitalocean/mod.ts` | Barrel. |
| `test/fixtures/digitalocean/*.json` | Scrubbed responses: `droplets.page1.json`, `droplets.page2.json` (with `links.pages.next`), `droplet.json` (tags and a volume), `sizes.json` (include `s-1vcpu-512mb-10gb` for the 0.5 GB case and an unavailable size), `regions.json`, `images.json`, `action.in-progress.json`, `action.completed.json`, `action.errored.json`, `error.unauthorized.json`, `error.unprocessable.json`. |

Decisions already made; do not reopen:

- 512 MiB sizes normalize to `0.5` GB; no rounding to integers anywhere.
- `shutdown` action only; `power_off` is never used.
- Sizes with `available: false` or not listing the region are excluded by the normalizer's
  `availableRegions`, which c1 and b5 filter on.
- The `requestId` from the error body (`request_id`) is attached to every error.

## Test plan

Through b3's `ApiClientTest` with fixture bodies; no network.

- **Contract tests (examples):** two-page inventory follows `links.pages.next` once and
  returns all droplets; `getVm` yields `memoryGb` 0.5 for the 512 MiB fixture, public v4
  only, tags in metadata, volume references; 404 is `NotFound`; `getCatalog` excludes the
  unavailable size and sets `availableRegions` from `regions[]`; `createVm` body equals
  `encodeCreate` and the preview redacts `user_data`; shutdown and power on post the right
  `type`; delete sends `DELETE` and returns a receipt without action id; `getAction` maps
  `in-progress`/`completed`/`errored`; 401 is `Auth`; 422 is `ProviderRejected` with the
  message and `request_id`.
- **Property:** `toVm` never throws for any schema-generated droplet; unknown statuses map
  to `unknown` with `rawStatus` kept; `gbFromMib` of the fixture sizes equals `memory /
  1024` exactly.
- **Reviewer checks:** fixture files scrubbed; every port method implemented; the
  `actionId` encoding never leaks into `Vm` or `Catalog` values (only in receipts).

## Hand-off

- d1 constructs `DigitalOceanProvider.layer` from the resolved token.
- d2 live smoke uses the smallest available size in one region.

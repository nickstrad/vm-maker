# b5 · Creation policy

**Wave b** · builder-strong (opus high / sol high: pure policy whose semantics are the thing
under test) · depends on a1 · owns `src/policy/**`

## What this slice gives us

The pure heart of `create`: given a decoded request, the live catalog, provider defaults,
and the configured limits, it either returns one `ResolvedCreate` or a typed refusal. It
implements steps 1, 3, and 4 of the [creation sequence](../architecture.md#creation-policy):
reject minimums above limits, filter types by region and image architecture, pick the
smallest fit by RAM then vCPU then disk then type id, and validate the selected type's RAM,
vCPU, and bundled disk against ceilings. No I/O, no provider knowledge, no price logic.

## Architecture of the slice

<!-- draw-visual: diagrams/b5-policy.mmd -->
```text
┌───────────────────────────────┐
│request, catalog, prefs, limits│
└───────────────┬───────────────┘
                ▼
┌───────────────────────────────┐
│       minimums vs limits      ├──────────────┐
└───────────────┬───────────────┘              │
                ▼                              ▼
┌───────────────────────────────┐   ┌────────────────────┐
│        resolveDefaults        │   │LimitExceeded exit 4│
└───────────────┬───────────────┘   └────────────────────┘
                ▼
┌───────────────────────────────┐
│        compatibleTypes        ├──────────────┐
└───────────────┬───────────────┘              │
                ▼                              ▼
┌───────────────────────────────┐   ┌────────────────────┐
│    smallestFit or explicit    │   │  Conflict exit 3   │
└───────────────┬───────────────┘   └────────────────────┘
                ▼
┌───────────────────────────────┐
│         type vs limits        │
└───────────────┬───────────────┘
                ▼
┌───────────────────────────────┐
│         ResolvedCreate        │
└───────────────────────────────┘
```

## Code plan

| File | Contents |
| --- | --- |
| `src/policy/defaults.ts` | Pure `resolveDefaults(request, prefs) -> Result<{ region, image, sshKeys }, Usage>`: flags win over config prefs; missing region or image after merge is `Usage` with a hint naming the flag and config key. |
| `src/policy/compatible.ts` | Pure `compatibleTypes(catalog, region, image) -> VmType[]`: type available in region and `type.arch === image.arch`. Pure `requireExplicit(types, typeId) -> Result<VmType, Conflict>` with a message listing up to five compatible ids. |
| `src/policy/select.ts` | Pure `rankTypes` order: `memoryGb`, `vcpu`, `diskGb`, then `id` ascending. Pure `smallestFit(types, minimums) -> Result<VmType, Conflict>` where a candidate meets `vcpu >= min.vcpu`, `memoryGb >= min.memoryGb`, and `diskGb >= min.diskGb` when given. Empty result is `Conflict` listing the minimums and the nearest candidate that failed and why. |
| `src/policy/limits.ts` | Pure `checkMinimumsAgainstLimits(minimums, limits) -> Result<void, LimitExceeded>` (step 1) and `checkTypeAgainstLimits(type, limits) -> Result<void, LimitExceeded>` (step 4), each reporting the first violated dimension with `limit`, `requested`, `maximum`. |
| `src/policy/userData.ts` | Pure `checkUserData(bytes, providerLimitBytes) -> Result<UserDataSummary, Usage>` producing `{ byteLength, sha256 }` and rejecting invalid UTF-8 or bytes above the provider limit (Hetzner 32 KiB, DigitalOcean 64 KiB; constants live here under `USER_DATA_LIMITS`). |
| `src/policy/resolve.ts` | Pure `resolveCreate(input: { request, catalog, prefs, limits }) -> Result<ResolvedCreate, Usage \| Conflict \| LimitExceeded>` composing the above in the documented order. |
| `src/policy/mod.ts` | Barrel. |

Decisions already made; do not reopen:

- Ties are broken by type id string comparison, so selection is total and deterministic.
- Disk ceiling is checked even when `--disk` was not given.
- An explicit `--type` that is not in the compatible set is `Conflict` (exit 3), not
  `NotFound`.
- A minimum above a limit fails before any catalog work (`LimitExceeded`, exit 4), so a
  user sees the config problem even when the catalog fetch would also fail.
- Never pick a type and then "fix" a violation by attaching a volume or relaxing a minimum.

## Test plan

This slice carries the most important logic, so it gets both treatments.

- **MC/DC on compatibility:** decision `regionOk && archOk` plus, for explicit type,
  `inCompatibleSet`. Rows: both true selected; region false; arch false; explicit id absent.
- **MC/DC on fit:** decision `vcpuOk && memoryOk && (diskNotRequested \|\| diskOk)`. Rows:
  baseline fits; vcpu short; memory short; disk requested and short; disk not requested with
  small bundled disk still fits.
- **MC/DC on ceilings:** decision `vcpu <= maxVcpu && memory <= maxMemoryGb && disk <= maxDiskGb`
  for the selected type. Rows: baseline passes; each dimension over alone; plus the two
  cli.md examples: `--memory 16` against default 8, and a type with 8 GB RAM and 160 GB disk
  failing the 128 GB disk ceiling.
- **Properties with fast-check** (catalog and limits arbitraries from `test/support`):
  - any resolved type is in `compatibleTypes` and meets every requested minimum;
  - any resolved type satisfies all three ceilings;
  - no compatible type that meets the minimums ranks lower than the chosen one (true
    minimality under `rankTypes`);
  - `resolveCreate` is deterministic: same input twice gives equal output, and shuffling the
    catalog's type order does not change the result;
  - raising limits never turns a success into a failure, and never changes the selected
    type (monotonicity);
  - with ceilings larger than defaults, a type over the default but under the new ceiling
    resolves.
- **User data:** examples for empty file, exactly at limit, one byte over, invalid UTF-8;
  property that `sha256` is stable and `byteLength` equals input length.
- **Reviewer checks:** `src/policy` imports only `effect` and `src/domain`; every function
  is synchronous and returns `Result`, never `Effect`.

## Hand-off

- c3 calls `resolveCreate` after fetching the catalog and reading config prefs and limits.
- c1 `catalog types --within-limits` reuses `checkTypeAgainstLimits`.

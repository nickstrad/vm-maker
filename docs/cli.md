# vm-maker CLI UX proposals

vm-maker is a Deno + Effect CLI for create/read/update/delete (CRUD) on cloud VMs. It calls provider HTTP APIs
directly, starting with Hetzner Cloud and DigitalOcean. This document proposes three CLI UX designs,
compares them, and recommends a hybrid.

Each design must make these invariants obvious:

| # | Invariant | Short name |
|---|-----------|------------|
| I1 | One VM per invocation by default. More than one needs `limits.maxCount` in config **and** an explicit flag, and both are capped by a compiled-in hard bound. The CLI never loops or retries without a bound. | *bounded count* |
| I2 | Hard upper bounds apply to vCPU, memory, disk, count, TTL and the number of active vm-maker VMs per provider. | *hard caps* |
| I3 | Policies never produce invalid or incompatible configurations, such as a type that is unavailable in the region, an image/arch mismatch or a TTL above the max. Validation runs before any API call, and `plan`/`--dry-run` shows the exact requests. | *valid-or-refuse* |
| I4 | Testing is first-class: `--provider fake`, deterministic dry-run output, `--output json` and stable exit codes. | *testable* |

---

## 0. Shared foundations (all three designs)

The designs differ in surface UX. They share the parts below, which the recommendation also assumes.

### 0.1 Pipeline

Every mutating command goes through the same pure pipeline. Only the last step performs I/O against the
provider, apart from the read-only catalog and inventory fetches in step 3.

```
input (flags | spec file | preset)
  -> decode     (Schema: unknown -> VmRequest)            exit 2 on failure
  -> resolve    (preset + defaults + config -> VmSpec)
  -> validate   (hard caps, config limits, catalog compat) exit 3 / 4
  -> plan       (VmSpec -> Plan: ordered list of HTTP requests, deterministic)
  -> apply      (Plan -> provider API, bounded retries)    exit 5 on API error
```

- `resolve` and `validate` are pure functions of `(request, config, catalog, inventory, now)`. Catalog means
  server types, locations, images and prices. Inventory means the currently active vm-maker VMs. Both are
  fetched once, read-only, and the fake provider serves them from fixtures.
- Property tests (fast-check) target `validate ∘ resolve`. For any generated request, the result is either
  a `VmSpec` that satisfies every cap and compatibility rule or a typed `PolicyError`. It is never both and
  never neither.

### 0.2 Hard caps (compiled in; config may lower them, never raise them)

| Cap | Hard bound | Default config value |
|-----|-----------:|---------------------:|
| `count` per invocation | 5 | 1 |
| vCPU per VM | 32 | 8 |
| memory per VM | 128 GB | 16 GB |
| disk per VM | 1000 GB | 160 GB |
| TTL | 30d | 7d (default TTL 8h) |
| active vm-maker VMs per provider | 20 | 5 |
| deletes per `reap` run | 20 | 10 |
| HTTP retries per request | 3 | 2 |

If a config value exceeds a hard bound, config loading fails (exit 2). The value is never silently clamped.

The active-VM cap is a check-then-act test against the inventory, so two concurrent invocations could both
pass it. The docs should state this plainly as a known limit.

### 0.3 Ownership and TTL labels

Every VM that vm-maker creates is labelled. The label encoding is per provider because Hetzner labels and
DigitalOcean tags have different character rules (DigitalOcean tags cannot contain `/`).

| Meaning | Hetzner label | DigitalOcean tag |
|---------|---------------|------------------|
| managed | `vm-maker/managed=true` | `vm-maker:managed` |
| expiry (UTC epoch seconds) | `vm-maker/expires-at=1791208800` | `vm-maker:expires-at:1791208800` |
| origin (preset/spec name) | `vm-maker/origin=dev-small` | `vm-maker:origin:dev-small` |

vm-maker never lists, updates or deletes a VM without `managed`. `reap` deletes only VMs where
`managed` is set **and** `expires-at < now`.

### 0.4 Exit codes (stable, documented, tested)

| Code | Name | When |
|-----:|------|------|
| 0 | `OK` | success, including a dry-run that would succeed |
| 1 | `INTERNAL` | bug or unexpected defect |
| 2 | `USAGE` | bad flags, unparsable spec or config, or a config value above a hard bound |
| 3 | `INVALID` | policy or compatibility violation (I3) |
| 4 | `LIMIT` | a cap was hit (I1/I2): count, size, TTL or active VMs |
| 5 | `PROVIDER` | provider API error after bounded retries |
| 6 | `NOT_FOUND` | the VM does not exist, or exists but is not managed by vm-maker |
| 7 | `ABORTED` | the user declined a confirmation, or confirmation was needed in a non-TTY without `--yes` |
| 8 | `DRIFT` | a saved plan no longer matches reality (stale plan, see Design 2) |
| 9 | `TIMEOUT` | the VM was created but did not become ready before the `--wait` deadline |

### 0.5 Global flags

```
--provider <hetzner|digitalocean|fake>   default from config
--output <text|json>                     json is a versioned envelope: {"schemaVersion":1, ...}
--dry-run                                validate + plan, never mutate (alias: plan)
--yes                                    skip interactive confirmation (required in non-TTY for destructive ops)
--config <path>                          default: ./vm-maker.toml, then ~/.config/vm-maker/config.toml
--now <RFC3339>                          freeze the clock (tests, reproducible plans); env VM_MAKER_NOW
-v / --verbose                           print the HTTP request summary (secrets redacted)
```

Tokens come only from the environment (`HCLOUD_TOKEN`, `DIGITALOCEAN_TOKEN`), or from an env var named in
config. Config files never hold them. The fake provider needs no token. It persists its state in
`$VM_MAKER_FAKE_STATE` (a JSON file), or in memory when that is unset, and it uses deterministic IDs
(`fake-0001`, ...).

---

## Design 1: Imperative noun-verb (`vm-maker vm create ...`)

### Philosophy

The design follows `gh`, `doctl` and `hcloud`: one command per action, and everything is expressible on the
command line. It is the least surprising option for scripting and the easiest to learn. Safety comes from
validation inside each command, not from a separate workflow.

### Command tree

```
vm-maker
├── vm
│   ├── create   [NAME] (--type T | --vcpu N --memory GB [--disk GB]) --region R --image I
│   │            [--ttl DUR | --no-ttl] [--user-data FILE | --cloud-init TEMPLATE [--var k=v]...]
│   │            [--ssh-key NAME]... [--label k=v]... [--count N] [--dry-run] [--wait]
│   ├── list     [--all-regions] [--expired] [--label k=v]...
│   ├── show     <NAME|ID>
│   ├── update   <NAME|ID> [--rename NEW] [--label k=v]... [--unlabel k]...
│   │            [--extend DUR | --ttl DUR] [--dry-run]
│   ├── resize   <NAME|ID> (--type T | --vcpu N --memory GB) [--grow-disk] [--dry-run] [--yes]
│   ├── delete   <NAME|ID> [--dry-run] [--yes]            # exactly one target, no globs
│   └── reap     [--dry-run] [--yes] [--max N]            # one-shot, deletes expired VMs
├── catalog
│   ├── types    [--region R] [--arch x86|arm]
│   ├── regions
│   └── images   [--arch x86|arm]
├── cloud-init
│   ├── list
│   └── render   <TEMPLATE> [--var k=v]...                 # prints final user-data, no API call
├── config
│   ├── show     [--effective]                             # merged config + hard caps
│   └── check
└── version
```

`delete` takes exactly one target. Bulk deletion goes only through `reap`, which is bounded by
`limits.reapMax`.

### Example sessions

**1. Create by spec with a TTL.** The resolver picks a type, and a dry run shows the requests.

```console
$ vm-maker vm create scratch --vcpu 2 --memory 4 --region fsn1 --image ubuntu-24.04 \
    --ttl 4h --cloud-init docker --dry-run --now 2026-10-05T10:00:00Z
Plan (provider: hetzner, dry run, nothing sent)

  resolve  vcpu>=2 memory>=4GB disk>=0GB @ fsn1, arch x86 (image ubuntu-24.04)
           -> cx22 (2 vCPU, 4 GB, 40 GB disk, €4.59/mo)   [cheapest of 3 candidates]
  ttl      4h -> expires 2026-10-05T14:00:00Z (max 7d)
  count    1

  POST /v1/servers
    name:         scratch
    server_type:  cx22
    location:     fsn1
    image:        ubuntu-24.04
    ssh_keys:     [nick-laptop]
    user_data:    <cloud-init:docker, 1.2 KB, sha256:9f3a…c1>
    labels:
      vm-maker/managed:    "true"
      vm-maker/expires-at: "1791208800"
      vm-maker/origin:     "cli"

$ echo $?
0
```

**2. Create for real, then list.**

```console
$ vm-maker vm create scratch --vcpu 2 --memory 4 --region fsn1 --image ubuntu-24.04 --ttl 4h --wait
Created scratch (id 51234567) cx22 @ fsn1, ipv4 49.12.1.2, expires in 4h0m

$ vm-maker vm list
NAME      ID        PROVIDER  TYPE   REGION  STATUS   IPV4        EXPIRES
scratch   51234567  hetzner   cx22   fsn1    running  49.12.1.2   in 3h58m
old-ci    50999001  hetzner   cpx21  nbg1    running  49.12.9.9   EXPIRED 2h ago
2 VMs (limit 5 active on hetzner)
```

**3. Count without the config option.** I1 makes the refusal explicit and names both missing pieces.

```console
$ vm-maker vm create web --type cx22 --region fsn1 --image ubuntu-24.04 --count 3
error[LIMIT/count]: --count 3 requires limits.maxCount >= 3 in config (current: 1)
  config: ./vm-maker.toml
  hint:   set [limits] maxCount = 3 (hard upper bound: 5), then re-run with --count 3
$ echo $?
4
```

**4. Incompatible configuration.** An ARM type with an x86 image is refused before any API call.

```console
$ vm-maker vm create a1 --type cax11 --region ash --image ubuntu-24.04-x86 --output json
{"schemaVersion":1,"ok":false,"exitCode":3,"errors":[
  {"code":"INVALID/type-region","message":"server type cax11 is not available in ash",
   "available":["fsn1","nbg1","hel1"]},
  {"code":"INVALID/arch-mismatch","message":"image ubuntu-24.04-x86 is x86, type cax11 is arm",
   "suggest":{"image":"ubuntu-24.04-arm"}}]}
```

All errors are collected in one pass (`Effect.validateAll`), so the user fixes everything at once.

**5. Extend a TTL, then reap.**

```console
$ vm-maker vm update scratch --extend 2h
scratch: expires-at 2026-10-05T14:00:00Z -> 2026-10-05T16:00:00Z (max allowed 2026-10-12T10:00:00Z)

$ vm-maker vm reap --dry-run
Would delete 1 expired VM (limit 10 per run):
  old-ci  50999001  hetzner  nbg1  expired 2026-10-05T08:00:00Z (2h ago)

$ vm-maker vm reap --yes
Deleted old-ci (50999001). 1 deleted, 0 failed.
```

### How invariants surface

- **I1:** `--count` > 1 needs both flag and config (session 3). A count > 1 also forces an interactive
  confirmation that echoes the number. Non-TTY runs need `--yes`, otherwise they exit 7.
- **I2:** caps are reported in the `LIMIT/*` error with the current value, the config limit and the hard
  bound. `vm list` footer always shows `n VMs (limit m active)`.
- **I3:** every create, update and resize resolves against the live catalog first. Errors name the
  offending field and offer a `suggest` value.
- **I4:** `--dry-run` output is byte-stable given `--now` and a fixed catalog. The JSON envelope is
  versioned and the exit codes are fixed.

### Config

```toml
# vm-maker.toml
defaultProvider = "hetzner"

[defaults]
region   = "fsn1"
image    = "ubuntu-24.04"
ttl      = "8h"
sshKeys  = ["nick-laptop"]

[limits]                 # may only be <= hard caps; violations fail config load (exit 2)
maxCount     = 1
maxVcpu      = 8
maxMemoryGb  = 16
maxDiskGb    = 160
maxTtl       = "7d"
maxActive    = 5         # per provider
reapMax      = 10
requireTtl   = true      # create without --ttl uses defaults.ttl; --no-ttl rejected

[providers.hetzner]
tokenEnv = "HCLOUD_TOKEN"

[providers.digitalocean]
tokenEnv = "DIGITALOCEAN_TOKEN"
region   = "fra1"        # provider-specific override
image    = "ubuntu-24-04-x64"

[cloudInit]
templateDir = "./cloud-init"   # docker.yaml, devbox.yaml, ...
```

### Pros and cons

| Pros | Cons |
|------|------|
| Familiar, matches `hcloud` and `doctl` muscle memory | Long command lines for realistic VMs |
| Trivial to script and test, one command per assertion | Nothing reviewable or versionable, since intent lives in shell history |
| Smallest implementation | `--dry-run` and the real run are two separate invocations that could differ |
| Errors are local to one command | The flag surface grows with every option |

---

## Design 2: Declarative spec + plan/apply ("terraform-lite")

### Philosophy

You describe the VM in a file, review a plan, then apply exactly that plan. A spec describes **one VM** by
default, and there is no state file: the provider labels are the state. Reviewing the plan is the safety
mechanism. Applying a saved plan guarantees that what you reviewed is what gets sent.

### Command tree

```
vm-maker
├── plan     -f SPEC [-o PLAN.json]              # validate + resolve + diff against live, write plan
├── apply    (PLAN.json | -f SPEC) [--yes]       # PLAN: apply exactly; SPEC: plan+confirm+apply
├── destroy  -f SPEC [--yes] [--dry-run]         # delete VM(s) owned by this spec
├── status   [-f SPEC]                           # live view; without -f, all managed VMs
├── show     <NAME|ID>
├── extend   -f SPEC --by DUR | <NAME> --by DUR  # TTL change without editing spec
├── reap     [--dry-run] [--yes] [--max N]
├── validate -f SPEC                             # offline: schema + hard caps only, no API
└── catalog  types|regions|images
```

### Spec file

```yaml
# specs/scratch.vm.yaml
apiVersion: vm-maker/v1
name: scratch                 # identity: label vm-maker/origin=spec:scratch
provider: hetzner
machine:
  vcpu: 2                     # either resources ...
  memoryGb: 4
  diskGb: 40
  # type: cx22                # ... or an explicit type (mutually exclusive)
region: fsn1
image: ubuntu-24.04
ttl: 4h                       # relative to apply time
cloudInit:
  template: docker            # or: file: ./cloud-init/custom.yaml
  vars: { user: nick }
sshKeys: [nick-laptop]
labels: { project: blog }
# count: 2                    # only honoured if limits.maxCount >= 2 AND apply --allow-count
```

### Example sessions

**1. Plan and save it, then apply the saved plan.**

```console
$ vm-maker plan -f specs/scratch.vm.yaml -o scratch.plan.json --now 2026-10-05T10:00:00Z
scratch.vm.yaml -> hetzner

  + create scratch
      type        cx22  (resolved from vcpu>=2, memory>=4, disk>=40 @ fsn1, x86)
      region      fsn1
      image       ubuntu-24.04
      cloud-init  template docker (sha256:9f3a…c1)
      expires     2026-10-05T14:00:00Z (ttl 4h, max 7d)
      labels      project=blog, vm-maker/managed=true, vm-maker/origin=spec.scratch, …

Plan: 1 to create, 0 to change, 0 to delete.  Active after apply: 3/5 on hetzner.
Saved: scratch.plan.json (sha256:51ab…9e)

$ vm-maker apply scratch.plan.json
Applying plan sha256:51ab…9e (1 action)
  + scratch  created  id 51234567  49.12.1.2
Done: 1 created.
```

**2. Change the spec, and the plan shows an in-place update.**

```console
$ sed -i 's/vcpu: 2/vcpu: 4/; s/memoryGb: 4/memoryGb: 8/' specs/scratch.vm.yaml
$ vm-maker plan -f specs/scratch.vm.yaml
  ~ update scratch (51234567)
      type   cx22 -> cx32           # requires power-off; disk kept at 40GB (no --grow-disk)
Plan: 0 to create, 1 to change, 0 to delete.
! This change powers off the VM for ~1 minute.
```

**3. A stale plan is refused.** Someone deleted the VM after the plan was saved.

```console
$ vm-maker apply scratch.plan.json
error[DRIFT]: plan sha256:51ab…9e was computed against a different state
  expected: scratch (51234567) running cx22
  actual:   scratch not found
  hint:     re-run `vm-maker plan -f specs/scratch.vm.yaml`
$ echo $?
8
```

**4. The spec asks for too much.** Offline validation catches it with no token needed.

```console
$ vm-maker validate -f specs/big.vm.yaml
specs/big.vm.yaml:6:13  error[LIMIT/memory]   memoryGb 256 > limits.maxMemoryGb 16 (hard bound 128)
specs/big.vm.yaml:9:6   error[LIMIT/ttl]      ttl 60d > limits.maxTtl 7d (hard bound 30d)
specs/big.vm.yaml:12:8  error[LIMIT/count]    count 4 requires limits.maxCount >= 4 (current 1)
3 errors.
$ echo $?
4
```

**5. Destroy with JSON output in CI.**

```console
$ vm-maker destroy -f specs/scratch.vm.yaml --yes --output json --provider fake
{"schemaVersion":1,"ok":true,"exitCode":0,
 "actions":[{"op":"delete","name":"scratch","id":"fake-0001","result":"deleted"}]}
```

### How invariants surface

- **I1:** a spec without `count` means 1. `count > 1` needs `limits.maxCount` **and**
  `apply --allow-count N`, where N must equal the planned count. The plan summary always prints
  `n to create`.
- **I2:** `validate` checks caps offline with file:line:col diagnostics. `plan` adds the live check
  (`Active after apply: 3/5`).
- **I3:** the saved plan holds the resolved, fully validated requests. `apply PLAN` re-validates
  against the current catalog and inventory, and any difference gives `DRIFT` (exit 8). It never
  "fixes it up".
- **I4:** a plan is a pure JSON document. Golden-file tests diff plans, and `--provider fake` applies
  them. The plan hash makes approvals auditable.

### Config

The file is the same as in Design 1, plus:

```toml
[plan]
requireSavedPlan = false   # true: `apply -f SPEC` refused, must apply a saved PLAN.json
maxPlanAge       = "1h"    # older saved plans -> DRIFT
specDir          = "./specs"
```

### Pros and cons

| Pros | Cons |
|------|------|
| Reviewable, versionable intent in git | Heavyweight for "give me a box for 2 hours" |
| The plan is exactly what will be sent, so dry run and apply cannot diverge | Spec identity, drift and diff logic are real implementation work |
| Natural place for file:line diagnostics | Without a state file, renames are ambiguous (delete+create vs update) |
| Golden tests on plans are easy | TTL is awkward in a declarative world (relative to *when*?) |

---

## Design 3: Presets / profiles (`vm-maker up dev-small --ttl 2h`)

### Philosophy

Most VMs are one of a handful of shapes. Name those shapes once in config as presets, which hold the
type or resources, image, region, cloud-init and default TTL. Day-to-day use is then a short verb plus
a preset. Ad-hoc overrides are allowed but limited to a small set of fields. Presets are validated when
config loads, so a preset that can never work fails before anyone uses it.

### Command tree

```
vm-maker
├── up       <PRESET> [NAME] [--ttl DUR] [--region R] [--count N] [--dry-run] [--wait]
├── down     <NAME|ID> [--yes] [--dry-run]
├── ls       [--preset P] [--expired]
├── info     <NAME|ID>
├── extend   <NAME|ID> <DUR>
├── resize   <NAME|ID> <PRESET> [--dry-run] [--yes]   # resize = move to another preset's shape
├── rename   <NAME|ID> <NEW>
├── label    <NAME|ID> k=v... [-k]...
├── reap     [--dry-run] [--yes]
└── presets
    ├── list
    ├── show   <PRESET> [--resolved]                  # what it resolves to right now, per provider
    └── check                                         # validate all presets against live catalogs
```

### Example sessions

**1. A one-liner dev box.**

```console
$ vm-maker up dev-small --ttl 2h
dev-small-7f3k  cx22 @ fsn1  ubuntu-24.04  cloud-init: devbox
  created id 51234590, ipv4 49.12.3.4, expires 12:00Z (in 2h)
  ssh root@49.12.3.4
```

The name is generated from the preset and a short random suffix. The suffix is deterministic under
`--provider fake` with `--seed`.

**2. List presets and see how they resolve.**

```console
$ vm-maker presets list
PRESET      PROVIDER      SHAPE                       TTL   CLOUD-INIT
dev-small   hetzner       2 vCPU / 4 GB / 40 GB        8h   devbox
dev-arm     hetzner       cax21 (arm)                  8h   devbox
ci-runner   hetzner       4 vCPU / 8 GB / 80 GB        2h   gh-runner
do-small    digitalocean  s-2vcpu-4gb                  8h   devbox

$ vm-maker presets show dev-small --resolved
dev-small -> hetzner cx22 (2/4/40) @ fsn1, ubuntu-24.04 (x86) ✓
  also valid in: nbg1, hel1   not available in: ash, hil, sin
```

**3. A preset becomes invalid because the provider retired a type or image.**

```console
$ vm-maker presets check
dev-small  ✓
dev-arm    ✗ INVALID/image-missing: debian-11-arm no longer offered by hetzner
           suggest: debian-12 (arm)
ci-runner  ✓
do-small   ✓
1 of 4 presets invalid.
$ echo $?
3
```

**4. A TTL over the preset's max, and a multi-VM request.**

```console
$ vm-maker up ci-runner --ttl 3d
error[LIMIT/ttl]: ttl 3d exceeds preset ci-runner maxTtl 12h (config limits.maxTtl 7d, hard 30d)

$ vm-maker up ci-runner --count 3
About to create 3 VMs from preset ci-runner (limits.maxCount 3, hard bound 5):
  ci-runner-a1, ci-runner-a2, ci-runner-a3   cpx31 @ nbg1   expires in 2h
Active after: 4/5 on hetzner.
Type 3 to confirm: 3
Created 3 VMs.
```

**5. Down, and a down on a VM vm-maker does not own.**

```console
$ vm-maker down prod-db
error[NOT_FOUND]: no vm-maker managed VM named prod-db
  note: a server named prod-db exists (id 4100022) but lacks label vm-maker/managed=true;
        vm-maker never touches unmanaged servers.
$ echo $?
6
```

### How invariants surface

- **I1:** the confirmation echoes the count and planned names, and the user must type the number.
  A preset may set `maxCount`, but it can only lower the global one.
- **I2:** presets carry their own optional `maxTtl` and override allow-list. The error shows the whole
  chain: preset limit, config limit and hard bound.
- **I3:** `presets check` and config load validate every preset per provider. `up` re-validates with
  the requested overrides.
- **I4:** presets are fixtures by nature, so tests run `up <preset> --provider fake --dry-run` over
  all presets.

### Config

```toml
defaultProvider = "hetzner"
[limits]
maxCount = 3
maxActive = 5
maxTtl = "7d"

[presets.dev-small]
provider  = "hetzner"
vcpu      = 2
memoryGb  = 4
diskGb    = 40
region    = "fsn1"
image     = "ubuntu-24.04"
cloudInit = { template = "devbox", vars = { user = "nick" } }
ttl       = "8h"
overridable = ["ttl", "region", "name"]   # anything else -> USAGE error

[presets.ci-runner]
provider  = "hetzner"
type      = "cpx31"
region    = "nbg1"
image     = "ubuntu-24.04"
cloudInit = { file = "./cloud-init/gh-runner.yaml" }
ttl       = "2h"
maxTtl    = "12h"
maxCount  = 3
overridable = ["ttl", "count"]

[presets.do-small]
provider  = "digitalocean"
type      = "s-2vcpu-4gb"
region    = "fra1"
image     = "ubuntu-24-04-x64"
cloudInit = { template = "devbox" }
```

### Pros and cons

| Pros | Cons |
|------|------|
| Fastest everyday UX, with no flag soup | Ad-hoc shapes need a config edit first |
| Shapes are pre-validated, so a bad preset fails early and loudly | Preset sprawl, with the config becoming a shadow catalog |
| A narrow override allow-list shrinks the invalid-input space | Less obvious for first-time users and for scripts |
| Easy to test exhaustively (enumerate presets) | `resize` to a preset is coarse |

---

## Comparison

| Criterion | 1 Imperative | 2 Spec + plan/apply | 3 Presets |
|-----------|:------------:|:-------------------:|:---------:|
| Time to first VM | medium | slow | **fast** |
| Ad-hoc / one-off shapes | **best** | ok (write a file) | weak |
| Reviewable / versioned intent | none | **best** | good (config in git) |
| Dry run equals real run, guaranteed | no (two runs) | **yes (saved plan)** | no |
| Bounded count clarity (I1) | good | good | **best (typed confirm)** |
| Early invalid-config detection (I3) | per command | file:line, offline | **at config load** |
| Scriptability / JSON | **best** | good | good |
| Test surface (I4) | simple | **golden plans** | enumerable presets |
| Implementation cost | **low** | high | low-medium |
| Fit for TTL'd scratch VMs | good | awkward | **best** |

---

## Recommendation: imperative core + presets + a plan document

Build **Design 1's command tree** as the canonical surface. Add **Design 3's presets** as a source of
defaults, and expose **Design 2's plan** as the artifact behind `--dry-run`, without a spec-file workflow
for now.

```
vm-maker vm create [NAME] [--preset P] [shape flags...] [--ttl DUR] [--cloud-init T | --user-data F]
                   [--count N] [--dry-run [--plan-out FILE]] [--yes]
vm-maker vm apply  <PLAN.json> [--yes]          # apply a saved plan exactly; DRIFT (8) if stale
vm-maker vm list | show | update | resize | delete | reap
vm-maker up <PRESET> [NAME] [--ttl DUR]         # alias: vm create --preset PRESET
vm-maker down <NAME>                            # alias: vm delete
vm-maker presets list|show|check
vm-maker catalog types|regions|images
vm-maker cloud-init list|render
vm-maker config show|check
```

Why this mix:

1. **One pipeline, three front-ends.** Flags, presets and (later) spec files all decode to the same
   `VmRequest`, then run the same `resolve → validate → plan → apply`. Invariants are enforced once, in
   pure code, and property-tested once.
2. **`--dry-run` *is* `plan`.** Its output (text or JSON) is the serialized `Plan`. With `--plan-out`
   it can be saved and applied verbatim with `vm apply`, which gives Design 2's "what you reviewed is
   what runs" guarantee without needing spec identity or diff engines.
3. **Presets make the safe path the short path.** `vm-maker up dev-small` is the daily driver, and full
   flags remain for one-offs. Presets are validated at config load and by `presets check`, which a CI
   job can run nightly against real catalogs.
4. **Spec files can come later.** `vm create -f spec.yaml` is just another decoder into `VmRequest`. It
   needs no new execution model.

Concrete rules for v1:

- `--count` > 1 requires `limits.maxCount >= N` **and** `--count N`, with N ≤ 5 (hard bound). In a TTY,
  the user must type N to confirm. In a non-TTY, `--yes` is required, otherwise exit 7.
- A TTL is required by default (`limits.requireTtl = true`). If no TTL is given, the preset or config
  default applies, so every VM carries `vm-maker/expires-at`. `--no-ttl` is allowed only when
  `requireTtl = false`.
- `reap` is one-shot and bounded (`reapMax`). It is meant to be run by a systemd timer or cron. vm-maker
  itself never runs a daemon or loop.
- `delete` takes exactly one name or ID. vm-maker refuses to touch any server without
  `vm-maker/managed`.
- Every HTTP call has a bounded number of retries (≤ 3) and a timeout. `--wait` polls on a fixed
  schedule (default 3 min, hard cap 10 min) and exits 9 (`TIMEOUT`) when it times out.
- Output: text for humans and `--output json` (`schemaVersion: 1`) for machines. Under `--now` and a
  fixed catalog, the dry-run output is byte-identical between runs. Golden tests rely on this.
- Exit codes follow §0.4 and are part of the public contract. Changing one is a breaking change.

Suggested first milestone: run `vm create/list/show/delete` and `reap` against `--provider fake` and
Hetzner, add `--dry-run` with JSON plans, then add presets. Then add DigitalOcean, `update`/`resize`
and `vm apply PLAN`.

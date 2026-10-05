# vm-maker

A small, safe CLI for creating, inspecting, updating and deleting virtual machines at cloud
providers, starting with **Hetzner Cloud** and **DigitalOcean**.

## Why

Provider CLIs (`hcloud`, `doctl`) are powerful, and that is the problem. One typo in a loop or
script can create forty servers. vm-maker trades breadth for guardrails:

- **One VM at a time** by default. More requires an explicit config option _and_ a flag, both capped
  by hard limits.
- **Hard upper bounds** on vCPU, memory, disk, TTL and the number of live VMs it manages.
- **No invalid combinations.** Specs are checked against the provider's real catalog (server type ×
  region × image × architecture) before any request is sent. `--dry-run` shows exactly what would be
  sent.
- **TTLs.** Give a VM a lifetime (`--ttl 4h`); `vm-maker reap` deletes expired VMs.
- **cloud-init built in,** so VMs boot already configured from a file or a typed template.
- **Direct API calls,** not CLI wrappers: one typed request and error model across providers.
- **Testing is first class.** The pure policy core is property-tested with fast-check, and the whole
  tool runs against an in-memory fake provider.

See [architecture.md](docs/architecture.md) for the design and [cli.md](docs/cli.md) for the
candidate command-line interfaces.

## Status

Early design. The code is currently a hello-world scaffold that confirms the toolchain works.

## Stack

[Deno 2](https://deno.com) · [Effect](https://effect.website) for typed errors, services and bounded
retries · Effect Schema (or zod) for parsing · [fast-check](https://fast-check.dev) for
property-based tests.

## Getting started

```bash
deno task start            # Hello, world!
deno task start Ada        # Hello, Ada!
deno task test             # unit + property tests
deno task check            # fmt --check, lint, type-check
deno task compile          # single binary at bin/vm-maker
```

Provider tokens will be read from `HCLOUD_TOKEN` and `DIGITALOCEAN_TOKEN`. Never commit them;
`.env*` is git-ignored.

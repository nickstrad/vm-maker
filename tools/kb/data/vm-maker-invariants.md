---
title: vm-maker safety invariants
summary: One VM per invocation by default, no unbounded loops, hard resource ceilings, policy never emits invalid configs, validated cloud-init, TTL labels enforced by reap.
tags: [vm-maker, invariants, policy, ttl, cloud-init, design]
updated: 2026-10-05
---

# vm-maker safety invariants

Every design decision serves these. Source: `architecture.md` at the repo root.

- **One VM per invocation by default (I1).** More needs `limits.maxCount > 1` in config *and* an
  explicit `--count`, both clamped by a compiled-in ceiling (default 1, hard ceiling 5).
- **No unbounded loops (I2).** Every retry, poll and pagination uses a finite Effect `Schedule`
  with an attempt count *and* a wall-clock cap. No `while (true)`, `Effect.forever`, or unbounded
  `Effect.repeat`.
- **Hard resource bounds (I3)** on vCPU, memory, disk, TTL, count and live vm-maker VMs per
  provider. Layered: compiled ceiling >= config limit >= request; config may lower, never raise.
- **Policy never emits an invalid config (I4).** `policy.resolve` is pure and total:
  `Spec -> ResolvedSpec | PolicyViolation[]` (non-empty), built only from tuples that exist in the
  provider catalog (type x region x image x arch). Never throws; stated as a fast-check property.
- **Nothing sent before validation (I5).** Planner/executor split; `--dry-run` prints the exact
  requests.
- **No duplicate creates on retry (I6)** via a `vm-maker/request-id` label.
- **Provider state is the source of truth (I7).** No local database; ownership, TTL and spec
  hash live in provider labels/tags.

## cloud-init

User-data comes from `--cloud-init file.yaml` or a typed template. Before sending it must start
with `#cloud-config`, decode against the supported `CloudConfig` subset, and fit the provider
limit (Hetzner 32 KiB, DigitalOcean 64 KiB). vm-maker injects `/etc/vm-maker.json` metadata.

## TTL and reap

TTL is stored as an absolute `vm-maker/expires-at` label (UTC epoch seconds). `vm-maker reap`
deletes expired owned VMs, at most `limits.maxReapPerRun` per run; scheduling (cron/systemd timer)
lives outside the tool. An optional in-VM power-off timer exists, but a powered-off VM is still
billed, so deletion stays with `reap`.

---
title: What vm-maker is
summary: vm-maker is a guarded Deno CLI that creates, inspects, updates and deletes cloud VMs on Hetzner Cloud and DigitalOcean via their HTTP APIs.
tags: [vm-maker, overview, providers]
updated: 2026-10-05
---

# What vm-maker is

vm-maker is a small, safe CLI for VM CRUD at cloud providers, starting with **Hetzner Cloud** and
**DigitalOcean**. It calls each provider's HTTP API directly (no `hcloud`/`doctl` wrapping), so it
has one typed request model, one error model, and an in-memory fake provider for tests.

The point is guardrails: provider CLIs make it easy to create forty servers with one bad loop.
vm-maker is designed so it cannot do something expensive by accident. See the
`vm-maker-invariants` entry for the rules every change must preserve.

## Status

Early design (as of 2026-10). `src/` is a hello-world scaffold that only proves the toolchain.
The design lives in `architecture.md` and `docs/cli.md`.

## Credentials

Provider tokens are read from `HCLOUD_TOKEN` and `DIGITALOCEAN_TOKEN`. Never commit them; `.env*`
is git-ignored.

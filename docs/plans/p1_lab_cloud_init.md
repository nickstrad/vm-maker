# p1 · Lab cloud-init

**Track p** · builder-strong (opus high / sol high: cloud-init module ordering, the envsubst
collision rule and the 32 KiB budget are easy to get subtly wrong) · depends on nothing ·
owns `cloud-init/**`

## What this slice gives us

`cloud-init/lab.yaml` becomes a root-only first-boot bootstrap closer to the reference box
in [example-vm-software.md](../example-vm-software.md): the apt set that box actually uses,
Docker, Tailscale with forwarding enabled, Node.js 22 with the two coding CLIs, and the
optional agent credential file. It does only what cloud-init is for: one unattended pass at
first boot using package managers and a few written files. Everything else the reference box
has is **not** provisioned by this slice; the section
[What cloud-init covers, and what a new VM will be missing](../example-vm-software.md#what-cloud-init-covers-and-what-a-new-vm-will-be-missing)
lists it. The template stays under 32 KiB, carries five placeholders and no secrets, and
boots cleanly unrendered. `cloud-init/check.ts` validates it offline.

## Architecture of the slice

<!-- draw-visual: diagrams/p1-stages.mmd -->
```text
┌─────────────────────────────────────────────┐
│  workstation: envsubst → lab.rendered.yaml  │
└──────────────────────┬──────────────────────┘
                       ▼
┌─────────────────────────────────────────────┐
│       vm-maker create NAME --user-data      │
└──────────────────────┬──────────────────────┘
                       ▼
┌─────────────────────────────────────────────┐
│ cloud-init: root, apt set, docker, tailscale│
└──────────────────────┬──────────────────────┘
                       ▼
┌─────────────────────────────────────────────┐
│     node 22 + claude, codex; agents.env     │
└──────────────────────┬──────────────────────┘
                       ▼
┌─────────────────────────────────────────────┐
│      /var/lib/cloud/instance/lab-ready      │
└──────────────────────┬──────────────────────┘
                       ▼
┌─────────────────────────────────────────────┐
│post-boot by hand: see example-vm-software.md│
└─────────────────────────────────────────────┘
```

## Code plan

| File | Contents |
| --- | --- |
| `cloud-init/lab.yaml` | Rewrite in place, keeping the header comment's contract (template, placeholders, size limit, `$(...)`/`$VAR` only) and updating its purpose line. **Users:** remove the `lab` user and the `docker` group block; `users: [default]` only; `ssh_pwauth: false`; `disable_root: false`. **apt:** keep the Docker source; `package_update`/`package_upgrade` true; `package_reboot_if_required: false`. **packages:** exactly `git gh curl wget ca-certificates gnupg jq htop btop tmux mosh vim build-essential make gcc libreadline-dev zlib1g-dev shellcheck bubblewrap strace lsof tcpdump net-tools dnsutils iproute2 rsync man-db less python3 sqlite3 sysstat poppler-utils fonts-dejavu-core fonts-dejavu-mono ripgrep fd-find fzf bat tree ncdu socat hyperfine httpie bpftrace linux-tools-generic netcat-openbsd unzip zip e2fsprogs time psmisc procps docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin`. Dropped on purpose: `neovim`, `python3-pip`, `python3-venv`, `pipx`. **write_files:** `/root/.config/lab/agents.env` (0600, same four `export` lines with placeholders `CLAUDE_CODE_OAUTH_TOKEN`, `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GH_TOKEN`); `/etc/sysctl.d/99-tailscale.conf` with the two forwarding lines from `docs/configs/sysctl.d/`. **runcmd**, in order: log redirect and `set -ex` as today; `systemctl enable --now docker`; `bat`/`fd` symlinks; `sysctl --system`; Tailscale install and `tailscale up --authkey --ssh` guarded exactly as today, plus `tailscale set --advertise-exit-node \|\| echo WARN` only when the key was present; Node.js 22 from NodeSource and `npm install -g @anthropic-ai/claude-code @openai/codex sql-formatter`, guarded as today; agents.env cleanup (drop empty and unrendered lines) and the `.bashrc` source line, now for `/root`; finally the `=== done` line and `touch /var/lib/cloud/instance/lab-ready`. **Removed:** the DuckDB `latest` download (the courses need 1.5.5 with extensions; that is a pinned install, out of scope here). |
| `cloud-init/check.ts` | Deno script, run as `deno run --allow-read=cloud-init --allow-run=shellcheck cloud-init/check.ts`. Imports `jsr:@std/yaml@1` by full specifier (no `deno.json` change). Checks, each printed as a line with `ok`/`FAIL`: first line is `#cloud-config`; file parses as YAML and the top level is a map with `packages`, `write_files`, `runcmd`; size is under 32768 bytes and the number is printed; the set of `${NAME}` occurrences equals exactly `{TAILSCALE_AUTHKEY, CLAUDE_CODE_OAUTH_TOKEN, ANTHROPIC_API_KEY, OPENAI_API_KEY, GH_TOKEN}` and no other `${` appears; every `runcmd` entry that is a string is piped to `shellcheck -s bash -` and must pass (entries that are arrays are joined with spaces first); no package name appears twice; the dropped packages (`neovim python3-pip python3-venv pipx`) are absent. Exit 1 on any FAIL. |
| `cloud-init/README.md` | Rewrite: what the template installs (the package list summarized, Docker, Tailscale, Node 22 with Claude Code and Codex, the agent env); the five placeholders and the `envsubst` command; the log (`/var/log/lab-cloud-init.log`) and the `lab-ready` marker, and that `vm-maker create --wait` does not wait for it (use `ssh root@HOST cloud-init status --wait`); the security section as today; a **Not installed** section that links the gap section of `docs/example-vm-software.md` instead of repeating it; the "without keys" section as today, for root. |

Decisions already made; do not reopen:

- **Root only.** The reference box has only `root`; the agent instruction files, Codex trust
  list, skills and caches all assume `/root`. Provider-injected SSH keys already land there.
- **Cloud-init scope only.** The template installs from apt repositories and npm, writes
  files, and joins Tailscale. Pinned binaries, source builds, dotfiles, agent config, skills,
  repository clones and anything needing a login are out of scope and listed as gaps in
  `docs/example-vm-software.md`; they are not hidden inside `runcmd`.
- Node.js comes from NodeSource, not nvm: nvm is a per-user shell tool and fits a first-boot
  script poorly. The gap section records the paths in `docs/configs` that assume nvm.
- The template must still boot with zero placeholders rendered: no Tailscale join, no agent
  env lines.

## Test plan

- **Offline checks:** `deno run --allow-read=cloud-init --allow-run=shellcheck
  cloud-init/check.ts` prints only `ok` lines and exits 0. Add a negative run to the
  reviewer notes: temporarily insert `${EXTRA}` and confirm the check fails.
- **Render round trip (example):** with all five variables exported to dummy values,
  `envsubst` produces a file where `grep -c '\${'` is 0; with none exported the rendered file
  is byte-identical to the template.
- **Schema (reviewer, needs Docker):** `docker run --rm -v "$PWD/cloud-init:/ci:ro"
  ubuntu:24.04 bash -c 'apt-get update -qq && apt-get install -y -qq cloud-init >/dev/null
  && cloud-init schema --config-file /ci/lab.yaml'` reports `Valid schema`.
- **Reviewer checks:** `wc -c cloud-init/lab.yaml` under 32768; `grep -n lab cloud-init/
  lab.yaml` shows no `lab` user or `/home/lab` path left; `grep -n 'git clone\|duckdb'
  cloud-init/lab.yaml` is empty; the `runcmd` order is log → docker → symlinks → sysctl →
  tailscale → node and CLIs → agents.env → done; every optional step is guarded with
  `|| echo`.

## Hand-off

- d2's README rewrite links `cloud-init/README.md`; it does not edit it.
- Closing the gaps listed in `docs/example-vm-software.md` is not planned yet.

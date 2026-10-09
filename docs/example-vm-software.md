# Example VM software: what the reference dev box has installed

This document is the inventory an agent needs to write a cloud-init (or a post-boot script)
that approximates the owner's current development VM. It was taken live from that box on
2026-10-09 and lists every tool, runtime, config file and piece of state that is **custom**, so
that a fresh Ubuntu 24.04 VM built by `vm-maker` comes up ready to work.

Companion folders:

- [`configs/`](configs/README.md): verbatim copies of the dotfiles and unit files, with a table
  of where each one installs.
- [`skills/`](skills/README.md): the five global agent skills (Claude Code and Codex) and how to
  reinstall them.
- [`../cloud-init/lab.yaml`](../cloud-init/lab.yaml): the current template. The
  [gap list](#gaps-in-the-current-cloud-initlabyaml) at the end says what it is missing.

Design decisions this inventory assumes:

- **Work as root.** The reference box has only `root`; both coding CLIs, their skills and all
  caches live in `/root`. `lab.yaml` creates a `lab` user instead; pick one and keep every path
  consistent with it.
- **Hosted services run in containers.** PostgreSQL, Redis and the like are no longer things
  to host natively; the repos that need them bring a `docker compose` file. The only native
  database binaries kept are the ones the course tooling calls directly (see
  [Databases](#databases-and-data-tools)).
- **Pinned where the owner pinned.** DuckDB, Firecracker and SQLite are at exact versions on
  purpose; everything else tracks its upstream stable channel.

## The box itself

| | |
| --- | --- |
| Provider | DigitalOcean droplet, `DO-Regular` shared CPU |
| OS | Ubuntu 24.04.4 LTS (noble), kernel 6.8, `x86_64` |
| Size | 4 vCPU, 7.8 GiB RAM, **no swap**, 160 GiB disk (about 24 GiB used) |
| Hostname | `ubuntu-PostgreSQL-practice`; Tailscale name `vm-1` |
| Timezone | `Etc/UTC` |
| Users | `root` only. `sshd` has `PasswordAuthentication no`. Four `authorized_keys` entries: `nickstrad@ipad`, `digitalocean-mac`, `user@iphone`, `hatch-muse` (the hatch agent's key). |
| Firewall | `ufw` installed but inactive; Tailscale handles access. |
| Nested virtualization | `/dev/kvm` present (`vmx` in cpuinfo). Required for Firecracker. Verify on the target provider and plan: Hetzner shared vCPU types expose KVM; check `test -r /dev/kvm -a -w /dev/kvm` after first boot. |

DigitalOcean-only services that should **not** be recreated elsewhere: `do-agent.service`,
`droplet-agent.service` and their apt repos.

## Tool inventory

Versions are what the box runs today. "Install" is how it got there, which is the method a
cloud-init should reuse unless noted.

### Base system packages (apt)

Present and used:

```
git curl wget ca-certificates gnupg jq htop tmux mosh vim build-essential make gcc
libreadline-dev zlib1g-dev shellcheck bubblewrap strace lsof tcpdump net-tools
rsync python3 sqlite3 sysstat poppler-utils fonts-dejavu-core fonts-dejavu-mono
```

- `tmux` 3.4 and `mosh` 1.4 are the remote-terminal stack. Mosh needs inbound UDP
  60000-61000 in any cloud firewall.
- `libreadline-dev` and `zlib1g-dev` are only there to build SQLite from source.
- `bubblewrap` is the Codex sandbox backend.
- `shellcheck` is used by the repos' script checks.
- `poppler-utils`, `xvfb`, `libnss3`, `libatk*`, `libgbm1`, `libxkbcommon0`, `fonts-liberation`,
  `fonts-noto-color-emoji`, `fonts-wqy-zenhei`, `fonts-ipafont-gothic`, `fonts-tlwg-loma-otf`,
  `fonts-freefont-ttf`, `fonts-unifont`, `xfonts-*`: the Chromium dependency set that Playwright
  installs with `--with-deps`. Needed because `prototyper` and `portfolio` run Playwright tests.
  Letting `npx playwright install --with-deps chromium` (or the Deno equivalent) pull them in is
  cleaner than listing them.

Expected by the tooling but **missing** on the box right now (the owner's own bootstrap script,
`skills-tools/scripts/lab-setup.sh`, installs them; a new VM should too):

```
ripgrep fd-find fzf bat tree ncdu socat btop hyperfine httpie bpftrace
linux-tools-generic netcat-openbsd unzip zip e2fsprogs time psmisc procps
```

Note on `rg`: in shells spawned by Claude Code, `rg` resolves to a shell function that runs
Claude Code's bundled ripgrep 14.1.1. The system has no `ripgrep` package, so `rg` fails in a
plain SSH shell. Install the package.

### Editor and shell

| Tool | Version | Install | Config |
| --- | --- | --- | --- |
| vim | 9.1 | apt | `configs/vimrc`; needs `mkdir -p ~/.vim/undo`. **Neovim is not installed** even though `lab.yaml` installs it. Either drop neovim or add `~/.config/nvim/init.vim` containing `source ~/.vimrc`. |
| bash | 5.2 | distro | `configs/bashrc` tail, `configs/inputrc` (vi mode everywhere, `jj` to command mode) |
| tmux | 3.4 | apt | `configs/tmux.conf` |
| glow | v3.0.0 (`charm.land/glow/v3`), built with Go 1.26.8 | built from source into `/usr/local/bin/glow`; on a new VM `go install charm.land/glow/v3@v3.0.0` then copy `$GOPATH/bin/glow` to `/usr/local/bin`, or use the charmbracelet apt repo | `configs/glow/glow.yml` |
| `EDITOR` | unset on the box (`kb edit` falls back to `vi`) | | `lab-setup.sh` exports `EDITOR=vim`; do the same |

### Version control and GitHub

| Tool | Version | Install | Notes |
| --- | --- | --- | --- |
| git | 2.43.0 | apt | `configs/gitconfig` (name, email, `defaultBranch=main`) |
| gh | 2.101.0 | GitHub release tarball copied to `/usr/local/bin/gh` (mode 0755). The `gh` apt package on noble is older; `lab.yaml` already installs the apt one, which is acceptable. | Logged in as `nickstrad` with `git_protocol: ssh`; token scopes `repo read:org gist admin:public_key`. Auth is a post-boot step (`gh auth login` or `GH_TOKEN`). |
| SSH deploy key | `~/.ssh/id_ed25519` (comment `digitalocean-droplet`) | generated on the box, public key added to GitHub | Repos are cloned over SSH (`git@github.com:nickstrad/...`). Generate a fresh key per VM and register it; do not copy the private key. |

### Runtimes

| Runtime | Version | Install | PATH / env | Notes |
| --- | --- | --- | --- | --- |
| Node.js | 22.23.2 | nvm 0.40.6 (`curl .../nvm-sh/nvm/v0.40.6/install.sh \| bash`, `nvm install 22`, `nvm alias default 22`) | nvm lines in `.bashrc` | `lab.yaml` uses NodeSource instead; either works, but the global npm packages below and `/root/.nvm/versions/node/v22.23.2/bin` paths in configs assume nvm. There is also a stray apt `nodejs` 18.19 at `/usr/bin/node`; do not install it. |
| npm globals | `@anthropic-ai/claude-code` 2.1.295, `@openai/codex` 0.161.0, `sql-formatter` 15.8.2, `corepack` | `npm install -g` | | `sql-formatter` is used by the course authoring tooling. |
| Deno | 2.9.5 | `curl -fsSL https://deno.land/install.sh \| sh -s -- --no-modify-path` into `/root/.deno` | **not on PATH** in `.bashrc`; agents must `export PATH="$HOME/.deno/bin:$PATH"`. Fix this in the new VM's profile. | Used by `vm-maker`, `prototyper`, `systems-trinkets` (k6 scripts). Deno's minimum-dependency-age policy can block fresh `deno install`; see the knowledge store entry `deno-minimum-dependency-age`. |
| Go | 1.26.8 | official tarball extracted to `/usr/local/go` (`lab-setup.sh` discovers the latest stable from `go.dev/dl/?mode=json`) | `/usr/local/go/bin` on PATH; `GOPATH=/root/.local/share/go`, `GOCACHE=/root/.cache/go-build`, `$GOPATH/bin` on PATH; also written to `go env -w` | `/root/go` is a stale leftover; do not recreate. Needed to build `kb`, `tutor`, `glow`, `mermaid-ascii`, the Firecracker lab helpers and all Go lessons. |
| Python | 3.12.3 | distro | | No `pip`, `pipx`, `uv` or venvs are installed or needed. `lab.yaml` installs them; harmless. |

### AI coding agents

| Tool | Version | Install | Global config | Auth |
| --- | --- | --- | --- | --- |
| Claude Code | 2.1.295 | `npm install -g @anthropic-ai/claude-code` under nvm (self-updates via npm). A second, stale copy (2.1.258) sits in `/usr/local/lib/node_modules` with a `/usr/local/bin/claude` symlink; the nvm one wins on PATH. Install exactly one. | `configs/claude/settings.json`, `statusline.sh`, `CLAUDE.md`, `agents/`, skills in `skills/` | Subscription OAuth. `CLAUDE_CODE_OAUTH_TOKEN` from `claude setup-token`, or log in once. Never add an API key to work around limits. |
| Codex | 0.161.0 | `npm install -g @openai/codex` (also has a `/usr/local/bin/codex` symlink) | `configs/codex/config.toml`, `AGENTS.md`, skills in `skills/` | Subscription; `codex login` or copy `~/.codex/auth.json`. |
| Project trust | | | Both CLIs ask once per repo. Codex trust is the `[projects."..."]` table in `config.toml`; Claude's is in `~/.claude.json` (not copied). | |
| Plugins | none installed | | Claude has the official marketplace registered but no plugins enabled; Codex has the curated Google Drive plugin skills **disabled**. Nothing to provision. | |

Both CLIs read the same instruction set: global `CLAUDE.md`/`AGENTS.md` (in `configs/`), the
owner preferences in `/root/AGENTS.md`, and `/root/MUSE_HATCH_README.md`.

### Networking

| Tool | Version | Install | Config |
| --- | --- | --- | --- |
| Tailscale | 1.102.4 | official apt repo (`pkgs.tailscale.com/stable/ubuntu noble`), as `lab.yaml` already does via `install.sh` | `tailscale up --ssh`, then `tailscale set --advertise-exit-node`; `configs/sysctl.d/99-tailscale.conf` turns on forwarding. Exit-node use still needs approval in the admin console. |
| mosh | 1.4.0 | apt | UDP 60000-61000 open |
| OpenSSH | distro | | `PasswordAuthentication no` (cloud-init default) |

### Containers

| Tool | Version | Install |
| --- | --- | --- |
| Docker CE + CLI | 29.7.2 | `download.docker.com/linux/ubuntu` apt repo (`lab.yaml` has this) |
| containerd | 2.3.4 (`containerd.io`) | same repo; `ctr` present, `nerdctl` not installed |
| buildx, compose plugins | current | same repo |

No `/etc/docker/daemon.json`. Docker is the delivery mechanism for every hosted service (next
section), so it must be enabled and started at boot.

### Hosted services: run them in containers

The owner is no longer interested in hosting these natively. Note what the repos expect and
provide Docker/containerd; each repo's compose file starts what it needs.

| Service | Who uses it | How it runs |
| --- | --- | --- |
| PostgreSQL 17 | `portfolio` (`docker-compose.yml`, `postgres:17-alpine` on `127.0.0.1:54329`, ephemeral e2e DB) | `docker compose` in the repo |
| PostgreSQL 18, Redis 8, Valkey 9, SeaweedFS, NATS, etcd, OCI registry, Toxiproxy, Temporal, OpenBao, PgBouncer | `systems-trinkets` (`software/*.compose.yaml`, `make up-<service>`) | `docker compose` per service |
| Redis | nothing native; `redis-server` is not installed | container only |
| Ollama (local embeddings) | `kb search` hybrid mode | native today (see below); could equally be the `ollama/ollama` container bound to `127.0.0.1:11434` |

Two PostgreSQL 16 clusters exist natively on the box (package `16 main` on 5432, and the
learner cluster `pglab` on 5440 under `/labs/pglab`). **Both are stopped and disabled** and the
owner has retired them. Keep the PostgreSQL 16 *binaries* (next section) and ship the
`pglab.service` unit file disabled, but do not create or start any cluster.

### Databases and data tools (native binaries still needed)

| Tool | Version | Install | Why native |
| --- | --- | --- | --- |
| PostgreSQL 16 binaries (`psql`, `pg_ctl`, `postgres`, contrib) | 16.15 Ubuntu build (`postgresql`, `postgresql-client`, `postgresql-contrib` from the Ubuntu archive; `lab-setup.sh` prefers the PGDG repo) | apt | The DuckDB and PostgreSQL course lessons call `pg_ctl`/`psql` directly and create throwaway clusters under `/tmp`. `/usr/lib/postgresql/16/bin` is on PATH. Mask or disable the auto-created `postgresql@16-main` cluster (`/etc/postgresql/16/main/start.conf` set to `manual`). |
| SQLite | 3.53.4, built from the verified upstream autoconf tarball with `-DSQLITE_ENABLE_DBSTAT_VTAB -DSQLITE_ENABLE_DBPAGE_VTAB -DSQLITE_ENABLE_BYTECODE_VTAB -DSQLITE_ENABLE_EXPLAIN_COMMENTS` and `--enable-fts5` into `/usr/local` | `lab-setup.sh` has the exact recipe and SHA3 check | Course lessons rely on the inspection virtual tables; the apt `sqlite3` (3.45) stays installed underneath. |
| DuckDB CLI | **1.5.5 pinned** at `~/.local/bin/duckdb` (release `d8cdaa33fd`), with `postgres` and `sqlite` extensions cached under `~/.duckdb/extensions/v1.5.5/linux_amd64/` | GitHub release zip for the pinned version, not `install.duckdb.org` (which tracks latest) | The Practical DuckDB course validated against 1.5.5. `lab.yaml` downloads `latest`; pin it. |
| sql-formatter | 15.8.2 | npm global | course authoring |

### Virtualization

| Tool | Version | Install | Notes |
| --- | --- | --- | --- |
| Firecracker + jailer | **v1.17.0 pinned**, `/usr/local/bin/firecracker`, `/usr/local/bin/jailer` | GitHub release tarball for `x86_64` | Requires `/dev/kvm`. |
| Guest images | `/var/lib/firecracker/vmlinux.bin` (21 MB), `/var/lib/firecracker/hello-rootfs.ext4` (30 MB) | the official Firecracker hello-world kernel and rootfs, read-only shared bases | Lessons copy the rootfs per VM; never boot it writable in place. |
| Lab helpers | `/root/Raw/scripts/firecracker` (installed copy of `skills-tools/curriculum-tools/courses/firecracker/scripts`) | `bash ~/Raw/scripts/firecracker/install.sh` builds `bin/lab` and `bin/images` with Go and checks for `docker`, `busybox`, `mkfs.ext4`, `losetup`, etc. | `perpetual` (the fx microVM project) also targets this Firecracker install. |

### Local AI and the knowledge store

| Tool | Version | Install | Notes |
| --- | --- | --- | --- |
| Ollama | 0.34.0, system unit `ollama.service` (`User=ollama`, loopback `127.0.0.1:11434`), CPU only | `curl -fsSL https://ollama.com/install.sh \| sh`, then `ollama pull nomic-embed-text` | Backs `kb search` vector ranking. If it is absent, `kb search --mode fts` or `KB_EMBEDDER=none` still works. Optional; a container is fine. |
| kb | `(devel)` build of `github.com/nickstrad/kb`, cgo with `-tags fts5` and sqlite-vec compiled in | `KB_DEFAULT_ROOT=/root/Raw/knowledge /root/Raw/knowledge/kb/scripts/install.sh` (needs Go and gcc; first build about 2 minutes). Produces `/root/Raw/knowledge/kb/.cache/kb` and the `/usr/local/bin/kb` symlink. | Every agent instruction file assumes `kb` exists. |
| Knowledge repository | `/root/Raw/knowledge` (`data/` entries, `kb/` pinned copy of the upstream module at commit `ad5c811`, `AGENTS.md` with `CLAUDE.md` symlinked to it) | **a local git repository with no remote.** It cannot be cloned onto a new VM. Before rebuilding, push it to a new private GitHub repo or `rsync` it over; otherwise the knowledge store and the `update-knowledge-store` skill have nothing to point at. | 29 entries as of today, 6 files with uncommitted changes. |

### Markdown, diagrams and visual review

| Tool | Where | Install |
| --- | --- | --- |
| glow | `/usr/local/bin/glow` | see Editor table |
| mermaid-ascii | `~/.local/share/draw-visual/bin/mermaid-ascii` (pinned `v0.0.0-20260908213847-5f00e3d9ac9f`) | `skills/draw-visual/scripts/setup.sh` (`go install`) |
| diagram-snapshot | `~/.local/share/draw-visual/bin/diagram-snapshot` | built by `scripts/snapshot.sh` from `scripts/snapshot/` |
| Private JRE 21 + PlantUML 1.2026.8 | `~/.local/share/draw-visual/jre`, `~/.local/share/draw-visual/plantuml.jar` | `setup.sh --with-plantuml` |
| Playwright Chromium | `~/.cache/ms-playwright/chromium-1234`, `chromium_headless_shell-1234`, `ffmpeg-1011` (about 650 MB) | from inside `prototyper`/`portfolio`: `npx playwright install --with-deps chromium` (also installs the apt font/lib set above) |
| SeaweedFS `weed` 4.46 | `~/.local/share/systemscoach/tools/seaweedfs-4.46/weed` | downloaded by the systems course launcher; not needed unless that course runs |

### Course and lab tooling

| Item | Where | Install |
| --- | --- | --- |
| `tutor` launcher | `/usr/local/bin/tutor -> /root/Software/skills-tools/bin/tutor` | `cd /root/Software/skills-tools && bin/tutor install` (also links the `tutor` and `curriculum-author` skills) |
| Course progress | `skills-tools/curriculum-tools/tutor.sqlite` | committed in the repo; nothing to migrate |
| PostgreSQL course env | `PGLAB=/labs/pglab PGHOST=/tmp PGPORT=5440 PGUSER=postgres PGDATABASE=lab` in `.bashrc`, `pgcoach`/`pgtutor`/`grpc` aliases | from `configs/bashrc` |
| gRPC course tools (`protoc`, `buf`, `grpcurl`, `protoc-gen-go`) | **not installed**; pruned 2026-09-12. The course README has the pinned installer. | only if that course is revisited |
| k6 | **not installed** (only a `~/.config/k6` id file remains); `systems-trinkets` expects it (`make k6-<lesson>`) | Grafana k6 apt repo or release binary, if trinkets load tests will run |

## Repositories to clone

All under `/root/Software`, over SSH as `nickstrad`:

```
kb perpetual portfolio prototyper skills-tools systems-trinkets vm-agent vm-maker
```

Plus the local-only `/root/Raw/knowledge` (see above) and the owner's `/root/Documents`.
Keep `/root` tidy: its visible top level is `AGENTS.md`, `MUSE_HATCH_README.md`, `Documents/`,
`Raw/`, `Software/`; scratch work goes in `/tmp` (which is emptied at every boot here).

Per-repo runtime needs, so the cloud-init can be checked against them:

| Repo | Needs |
| --- | --- |
| `vm-maker` | Deno (Effect 4, fast-check, cached offline under `~/.cache/deno`) |
| `prototyper` | Deno, Vite, Playwright + Chromium, sqlite-wasm and duckdb-wasm (npm) |
| `portfolio` | Node 22, Next 16, Prisma 6, Playwright, Docker (Postgres 17 compose), Vercel CLI via `npx` |
| `systems-trinkets` | Go, Deno, Docker compose services, DuckDB, k6, `perf` |
| `skills-tools` | Go, `psql`, `sqlite3` 3.53.4, `duckdb` 1.5.5, Docker (for `scripts/docker/test.sh`), Firecracker + `/var/lib/firecracker` images |
| `kb` | Go + gcc (cgo), optionally Ollama |
| `perpetual`, `vm-agent` | Go, Firecracker/KVM (perpetual), Docker |

## State that cloud-init cannot create

These exist on the box as the result of logins or long runs and need either a secret passed at
render time or a manual post-boot step:

1. Claude Code login (`~/.claude/.credentials.json`) and Codex login (`~/.codex/auth.json`).
2. `gh auth login` and the GitHub SSH deploy key for `git@github.com` remotes.
3. Tailscale node identity (use an ephemeral auth key).
4. `/root/Raw/knowledge` contents (no remote; copy or push first).
5. Ollama model pull (`nomic-embed-text`, 274 MB) and the first `kb reindex --all`.
6. Playwright browser download (650 MB) and Deno/npm/Go caches (about 1.9 GB total). They
   refill on first use; just do not be surprised by the first-run time.
7. The `/labs/pglab` cluster data (about 353 MiB). Retired; do not restore.

## Gaps in the current `cloud-init/lab.yaml`

What the template does today versus what this box has:

| Area | `lab.yaml` | Reference box | Suggested change |
| --- | --- | --- | --- |
| User | creates `lab` with sudo | root only | choose; if root, drop the `lab` user and write dotfiles to `/root` |
| Node | NodeSource `nodejs` 22 | nvm 0.40.6 + Node 22 | either; add `sql-formatter` to the npm globals |
| Deno | absent | 2.9.5 in `~/.deno`, not on PATH | add install and a `/etc/profile.d/deno.sh` |
| Go | absent | 1.26.8 tarball + `GOPATH`/`GOCACHE` under XDG dirs | add (see `lab-setup.sh` for the discovery snippet) |
| gh | apt | GitHub release 2.101 | apt is fine |
| Editor | `vim` + `neovim` | vim 9.1 only, `.vimrc`, undo dir | write `configs/vimrc`, `inputrc`, `tmux.conf`, `gitconfig`; create `~/.vim/undo`; decide on neovim |
| glow | absent | v3.0.0 built from source | `go install charm.land/glow/v3@v3.0.0` or charm apt repo; write `glow.yml` |
| DuckDB | `latest` zip | pinned 1.5.5 in `~/.local/bin` + two extensions | pin the version and pre-install `postgres`/`sqlite` extensions (`duckdb -c 'INSTALL postgres; INSTALL sqlite;'`) |
| SQLite | apt only | 3.53.4 from source in `/usr/local` | port the recipe from `lab-setup.sh` |
| PostgreSQL | absent | 16 binaries, clusters disabled | install `postgresql-16 postgresql-contrib postgresql-client-16`, set `start.conf` to `manual`, install `pglab.service` disabled |
| Firecracker | absent | v1.17.0 + jailer + images in `/var/lib/firecracker` | add release download and image fetch; check `/dev/kvm` |
| Ollama | absent | 0.34.0 service + `nomic-embed-text` | optional: install script + pull, or container |
| kb | absent | built from `/root/Raw/knowledge/kb` | clone/copy the knowledge repo, run its `install.sh` |
| Skills and agents | absent | five skills, four agents, statusline | follow `skills/README.md`; copy `configs/claude/*` and `configs/codex/*` |
| Global instructions | absent | `/root/.claude/CLAUDE.md`, `/root/.codex/AGENTS.md`, `/root/AGENTS.md`, `MUSE_HATCH_README.md` | write from `configs/` |
| Tailscale | installed, `up --ssh` | also exit node + sysctl forwarding | write `99-tailscale.conf`; optionally `tailscale set --advertise-exit-node` |
| SSH keys | provider-injected root keys | four named keys | pass the public keys through the provider or `ssh_authorized_keys` |
| Repos | none | eight repos in `/root/Software` | clone after `gh`/SSH auth is in place (post-boot script) |
| Missing apt tools | has most | box lacks `ripgrep fd-find fzf bat tree ncdu socat btop` | keep them in the template; add `mosh shellcheck sysstat hyperfine httpie bpftrace linux-tools-generic poppler-utils fonts-dejavu-core` |
| Playwright deps | absent | Chromium lib/font set | `npx playwright install --with-deps chromium` in a post-boot step for the repos that need it |
| `/tmp` policy | default | emptied at boot, 30-day age-out (distro default) | nothing to do; just do not store state there |

## Verifying a new VM

A short check that mirrors the end of `lab-setup.sh`; every line should print a version:

```bash
export PATH="$HOME/.deno/bin:$HOME/.local/bin:/usr/local/go/bin:$PATH"
for t in git gh claude codex node deno go tailscale docker containerd firecracker jailer \
         vim glow tmux mosh rg jq htop duckdb sqlite3 psql ollama kb tutor shellcheck; do
  printf '%-12s %s\n' "$t" "$(command -v $t >/dev/null && $t --version 2>&1 | head -1 || echo MISSING)"
done
test -r /dev/kvm -a -w /dev/kvm && echo "KVM ready" || echo "KVM unavailable"
ls -lh /var/lib/firecracker/
ls -la /root/.claude/skills /root/.codex/skills
kb doctor
```

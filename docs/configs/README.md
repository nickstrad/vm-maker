# configs: dotfiles and unit files copied from the reference dev box

Verbatim copies of the configuration that makes the reference VM feel like itself. Each row
says where the file lives on the box, so a cloud-init `write_files` entry (or a post-boot
script) can put it back. None of these files hold secrets; tokens live in files that are
deliberately not copied (see "Not copied" below).

| File here | Installs to | Notes |
| --- | --- | --- |
| `bashrc` | `/root/.bashrc` | Stock Ubuntu file plus the custom tail: nvm, PATH entries (skills-tools `bin`, `~/.local/bin`, Go, PostgreSQL 16 binaries), `GOPATH`/`GOCACHE`, PostgreSQL lab env (`PGHOST=/tmp PGPORT=5440 PGUSER=postgres PGDATABASE=lab`), course aliases, `set -o vi`. The tail from `alias cls=` onward is the part worth keeping; the rest is the distro default. It does not add `~/.deno/bin` to PATH, which is a known gap. |
| `inputrc` | `/root/.inputrc` | vi editing mode at every readline prompt, mode indicator, `jj` to leave insert mode, emacs shortcuts kept in insert mode. |
| `vimrc` | `/root/.vimrc` | 2-space indent, `jj` escape, persistent undo in `~/.vim/undo//` (create that directory: `mkdir -p /root/.vim/undo`), truecolor, dark cursorline. Written for vim 9.1; the box has `vim`, not neovim. For neovim, source it from `~/.config/nvim/init.vim` with `source ~/.vimrc`. |
| `tmux.conf` | `/root/.tmux.conf` | Two bindings: `H` even-vertical, `V` even-horizontal. |
| `gitconfig` | `/root/.gitconfig` | Identity and `init.defaultBranch = main`. |
| `glow/glow.yml` | `/root/.config/glow/glow.yml` | glow (Markdown reader) defaults: auto style, no pager, 80-column wrap. |
| `gh/config.yml` | `/root/.config/gh/config.yml` | `gh` preferences and the `co` alias. The real box uses `git_protocol: ssh` in `hosts.yml` (per-host, written by `gh auth login`), which overrides the `https` here. `hosts.yml` holds the OAuth token and is not copied. |
| `claude/settings.json` | `/root/.claude/settings.json` | Claude Code global settings: model `fable`, `KB_CALLER=claude`, statusline command, dark-ansi theme, fullscreen TUI, broad tool allowlist. |
| `claude/statusline.sh` | `/root/.claude/statusline.sh` | Status line script referenced by `settings.json`. Needs `jq` for the fast path. |
| `claude/CLAUDE.md` | `/root/.claude/CLAUDE.md` | Global Claude Code instructions (knowledge store, Muse/hatch workflow, usage limits, subagent dispatch). |
| `claude/agents/*.md` | `/root/.claude/agents/` | Four global subagents: `implementer-opus-high`, `implementer-sonnet-high`, `reviewer-opus-high`, `visual-drawer` (used by the draw-visual skill). |
| `codex/config.toml` | `/root/.codex/config.toml` | Codex global config: model, `KB_CALLER=codex`, TUI theme and status line, project trust list (paths only; prune entries for paths that will not exist), disabled curated plugins. |
| `codex/AGENTS.md` | `/root/.codex/AGENTS.md` | Codex twin of the global `CLAUDE.md`. |
| `AGENTS.md` | `/root/AGENTS.md` | The owner's working preferences (visual validation rule). Loaded by both CLIs when working under `/root`. |
| `MUSE_HATCH_README.md` | `/root/MUSE_HATCH_README.md` | One-page orientation for the hatch agent (Muse). Paths in it assume this layout. |
| `systemd/pglab.service` | `/etc/systemd/system/pglab.service` | Learner PostgreSQL 16 cluster at `/labs/pglab/primary`, port 5440. Intentionally disabled on the reference box; install the unit, do not enable it. |
| `systemd/ollama.service` | `/etc/systemd/system/ollama.service` | Unit written by the Ollama installer, with an `Environment=PATH=...` line that captured the installing shell's PATH. Harmless noise; a fresh `curl -fsSL https://ollama.com/install.sh \| sh` regenerates an equivalent unit. |
| `sysctl.d/99-tailscale.conf` | `/etc/sysctl.d/99-tailscale.conf` | IPv4/IPv6 forwarding so the VM can serve as a Tailscale exit node. |

## Not copied (secrets or machine state)

- `/root/.claude/.credentials.json`, `/root/.claude.json`: Claude Code OAuth and per-machine state. Use `claude setup-token` on a workstation and pass `CLAUDE_CODE_OAUTH_TOKEN`, or run `claude` once after boot.
- `/root/.codex/auth.json`: Codex login. Copy after boot or run `codex login`.
- `/root/.config/gh/hosts.yml`: GitHub OAuth token (`gh auth login`, scopes `repo read:org gist admin:public_key`).
- `/root/.ssh/*`: the four `authorized_keys` entries (`nickstrad@ipad`, `digitalocean-mac`, `user@iphone`, `hatch-muse`) and root's own `id_ed25519` (comment `digitalocean-droplet`) that is registered at GitHub for SSH remotes. Inject the authorized keys through the provider's SSH-key setting or a cloud-init `ssh_authorized_keys` list; generate a new deploy key for GitHub on the new VM.
- Tailscale node state (`/var/lib/tailscale`): join with an auth key instead.

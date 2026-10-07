# cloud-init

`lab.yaml` is a cloud-init template for vm-maker lab VMs (Ubuntu 24.04,
x86_64 or arm64, Hetzner Cloud or DigitalOcean).

## What gets installed

- A `lab` user (passwordless sudo, docker group) that also accepts the
  provider-injected SSH keys.
- Docker CE from the official apt repo, with buildx and compose plugins.
- Tailscale (joined with `--ssh` if an auth key is supplied).
- Node.js 22 plus `@anthropic-ai/claude-code` and `@openai/codex`.
- DuckDB CLI, `gh`, git, tmux, neovim, ripgrep, fd, fzf, bat, jq, btop,
  build-essential, Python 3 with pip/venv/pipx, and other common tools.
- Optional `~lab/.config/lab/agents.env`, sourced from `.bashrc`.

Progress is logged to `/var/log/lab-cloud-init.log`. The file
`/var/lib/cloud/instance/lab-ready` appears when setup is done.

## Render a copy with secrets

The committed file holds only `${NAME}` placeholders. Export the values you
want (unset or empty ones are skipped), then render:

```sh
export TAILSCALE_AUTHKEY=... CLAUDE_CODE_OAUTH_TOKEN=... \
       ANTHROPIC_API_KEY=... OPENAI_API_KEY=... GH_TOKEN=...
envsubst '${TAILSCALE_AUTHKEY} ${CLAUDE_CODE_OAUTH_TOKEN} ${ANTHROPIC_API_KEY} ${OPENAI_API_KEY} ${GH_TOKEN}' \
  < cloud-init/lab.yaml > cloud-init/lab.rendered.yaml
```

`*.rendered.yaml` is git-ignored. Keep the file under 32 KiB (Hetzner limit;
DigitalOcean allows 64 KiB).

## Use it

```sh
vm-maker create NAME --user-data cloud-init/lab.rendered.yaml
```

## Security

User-data is readable from the instance metadata service and through the
provider API, so anything rendered into it is exposed. Prefer short-lived keys:

- Tailscale: a one-off or ephemeral auth key.
- Claude Code: `claude setup-token` gives `CLAUDE_CODE_OAUTH_TOKEN`, or use
  `ANTHROPIC_API_KEY`.
- Codex: `OPENAI_API_KEY`, or copy `~/.codex/auth.json` after boot.
- GitHub: `GH_TOKEN` (fine-grained, short expiry).

## Without keys

Everything still installs. Log in manually: `sudo tailscale up --ssh`,
`claude`, `codex login`, `gh auth login`.

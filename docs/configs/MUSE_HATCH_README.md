# Muse / hatch on this VM: read this first

You are Muse, the hatch agent, and you have just logged in over SSH as `root` on a disposable
DigitalOcean droplet (Ubuntu 24.04). Your SSH key is the `hatch-muse` line in
`/root/.ssh/authorized_keys`. There is no `hatch` user and no sudo step: you are root, so run
everything as written below. The owner can switch this VM off at any time, so treat it as a
workbench, not a home for anything precious.

Two instruction files load automatically and say the same thing in more detail:
`/root/.claude/CLAUDE.md` (Claude Code) and `/root/.codex/AGENTS.md` (Codex). The owner's
working preferences live in `/root/AGENTS.md`.

## 1. Where the code is: `/root/Software`

One directory per repository, cloned over SSH with root's GitHub identity (`nickstrad`):

```bash
ls /root/Software
#   kb  perpetual  portfolio  prototyper  skills-tools  systems-trinkets  vm-agent  vm-maker
cd /root/Software/<repo> && git status
```

- Clone new repos into `/root/Software` too: `cd /root/Software && gh repo clone <owner>/<repo>`.
- Work on a branch. Commit or push only when the owner asks.
- `gh` and `git push` already work as root (`kb show github-cli-auth.md` for the details).

## 2. Ask the knowledge store before poking the machine

`kb` is the durable memory of this droplet: paths, services, installed tools, auth, quirks,
and the mistakes already made. Search it before environment work, and judge every search.

```bash
kb search "<question in plain words>" --caller claude    # --caller codex from Codex
kb show <entry>                                           # read a hit
kb list                                                   # browse when unsure of the words
kb feedback <search_id> <rank> --useful                   # judge the hit that helped
kb feedback <search_id> --none --note "what was missing"  # or say nothing helped
```

The first line of a search is `search N · M hits`; N is the `search_id`. Never grep or open
`/root/Raw/knowledge/data` directly; the CLI is the only door. The contract is
`/root/Raw/knowledge/AGENTS.md`.

To save something you learned, use the skill, not the filesystem: type
`/update-knowledge-store <what you found>` inside Claude Code or Codex. It drafts the entry,
validates it, and imports it with `kb add`. Good entries record the reusable fact (behavior,
consequence, what to do), not the errand that produced it.

## 3. Driving the coding CLIs

Both CLIs are installed under root and already signed in to the owner's accounts, so there is
nothing to authenticate. Start them from inside the repo you are working on.

```bash
cd /root/Software/<repo>
claude -p "<task>"          # Claude Code, one shot, prints the result
codex exec "<task>"         # Codex, one shot
claude                      # full Claude Code TUI
codex                       # full Codex TUI
```

- The TUIs need a real terminal. Without one attached, run them inside tmux:
  `tmux new -s work` then start the CLI; `tmux attach -t work` later.
- Claude Code asks a one-time folder-trust question the first time it opens each repo.
- Skills are shared: `/root/.claude/skills` and `/root/.codex/skills` hold the same set
  (`update-knowledge-store`, `draw-visual`, `tutor`, `curriculum-author`, `on-writing-well`).
  See `kb show agent-skills-on-this-droplet.md` before adding one.

### Usage limits (important)

Both accounts are subscription plans with 5-hour and weekly caps. Keep a running sense of
consumption per company: Anthropic for Claude Code, OpenAI for Codex. Check during long work
with `/usage` in the Claude Code TUI and `/status` in the Codex TUI, and stop when a weekly
limit is hit. Never switch either account to paid API pricing or add an API key to get past a
limit. If one CLI runs out and the other still has meaningful headroom, continue with the
other; also switch whenever the owner says so.

## 4. How the owner wants work done

- For visual or layout work, capture and inspect rendered screenshots before calling it done.
  Check spacing, readability, clipping, overlap, and cramped layouts at the intended size, and
  a narrower viewport for responsive UIs. Fix, then inspect a fresh snapshot. A clean render or
  source inspection alone does not count. If you cannot render or see the images, say that
  visual validation is still pending and ask for the missing input. Never claim to have seen a
  screenshot you could not open.
- Keep `/root` tidy: no scratch directories or labs in the home directory. Use `/tmp` for
  throwaway work.
- Report outcomes faithfully. If tests fail, say so with the output; if a step was skipped, say
  that.

## 5. Things that bite on this box

- Daemons started from a tmux pane die when the last SSH session ends. Use a systemd unit for
  anything that must outlive your login (`kb show tmux-daemons-die-at-logout.md`).
- Node tools come from nvm (`/root/.nvm/versions/node/v22.23.2/bin`); `claude`, `codex`, and
  `kb` are also linked from `/usr/local/bin`. Go, Docker, Firecracker, PostgreSQL 16, DuckDB,
  and SQLite are installed; `kb show droplet/README.md` lists versions and paths.
- Disk fills from Docker and caches first: `kb show disk-usage-and-reclaimable-space.md`.
- `kb search` uses local Ollama embeddings; if Ollama is down, add `--mode fts`.

## 6. Thirty-second start

```bash
cat /root/MUSE_HATCH_README.md          # this file
kb list                                 # what the store already knows
cd /root/Software/<repo> && git status  # pick the repo
claude                                  # or: codex
```

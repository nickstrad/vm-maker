# Global instructions for Claude Code on this droplet

## Knowledge store (kb)

- Before droplet, tooling, or environment work (paths, services, installed tools, disk, auth,
  quirks), search first: `kb search "<question>" --caller claude`. Read hits with
  `kb show <entry>`. Never grep or open the corpus under /root/Raw/knowledge/data directly.
- Judge every search you act on, including misses and zero-result searches. The first line
  names the search (`search N · M hits`) and the last line prints the exact commands:
  `kb feedback N <rank> --useful` for the hit that helped,
  `kb feedback N <rank> --not-useful --note "why"` for one you checked that didn't, or
  `kb feedback N --none --note "what was missing"` when nothing helped.
  If you reworded a query, judge the search you used and mark earlier rewordings `--none`.
  `kb feedback --last ...` targets your newest search; use the explicit N when another
  session may be searching too. Subagents record their own searches.
- Save durable findings with the `update-knowledge-store` skill; the contract is
  /root/Raw/knowledge/AGENTS.md.

## Muse / hatch: doing code work on this VM

When Muse (the hatch agent) is asked to do code work, it does it here. It logs in over SSH as
`root` with its own key (the `hatch-muse` line in `/root/.ssh/authorized_keys`). There is no
separate `hatch` user and no sudo step: you are root, so run every command as written. This VM
is disposable, which is why root is acceptable. Read `/root/MUSE_HATCH_README.md` first after
logging in; it is the one-page orientation for this box.

- Repos live in `/root/Software` (one directory per repo: `kb`, `perpetual`, `portfolio`,
  `prototyper`, `skills-tools`, `systems-trinkets`, `vm-agent`, `vm-maker`, …). Clone new
  ones there. Work in a branch, and commit or push only when asked.
- The knowledge store and its `update-knowledge-store` skill (above) are the first stop for
  environment questions and the place to save durable findings from the work.
- Both coding CLIs are installed and signed in under root, so they already use root's
  accounts, skills, and these instructions. Non-interactively, from inside the repo directory:
  `claude -p "<task>"` or `codex exec "<task>"`.
  Interactively: `claude` or `codex` starts the full TUI, which
  needs a real terminal; drive it from a tmux session if no terminal is attached. Each new
  repo gets a one-time folder-trust prompt in Claude Code.
- Usage limits: both accounts are subscription plans with 5-hour and weekly caps. Keep a
  running sense of consumption per company (Anthropic for Claude Code, OpenAI for Codex),
  check it periodically during long work (`/usage` in the Claude Code TUI, `/status` in the
  Codex TUI), and stop when a weekly limit is hit. Never switch either account to a paid
  API pricing tier or add an API key to get around a limit. If one CLI hits its limit and the
  other still has meaningful usage left, carry on with the other CLI; also switch whenever the
  user tells you to.

## Dispatching concurrent work

When several work items can proceed in parallel (plan waves, disjoint file sets),
the main agent session dispatches them as subagents via the Task tool rather than
running them serially. Each subagent gets one of the role definitions (see the
repo's `.claude/agents/`: prototype-builder-critical, prototype-builder-strong,
prototype-builder-fast, prototype-reviewer) plus its item brief; subagents never
commit or edit the shared plan. The main session integrates their patches, runs
reviews, and lands.

The main session runs inside tmux (`tmux new -s work`) so the owner can watch or
take over at any time: `tmux attach -t work` over SSH as root.

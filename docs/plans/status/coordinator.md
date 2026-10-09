# Coordinator log

Owned by the coordinator (`fable` under Claude Code, `astra` under Codex). Append-only table,
newest last. A fresh coordinator reads the last entries, runs the dashboard in
[AGENTS.md](../AGENTS.md#see-what-peers-and-the-coordinator-are-doing), then `git log
--oneline -8`, `git worktree list`, `git status --short`, and continues. The last entry always
says what is next.

## Board

| Wave | Slices | State |
| --- | --- | --- |
| a | a1, a2 | not started |
| b | b1, b2, b3, b4, b5, b6 | waits on wave a |
| c | c1, c2, c3, c4, c5, c6 | waits on wave b |
| d | d1, d2 | waits on wave c |
| p | p1 | not started; runs alongside any wave |

## Log

| When (UTC) | Entry |
| --- | --- |
| 2026-10-08 | Plans tiered (deep / strong / fast), roles installed under `.claude/agents/vm-*.md`, protocol in `docs/plans/AGENTS.md`, status directory created. **Next: dispatch wave a (a1 → builder-deep, a2 → builder-strong), waiting on the user's go.** |
| 2026-10-09 | Track p (p1–p5) planned from `docs/example-vm-software.md`, `docs/configs`, `docs/skills`; `cloud-init/README.md` moved from d2 to p1. p1 and p2 can be dispatched with wave a. **Next: unchanged, waiting on the user's go.** |
| 2026-10-09 | Track p cut to p1 (cloud-init only) at the user's request; p2–p5 and their diagrams deleted. What a new VM will still be missing is recorded in `docs/example-vm-software.md`. **Next: unchanged, waiting on the user's go.** |

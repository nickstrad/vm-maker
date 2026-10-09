# vm-maker: agent entry point

Read, in this order:

1. [docs/plans/README.md](docs/plans/README.md): what the slices are, the model tier for each, the
   conventions every slice follows.
2. [docs/plans/AGENTS.md](docs/plans/AGENTS.md): the coordination protocol. Where state lives
   (`docs/plans/status/`, `.worktrees/<id>`), who commits (only the coordinator), how to resume any
   slice from a fresh context, how a struggling builder escalates.
3. Your plan file, `docs/plans/<id>_<slug>.md`, if you were given one.

Roles live in `.claude/agents/vm-*.md`. Claude Code reads their frontmatter; Codex reads the first
body line for its model (`astra` = fable, `sol` = opus, `luna` = sonnet).

The coordinator is a `fable` (Claude Code) or `astra` (Codex) session. Its log and the board are in
[docs/plans/status/coordinator.md](docs/plans/status/coordinator.md).

Toolchain: `export PATH="$HOME/.deno/bin:$PATH"`; `deno task check && deno task test` must be green
before any slice is reviewed. Default tests never touch the network.

# Working a plan slice alongside other agents

You were given one plan file from this folder, for example `b3_http.md`, or you are the
**coordinator** that hands slices out. Other agents are working other slices of the same wave
at the same time, each in its own git worktree. This file is the coordination protocol: where
state lives so that any agent, on either platform, with a fresh context, can pick up any
slice; who commits; and how a struggling agent hands off to a stronger one. Read
[README.md](README.md) once for the conventions and the model tiers.

Two platforms run the same protocol. Under Claude Code the roles are the agent definitions in
`.claude/agents/vm-*.md` (frontmatter `model`, `effort`, `tools`). Under Codex the same files
are read as prose and the Codex column applies. Every plan header names both.

| Role | Claude Code | Codex | Job |
| --- | --- | --- | --- |
| coordinator | `fable` (the controlling session) | `astra` (the controlling agent) | creates worktrees and status files, writes briefs, dispatches builders and the reviewer, does the final tweaks, commits, merges, pushes, keeps `status/coordinator.md` |
| builder-deep | `.claude/agents/vm-builder-deep.md`: `fable`, high | `astra`, high | slices where a wrong call is expensive or the semantics are the thing under test |
| builder-strong | `.claude/agents/vm-builder-strong.md`: `opus`, high | `sol`, high | slices with several moving parts and a clear spec |
| builder-fast | `.claude/agents/vm-builder-fast.md`: `sonnet`, high | `luna`, high | self-contained slices where the plan fixes the interfaces and the tests are the spec |
| reviewer | `.claude/agents/vm-reviewer.md`: `opus`, high, read-only | `sol`, high, read-only | reviews every slice before the coordinator commits it |

Escalation order when a builder is struggling: fast → strong → deep → coordinator.

## Where state lives

Everything is derived from the repository; nothing lives in `/tmp` or in an agent's memory.

| What | Where | Written by |
| --- | --- | --- |
| Primary checkout (`$ROOT`) | `git worktree list --porcelain \| head -1 \| cut -d' ' -f2-` | coordinator commits here, on `main` |
| Slice worktree | `$ROOT/.worktrees/<id>` on branch `slice/<id>`, branched from `main` | the builder edits here; nobody commits here except the coordinator |
| Slice status file | `$ROOT/docs/plans/status/<id>.md` (the copy in the **primary checkout**, never the copy inside a worktree) | the builder working the slice; the coordinator writes only the `## Review` section and `status: merged` |
| Coordinator log | `$ROOT/docs/plans/status/coordinator.md` | coordinator only |
| Status template | `$ROOT/docs/plans/status/TEMPLATE.md` | nobody; copy it |

The status directory is committed on `main` by the coordinator until the project is done, so
a fresh clone on another machine sees the same board. Agents update status files in place and
never commit them; the coordinator sweeps them into its own commits.

## Start or resume a slice (builder)

1. `export PATH="$HOME/.deno/bin:$PATH"`, then
   `ROOT=$(git worktree list --porcelain | head -1 | cut -d' ' -f2-)`.
2. Your slice id is the plan file's prefix (`b3` for `b3_http.md`); the wave is the letter.
3. Read `$ROOT/docs/plans/status/<id>.md`. The coordinator created it and the worktree
   before dispatching you, so it exists.
   - `status: planned` means you are starting. Set `status: in-progress`, put your role and
     model in `agent:`, and fill **Next** from the plan's code plan before writing code.
   - Any other status means you are resuming, possibly after another agent. Read the file top
     to bottom, then `git -C $ROOT/.worktrees/<id> status --short` and `git diff --stat`.
     Continue from **Next**. Do not redo **Done**. If **In progress** names a half-written
     file, open it first. Replace `agent:` with yourself and bump `updated:`.
4. `cd $ROOT/.worktrees/<id>`. Work only there. The plan file, `docs/cli.md`, and
   `docs/architecture.md` are the spec; read your plan in full and the sections it links. Do
   not read other plans unless yours names them as a dependency.

## Status file

One file per slice, keep every heading even when empty, keep it under about 60 lines, and
rewrite the whole file on every update. Update it after every completed item in **Next**,
whenever what you are doing next changes, before you stop for any reason, and at least every
30 minutes of work. `updated:` is a UTC timestamp (`date -u +%FT%TZ`); the coordinator uses
it to spot stalled slices.

```markdown
# b3 · HTTP core
status: in-progress      # planned | in-progress | struggling | blocked | ready-for-review | in-review | needs-fixes | merged
tier: strong             # fast | strong | deep | coordinator (current tier; rises on escalation)
agent: vm-builder-strong (opus high) | sol high
worktree: .worktrees/b3
branch: slice/b3
base: 1a2b3c4            # main commit the worktree was branched from
updated: 2026-10-08T14:32:00Z
escalations: 0

## Done
- retry.ts with MC/DC table (5 rows green)

## In progress
- paginate.ts: nextPage done, collectPages half written (fails on repeated cursor test)

## Next
- finish collectPages
- client.ts read/mutate
- layers.ts test client, then layer tests

## Blockers
- none

## Notes for peers
- IncompleteRead.reason values I emit: pages | time | cursor (matches a1)
- ApiClientTest(handler) signature: (req: ApiRequest) => Effect<ScriptedResponse>

## Review
- (coordinator writes: date, verdict, findings count, commit)
```

- **Done** lists finished, tested work present in the worktree. **Next** is a list a stranger
  could execute. If you were cut off mid-file, say which file and what is missing.
- **Blockers** holds anything you need from outside your owned paths, with the exact change
  you need. Keep working on items that do not depend on it.
- **Notes for peers** holds what another slice will plug into: exported names, signatures,
  error codes you emit, decisions you made where the plan left a choice.
- Never edit a peer's status file. Never edit `coordinator.md`.

## Struggling and escalation

Set `status: struggling` and stop when any of these is true:

- the same test or type error has survived three distinct attempts;
- you cannot verify an Effect 4 API in the installed source
  (`~/.cache/deno/npm/registry.npmjs.org/effect/4.0.0/src/`) and the plan depends on it;
- the plan contradicts `docs/cli.md` or `docs/architecture.md` and the "pick the reading
  consistent with cli.md" rule does not settle it;
- you have been working for two hours without moving an item from **Next** to **Done**.

Before stopping, write under **Blockers**: what you tried, the verbatim failing output, and
your best hypothesis. Leave the worktree as it is, even with failing tests. Do not revert.

The coordinator reads the dashboard, raises `tier:` by one step, increments `escalations:`,
and dispatches the next tier on the same worktree with the same brief plus the status file.
The new agent resumes from **Next** and **Blockers**; it does not start over. A slice that
reaches `tier: coordinator` is built by the coordinator in its own session.

`status: blocked` is different: it means a peer or the coordinator must do something outside
your paths before you can finish. Name it under **Blockers** and keep working on what does
not depend on it; the coordinator resolves blocked items between waves.

## See what peers and the coordinator are doing

```bash
grep -H '^status:\|^updated:' $ROOT/docs/plans/status/*.md              # dashboard
sed -n '/## Notes for peers/,/## Review/p' $ROOT/docs/plans/status/<peer>.md   # seams
tail -20 $ROOT/docs/plans/status/coordinator.md                           # coordinator log
for w in $ROOT/.worktrees/*; do echo "$w $(git -C $w status --short | wc -l) changed"; done
```

Wave letters order the dependencies: everything in an earlier wave is merged on `main` before
your wave starts, so you read it from the code, not from status files. Status files of your
own wave tell you what is in flight; they never grant you permission to touch a path your plan
does not list.

## Staying out of each other's way

- Only create or edit paths under your plan's **owns** list, plus test files for those paths.
  `test/support/arbitraries.ts` may be appended to, never reordered.
- If your work needs a change in a shared file (`deno.json`, `src/domain/**`, another slice's
  folder), do not make it. Write the exact change under **Blockers** and under **Notes for
  peers**, then work around it locally if you can (a local type, a wrapper). The coordinator
  applies shared changes between waves.
- **Builders never commit, never push, never touch `main`, never rebase, never create or
  remove worktrees.** The worktree's working tree and the status file are the durable state;
  the coordinator makes the commits. If you need a checkpoint, update the status file.
- Keep `deno task check` and `deno task test` green inside your worktree at every status
  update that moves something to **Done**. Default tests have no network access; if a test
  of yours needs it, the design is wrong.
- When the plan is ambiguous, pick the reading that is consistent with `docs/cli.md`, note
  the decision under **Notes for peers**, and move on. Do not wait for an answer.

## Finishing (builder)

1. In the worktree: `deno task check && deno task test` green.
2. Walk the plan's **Test plan** and **Reviewer checks** and confirm each one.
3. Set `status: ready-for-review` in the status file, with **Done** complete, **Next** empty,
   and every seam under **Notes for peers**.
4. Report to the coordinator in exactly this format, nothing else:

   1. **Files changed**: list.
   2. **Verification**: each command and its one-line outcome.
   3. **Deviations from the plan**: with reasons, or "none".
   4. **Open questions / decisions needed**: or "none".
   5. **Lessons**: reusable, non-obvious things learned, or "none".

5. Stop. Do not commit. Stay available if the harness keeps you alive; the coordinator sends
   review findings back to the same agent when it can.

## Review (reviewer)

The coordinator dispatches you with the builder's brief and report. You are read-only.

1. `cd $ROOT/.worktrees/<id>`. Read the plan, the status file, then the diff:
   `git status --short`, `git diff`, and every untracked file the builder listed.
2. Rerun `deno task check && deno task test` and every command under the plan's **Reviewer
   checks** exactly as written. Never trust the report for these.
3. Check the diff against the plan's **Code plan** signatures (report every deviation,
   reasonable or not), the **owns** list (touched anything else?), and the conventions in
   [README.md](README.md): errors as data, spans on every step, no tokens or user-data in
   logs, no network in default tests.
4. Look for: swallowed errors, resources not closed, tests that cannot fail, MC/DC tables
   whose rows do not flip the outcome, documentation that claims what the code does not do.

Report in exactly this format: **Verdict** (`clean` | `nits only` | `needs fixes`);
**Commands run**, each with a one-line outcome; **Findings** ranked must-fix → should → nit,
each with `file:line`, one sentence stating the defect, and a concrete failure scenario;
**Plan deviations**, or "none".

## Coordinator

The coordinator is a `fable` session under Claude Code or `astra` under Codex. Its context is
for briefs, verdicts, decisions, commits, and the log, not for diffs or transcripts. It reads a
builder's diff only when a finding needs a decision, and for `a1`, whose contract is the
design.

### Dispatch a slice

1. Confirm every dependency in the plan header is `merged` on `main`. Inside a wave, `b6`
   starts only after `b4` has merged (its tests use the fake).
2. `git -C $ROOT worktree add .worktrees/<id> -b slice/<id> main`.
3. `cp docs/plans/status/TEMPLATE.md docs/plans/status/<id>.md`; fill the header with
   `status: planned`, the tier from the plan, the worktree, branch, and `base:` (`git rev-parse
   --short main`).
4. Append to `coordinator.md`: `<id> dispatched → <role> (<model>) in .worktrees/<id>`.
5. Write a **self-contained brief** and dispatch the role named in the plan header. The brief
   contains, in order: the slice id and plan path; the absolute worktree path; the absolute
   status file path; the plan header line verbatim; the **Code plan** table copied in (never
   "see the plan"); the **owns** list and the shared files it must not touch; "match the
   surrounding style; only the imports in `deno.json` unless the plan adds one"; the
   done-when (`deno task check && deno task test` green in the worktree plus the plan's
   **Reviewer checks**); the report format from **Finishing**.
6. Commit the new status file: `git add docs/plans/status && git commit -m "status: <id>
   dispatched"`.

Dispatch every slice of a wave at once when the harness allows; the worktrees keep them
apart. Fewer at once is fine; the order inside a wave does not matter except for `b6`.
Track `p` (provisioning, one slice: `p1`) can be dispatched during any wave; it depends on
nothing.

### Check in

Run the dashboard from **See what peers are doing**. Act on:

| Seen | Action |
| --- | --- |
| `struggling` | raise `tier:`, bump `escalations:`, dispatch the next tier on the same worktree with the same brief plus the status file; log it |
| `blocked` | read **Blockers**; if it is a shared-file change, decide now and either apply it on `main` and tell the builder to rebase their worktree onto `main`, or defer it to the wave gate and say so under **Review** |
| `in-progress` with `updated:` older than two hours and no live agent | treat as stopped: re-dispatch the same tier with the status file; the agent resumes from **Next** |
| `ready-for-review` | go to **Review and land** |
| a question in a builder's report | answer in the brief of the re-dispatch, and record the decision in the plan file if it changes a contract |

### Review and land

1. Set `status: in-review`, dispatch `vm-reviewer` into the worktree with the brief and the
   builder's report. Write the verdict, date, and findings count under **Review** in the
   status file.
2. `needs fixes`: set `status: needs-fixes`, paste the must-fix and should findings under
   **Next**, and send them to the **same builder agent** if it is still alive, otherwise
   dispatch a fresh one at the same tier with the status file. Loop until `clean` or `nits
   only`.
3. Final tweaks: the coordinator may edit in the worktree itself to fix nits, align names
   with the plan, or tidy. Anything larger goes back through step 2. Then in the worktree:
   `deno task check && deno task test`.
4. Commit on the slice branch, one commit per slice, subject `<id>: <slug>`, body listing
   the reviewer's verdict and anything the coordinator changed after review.
5. Merge and push from the primary checkout:

   ```bash
   cd $ROOT
   git add docs/plans/status && git commit -m "status: <id> reviewed" # if anything changed
   git merge --no-ff slice/<id> -m "Merge slice/<id>: <slug>"
   deno task check && deno task test
   git push origin main
   ```

   If the post-merge check fails, `git reset --hard HEAD~1`, set `status: needs-fixes` with
   the output under **Next**, and go back to step 2. Never force-push.
6. Set `status: merged` in the status file, record the merge commit under **Review**, append
   `<id> merged <sha>` to `coordinator.md`, commit the status directory, push.
7. `git worktree remove .worktrees/<id>` and `git branch -d slice/<id>`.

### Wave gate

When every slice in a wave is `merged`: read every **Blockers** and **Notes for peers** in the
wave, apply any shared-file changes on `main` in one commit (`wave <x>: shared changes`),
update the wave table in [README.md](README.md), run `deno task check && deno task test`,
push, log `wave <x> done. Next: wave <y>` in `coordinator.md`, and dispatch the next wave.

### Recover after a context clear or platform switch

1. `tail -20 docs/plans/status/coordinator.md`, then the dashboard, then
   `git log --oneline -8`, `git worktree list`, `git status --short`.
2. Running agents did not survive; their worktrees and status files did. For each slice:
   `ready-for-review` → dispatch the reviewer; `in-review` → dispatch the reviewer again;
   `needs-fixes` or `in-progress` → re-dispatch the tier in `tier:` with the status file;
   `struggling` → escalate; `planned` → dispatch.
3. Log `Resumed on <platform> as <model>; <n> slices in flight` before dispatching anything.
4. Under Codex the coordinator is `astra` and the mapping in the roles table applies; the
   role files repeat it on their first body line. Nothing else changes.

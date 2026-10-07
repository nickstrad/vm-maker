# Working a plan slice alongside other agents

You were given one plan file from this folder, for example `b3_http.md`. Other agents are
working other slices of the same wave at the same time, on their own branches. This file
tells you how to start, how to leave a trail so you (or a replacement with a fresh context)
can resume, and how to see what your peers are doing. Read [README.md](README.md) once for
the conventions; this file is about coordination.

## Start or resume

1. `export PATH="$HOME/.deno/bin:$PATH"` and `cd /root/Software/vm-maker`.
2. Work out your slice id from the plan file name (`b3` for `b3_http.md`) and your wave
   (the letter).
3. Look for your state file: `/tmp/vm-maker-plans/<wave>/<id>.md`.
   - **It exists:** you are resuming. Read it top to bottom, run `git status` and
     `git log --oneline -5` on the branch it names, and continue from its **Next** section.
     Do not redo items under **Done**.
   - **It does not exist:** you are starting fresh. Create it from the template below,
     create the branch `slice/<id>` from `main`, and record both in the file.
4. Read your plan file in full, then the sections of `docs/cli.md` and
   `docs/architecture.md` it links. Do not read other plans unless yours says they are a
   dependency; the hand-off sections tell you what you need.

## State file

Path: `/tmp/vm-maker-plans/<wave>/<id>.md`. Create the directory with `mkdir -p`. One file
per slice, written only by the agent working that slice. It is advisory: the branch and its
commits are the durable truth, and `/tmp` can vanish on reboot. Anyone can read anyone's.

Template, keep every heading even when empty:

```markdown
# b3 · HTTP core
status: in-progress        # planned | in-progress | blocked | ready-for-review | merged
branch: slice/b3
last-commit: 1a2b3c4 Add retry decision and tests
updated: 2026-10-07T14:32:00Z
agent: implementer-opus-high

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
```

Rules:

- Update the file after every commit, whenever you change what you are doing next, and the
  moment you become blocked. Updating means rewriting the whole file; keep it under about 60
  lines.
- **Done** lists finished, tested, committed work. **Next** is a list a stranger could
  execute. If you were cut off mid-file, say which file and what is missing.
- **Notes for peers** holds anything another slice in the wave will plug into: exported
  names, signatures, error codes you emit, decisions you made that the plan left open.
- Set `status: ready-for-review` when `deno task check` and `deno task test` are green and
  the pull request is open; the orchestrator sets `merged`.
- Never edit a peer's state file. If you need something from a peer, write it under
  **Blockers** in your own file and keep working on items that do not depend on it.

## See what peers are doing

```bash
ls /tmp/vm-maker-plans/<wave>/                       # who is active in your wave
grep -H '^status:' /tmp/vm-maker-plans/*/*.md        # one-line dashboard across waves
sed -n '/## Notes for peers/,$p' /tmp/vm-maker-plans/<wave>/*.md   # seams peers exposed
```

Wave letters order the dependencies: everything in an earlier wave is merged on `main`
before your wave starts, so you read it from the code, not from state files. State files of
your own wave tell you what is in flight; they never grant you permission to touch a path
your plan does not list.

## Staying out of each other's way

- Only create or edit paths under your plan's **owns** list, plus test files for those
  paths. `test/support/arbitraries.ts` may be appended to, never reordered.
- If your work needs a change in a shared file (`deno.json`, `src/domain/**`, another
  slice's folder), do not make it. Write the exact change you need under **Blockers** and
  under **Notes for peers**, then work around it locally if you can (a local type, a
  wrapper). The orchestrator applies shared changes between waves.
- Commit small and often on your branch with messages that name the slice:
  `b3: add pagination driver with cursor guard`. Never commit to `main`, never rebase a
  peer's branch, never force-push.
- Keep `deno task check` and `deno task test` green at every commit. Default tests have no
  network access; if a test of yours needs it, the design is wrong.
- When the plan is ambiguous, pick the reading that is consistent with `docs/cli.md`, note
  the decision under **Notes for peers**, and move on. Do not wait for an answer.

## Finishing

1. `deno task check && deno task test` green.
2. Walk the plan's **Test plan** and **Reviewer checks** and confirm each one.
3. Push the branch and open the pull request titled `<id>: <slug>` with the checklist.
4. Set `status: ready-for-review` in the state file with the final commit hash.
5. Stop. The orchestrator reviews, merges, and starts the next wave.

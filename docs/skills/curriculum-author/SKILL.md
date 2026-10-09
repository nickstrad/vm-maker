---
name: curriculum-author
description: Plan or incrementally author hands-on systems courses in the tutor engine. Use for future course proposals, Markdown lesson creation or revision, real-tool validation, and shared skill installation. Planning alone does not implement a course.
---

# Curriculum author

Resolve this skill directory through any installation symlink. The repository root is `../../..`
from that resolved directory; resolve repository-relative paths below against that root.
The engine is the repository's `curriculum-tools/` directory (TUTOR). Read the repository
`docs/README.md`, relevant knowledge entries, learner profile, and `$TUTOR/docs/AUTHORING.md`. The
learner studies around work and children: aim for about ten minutes per new lesson, with a
fifteen-minute core ceiling including context, setup, experiment, interpretation, and cleanup. Keep
unfamiliar mechanisms well explained. Existing reference courses are not a default size.

## Plan before implementing

Use [`future-courses/README.md`](../../../future-courses/README.md) for the planning contract:
create `course.md` immediately, retain research/feedback/decisions there, invite suggestions, and
record explicit final-outline sign-off. Planning does not scaffold a course, create progress or
allocate validation infrastructure. Implementation requires that sign-off plus a batch request;
material scope/count/order changes need renewed sign-off. Keep one canonical route and link to it.

## Implement only the requested batch

1. Read the agreed route and `docs/lesson-batch-workflow.md`. Use the current user's explicit
   model/delegation choice; historical plan assignments do not bind the current batch. Usually
   author 3–4 short lessons; do not build the whole future course.
2. Scaffold only if implementation is requested and the course does not exist:
   `cd $TUTOR && tutor new-course <id> "<Name>" <tool> "<description>" <minVersion>`. Keep
   availability distinct from the planned route.
3. Add `courses/<id>/lessons/NN-<slug>.md` files following the grammar in AUTHORING.md; ordinals
   come from the filename. Prerequisites use earlier slugs as sequencing metadata; the CLI does not
   gate lesson availability on their completion. Preserve learner experiments for metadata-only
   edits. Never edit learner progress by hand.
4. Run `tutor <id> check` and relevant structural checks. Validate each new or changed
   experiment against the real tool in an owned lab; check actual outcomes, not only process exit.
   Run independently and in sequence where state could leak. Use `docs/VALIDATION.md` and the
   course-specific findings. Delegate bounded files only when authorized; review the resulting
   evidence yourself.
5. Smoke-test the shared `tutor <id> <n> lesson|done|skip` flow with an explicit temporary `--db PATH`.
   The learner database is the shared `curriculum-tools/tutor.sqlite`, with rows still scoped to the
   course. Showing content never completes it. Preserve stable identities; bump revisions only for
   material lesson changes and check progress migration on a copy when necessary.
6. Install or synchronize the shared tutor skill if required. Record non-obvious findings in
   `docs/knowledge/` and its index, update authored availability, and report validation limits.
   Clean all owned labs/evidence and verify learner readiness before finishing.

New CLI logic, labs, fixtures, harnesses, and other course tooling follow the repository's
[Go-first language policy](../../../AGENTS.md#language-policy); lesson experiments continue to use
their tool's native commands. Every new lesson also reserves meaningful learner work with a supplied
boundary, evidence to collect, and an attempt budget, as described in the repository's
[learner-work norm](../../../docs/knowledge/learner-work.md).

Read `docs/knowledge/vm-resource-cleanup.md` before allocating labs. Account for peak backup,
replica, archive, and evidence copies, preserve `/labs/pglab` and unrelated work, and retire owned
resources after validation. Historical scratch paths are not instructions to rebuild old labs.

## Shared lesson design

For every course, follow `docs/knowledge/learner-work.md#simple-lab-lifecycle`: simple setup,
focused learner work, simple cleanup. Put recurring environment/fixture setup, starter-file
creation, connection plumbing and teardown in reusable helpers. Keep mechanism-relevant SQL,
commands and evidence visible; helpers must not solve the learner's task. Apply this when
authoring or revising lessons. Verify rendered content, not only Markdown. An explicitly
authorized catalog rollout uses the tutor CLI after copy-based progress checks; authoring alone
continues to use temporary databases.

Format every SQL block, starter and fixture you write with `sql-formatter -l <dialect>` before
validating it, using the repository-root `.sql-formatter.json`. Where the tool cannot take the SQL
(psql meta-commands, SQL inside shell strings), format it by hand in the same style. Follow
AUTHORING.md's Formatting SQL section.

Offer **Setup - script** and **Setup - manual** for software preparation that is useful practice,
following AUTHORING.md's Setup subsection grammar. Both may hide folders, fixtures and shell
variables in helpers; manual exposes the target software's native commands, SQL and settings.
They lead to equivalent starting state and the same visible Run/cleanup. Validate both choices;
generic validation runs the script choice only. Apply across courses as lessons are authored or revised.

Every lesson causes a phenomenon and observes it. The generic CLI owns presentation:
`tutor <course> route` lists status, `<n> lesson` prints one complete lesson, and explicit
`<n> done` or `<n> skip` records the learner's decision. `undone` restores a skipped lesson to
next-lesson eligibility. Follow the complete lesson contract in
[`docs/AUTHORING.md`](../../docs/AUTHORING.md): context and a plain-text-readable mechanism map
before commands, exact setup/action, expected evidence, interpretation, cleanup and honest safety
metadata. Do not implement course-specific renderers, quiz/review stages, prediction/reveal
checkpoints or required external reading. Retired source metadata is preserved in the repository
archive, outside active lessons.

## Validation pitfalls to retain

- The generic harness detects timeouts, not all SQL failures. Read output, classify intentional
  errors, and compare data outcomes with the Expected result section.
- A lesson must recreate its own state or explicitly identify an earlier lesson whose experiment
  provides it; that prerequisite documents setup sequencing, not a CLI availability gate.
- Real two-session experiments must keep separate live connections and deterministic ordering.
- Crash/restart/replication lessons require serial owned fixtures and reliable teardown.
- Feature-probe the actual runtime and any language binding; version text alone may not establish
  required optional extensions.
- Distinguish process failure from power loss, plans from measured work, and engine counters from
  physical device behavior. Do not claim a stronger guarantee than the experiment establishes.

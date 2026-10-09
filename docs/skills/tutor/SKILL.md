---
name: tutor
description: "Guide the PostgreSQL Essentials and Practical DuckDB courses; the PostgreSQL Systems, SQLite Systems, Linux Systems, and gRPC and Protocol Buffers reference courses; the planned SQLite Essentials and Linux Systems v2 routes; and the overall learning roadmap through the tutor CLI. Use for course routes, lessons, search, modules, notes, progress, and what to learn next; not for unrelated systems troubleshooting."
---

# Systems Tutor

Resolve this skill directory through any installation symlink. The repository root is `../../..`
from that resolved directory; resolve repository-relative paths below against that root.
Use the repository's `bin/tutor` (TUTOR below) for curriculum, progress, and roadmap requests.
The CLI prints lesson content; the learner runs the supplied experiment. Do not read or
edit lesson files or `curriculum-tools/tutor.sqlite` directly when serving lessons. The single
database keeps progress course-scoped, so a lesson in one course never completes a lesson in
another. Migrated per-course databases are backups under
`curriculum-tools/.cache/legacy-progress/<course-id>/progress.sqlite*`.

## Course requests

- List courses: `TUTOR courses`.
- Show the full route, including completed, available, and planned entries:
  `TUTOR COURSE route`.
- Show the next unfinished lesson: `TUTOR COURSE lesson`.
- Show a numbered lesson: `TUTOR COURSE NUMBER lesson`.
- Record completion, only when explicitly requested: `TUTOR COURSE NUMBER done [--note TEXT]`.
- Find content: `TUTOR COURSE search TEXT`, `TUTOR COURSE topics`, or
  `TUTOR COURSE modules`.
- Inspect progress: `TUTOR COURSE status --json`.
- Apply an explicit correction: `TUTOR COURSE undone NUMBER`,
  `TUTOR COURSE skip NUMBER [--note TEXT]`, `TUTOR COURSE NUMBER skip`, or
  `TUTOR COURSE note NUMBER TEXT`.

TUTOR and COURSE are abbreviations in this file, not literal arguments. Let the CLI choose the next
lesson rather than deriving it from status output. A planned course supports `route` only; its plan
does not authorize implementation or create progress. If an implemented course is uninitialized,
run `TUTOR COURSE init` and retry. Use `--db PATH` only for isolated author or validation checks.

Show a complete lesson in one view: context, any terminal diagram, setup and commands, expected
evidence, interpretation, and cleanup. Explain unfamiliar concepts and command purpose before the
experiment. Preserve supplied commands and cautions. Give focused help when requested; there is no
separate review, required prediction or reveal, written response, or reading checkpoint.

When a lesson offers **Setup - script** and **Setup - manual**, explain that the learner chooses
one. Manual practices the software's native commands and settings while helpers still prepare
lab folders, fixtures and shell variables; script bypasses that preparation practice. Both feed
the same visible Run/cleanup. Do not hide the tool commands behind an execution helper.

## Courses and routes

- `duckdb` is Practical DuckDB: Data Flows and Improving AI Systems. Lessons 1–5 of its
  32-lesson route are authored; remaining core lessons and optional projects are planned.
  Its experiments use the pinned DuckDB 1.5.5 wrapper and fresh disposable fixtures documented
  in `curriculum-tools/courses/duckdb/README.md`. No prior PostgreSQL course completion is needed.
- `postgres-essentials` is the current PostgreSQL learning path. Its fixed 40-lesson route may
  include planned entries that are not yet authored.
- `postgres-legacy` is the PostgreSQL Systems reference (`postgres` remains its compatibility alias).
  It has its own course-scoped progress; reference lesson numbers and completions do not transfer to
  PostgreSQL Essentials. Use the complete lesson and give focused help with its evidence or optional
  variation.
- `sqlite-legacy` is the SQLite Systems reference (`sqlite` remains its compatibility alias).
  `TUTOR_SQLITE_DB` is needed only for real-tool
  validation; ordinary route, lesson, and progress requests do not need it.
- `linux-legacy` is the Linux Systems reference (`linux` remains its compatibility alias).
- `grpc` is the short gRPC and Protocol Buffers reference. Its local tools and compiled artifacts
  were pruned; before running its experiments, reinstall them as directed by
  `curriculum-tools/courses/grpc/README.md`.
- `sqlite-essentials` and `linux-v2` are planned routes. Show their route and planned status without
  inventing lessons or transferring progress from the reference courses.

For “what should I learn next?” and broader sequencing requests, use `TUTOR roadmap`. Use
`TUTOR roadmap show SLUG` for the selected topic's goals, diagram, and optional Go follow-ups.
Viewing the roadmap is read-only; change it only when the user explicitly asks to update the
roadmap.

## Progress invariants

- Showing, explaining, or author-validating a lesson never marks it done.
- Complete only on an explicit request such as “done” or “mark 12 complete”; resolve the course and
  lesson from recent context only when unambiguous. Do not infer completion from pasted output.
- Let the CLI select the next eligible lesson; it excludes skipped entries and handles stale entries.
- Pass note text as one argument, preserve unrelated progress, and report command errors.

## New lessons

Across all courses, use simple setup, focused learner work and simple cleanup as specified in
`docs/knowledge/learner-work.md#simple-lab-lifecycle`. Recurring shell setup, fixtures,
starter-file creation and teardown belong in reusable helpers. Keep the taught mechanism and
learner decisions visible. Apply this to new or revised lessons; do not assume legacy lessons
already follow it. Verify that the stored catalog displays the revised workflow during an
authorized rollout, preserving recorded progress.

Apply `docs/knowledge/learner-work.md` whenever a new lesson is planned or authored. Reserve a useful
task whose result depends on the learner's work, while supplying setup, fixtures, process control,
success evidence, interpretation, cleanup, optional hints, and a clearly marked worked answer.
Running a finished script, copying a complete solution, or answering a forced quiz does not satisfy
the norm. Fit the explanation, attempt, debugging allowance, evidence, interpretation, and cleanup
into a 10–15 minute lesson.

## Firecracker

`firecracker` is Practical Firecracker: MicroVMs for AI Sandboxes. Lessons 1–7 of its
27-lesson route are authored; later entries remain planned. Lessons 2–4 teach
ext4/loop mounts, Dockerfile build/export, and booting the exported files with Firecracker.
The former API, disk-ownership, and TAP lessons are now 5–7; stable slugs preserve progress.
Use the standard route,
lesson, and explicit progress commands above. Read the course README for host prerequisites.
Helpers are installed at `~/Raw/scripts/firecracker`; lessons show native Firecracker,
curl, Docker, mount, guest-shell, and ip commands. Lesson 4 offers equivalent script/manual
OCI export setup; lesson 7 offers script/manual TAP setup. Image lessons use
`image-session.sh 2|3|4` and `fi_cleanup`; original VM fixtures use `session.sh`
with stable slug arguments and `fc_cleanup`. Docker daemons and loop mounts are private
to an attempt and must be cleaned before closing its shell.
The real labs require host KVM access and namespace/TAP privileges; do not infer those
capabilities from sandbox visibility. Always retain the lesson's scoped cleanup.

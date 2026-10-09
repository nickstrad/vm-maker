# skills: the global agent skills installed on the reference dev box

Both coding CLIs load skills from a per-user directory, and the same five skills are installed
in both: `/root/.claude/skills/` (Claude Code) and `/root/.codex/skills/` (Codex). A skill is a
directory whose name is the invocation name (`/tutor`, `/draw-visual`, ...) containing a
`SKILL.md` with YAML front matter.

The directories here are full copies taken on 2026-10-09 so an agent can see exactly what the
box has. They are **snapshots for reference**; the install procedure below recreates the real
thing, which is a set of symlinks into the repositories that own the skills.

| Skill | Canonical source on the box | Repo | Installed as |
| --- | --- | --- | --- |
| `tutor` | `/root/Software/skills-tools/curriculum-tools/skills/tutor` | `nickstrad/skills-tools` | symlink in both skill dirs, created by `bin/tutor install` |
| `curriculum-author` | `/root/Software/skills-tools/curriculum-tools/skills/curriculum-author` | `nickstrad/skills-tools` | symlink, created by `bin/tutor install` |
| `on-writing-well` | `/root/Software/skills-tools/skills/on-writing-well` | `nickstrad/skills-tools` | symlink in both skill dirs and in `/root/.agents/skills/`, created by `scripts/install-writing-skills.sh` |
| `update-knowledge-store` | `/root/Raw/knowledge/kb/skills/update-knowledge-store` (a pinned copy of `kb/skills/` from `nickstrad/kb`) | `nickstrad/kb` | symlink. On the reference box the Codex link points at a path that no longer exists (`/root/Raw/knowledge/skill/...`); point both CLIs at the `kb/skills/` path. |
| `draw-visual` | `/root/.claude/skills/draw-visual` and `/root/.codex/skills/draw-visual` | none; the skill dirs are the only copies | two independent directories. They are byte-identical except `SKILL.md` (Claude's delegates to the `visual-drawer` subagent; Codex's runs the workflow inline). Both variants are here: `SKILL.md` (Claude) and `SKILL.codex.md` (Codex). |

Claude Code also has a `synced/` directory under its skills folder (Anthropic-managed skills
such as `pdf`, `docx`, `xlsx`, `deep-research`). It is downloaded by the CLI itself on login
and must not be provisioned.

## Runtime dependencies of the skills

- `tutor` and `curriculum-author` need Go 1.26+ (the launcher builds the Go CLI from
  `curriculum-tools/`), plus the course tools when exercises run (`psql`, `duckdb`, `sqlite3`).
  Progress lives in `curriculum-tools/tutor.sqlite`, which is committed in the repo.
- `update-knowledge-store` needs the `kb` binary and `/root/Raw/knowledge` (see the main doc;
  that repository has no remote and must be copied to the new VM).
- `draw-visual` needs Go for `mermaid-ascii` and the PNG snapshot helper, and optionally a
  private JRE plus `plantuml.jar`. `scripts/setup.sh` installs all of it under
  `~/.local/share/draw-visual` (`setup.sh --with-plantuml` for PlantUML). Snapshots use the
  DejaVu Sans Mono font from the `fonts-dejavu-core` package.

## Reinstall on a new VM

```bash
# repos
mkdir -p /root/Software && cd /root/Software
gh repo clone nickstrad/skills-tools && gh repo clone nickstrad/kb

# tutor, curriculum-author (symlinks + /usr/local/bin/tutor)
cd /root/Software/skills-tools && bin/tutor install
# on-writing-well
bash scripts/install-writing-skills.sh

# update-knowledge-store (after /root/Raw/knowledge exists; see main doc)
for d in /root/.claude/skills /root/.codex/skills; do
  ln -sfn /root/Raw/knowledge/kb/skills/update-knowledge-store "$d/update-knowledge-store"
done

# draw-visual: copy the two variants from this folder
mkdir -p /root/.claude/skills /root/.codex/skills
cp -r docs/skills/draw-visual /root/.claude/skills/draw-visual
cp -r docs/skills/draw-visual /root/.codex/skills/draw-visual
mv /root/.codex/skills/draw-visual/SKILL.codex.md /root/.codex/skills/draw-visual/SKILL.md
rm /root/.claude/skills/draw-visual/SKILL.codex.md
bash /root/.claude/skills/draw-visual/scripts/setup.sh --with-plantuml

# global subagents used by draw-visual and the plan workflow
mkdir -p /root/.claude/agents && cp docs/configs/claude/agents/*.md /root/.claude/agents/
```

Use `ln -sfn`, never `ln -sf`, when re-pointing a symlink to a directory; without `-n` the new
link is created inside the old target.

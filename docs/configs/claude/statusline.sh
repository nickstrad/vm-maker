#!/bin/bash
# Claude Code status line.
#
# Reads the status line JSON payload on stdin and prints a single line:
#   <model> | ctx <used>/<max> (<pct>%) | <cwd basename> | <branch>[*] | <cost> (<duration>) | +<added>/-<removed>
#
# Any segment whose source data is missing is silently omitted. No network
# calls are made and git is invoked with --no-optional-locks to stay fast and
# safe to run concurrently with other git operations.

input=$(cat)

# ---- field extraction: jq when available, tiny fallback otherwise ---------
if command -v jq >/dev/null 2>&1; then
  get() { printf '%s' "$input" | jq -r "$1 // empty" 2>/dev/null; }
else
  # Best-effort fallback: grab the first "key": "value" or "key": number pair
  # matching the final path component. Good enough for the flat/near-flat
  # fields this script reads.
  get() {
    key=$(printf '%s' "$1" | sed -E 's/.*\.([A-Za-z0-9_]+)$/\1/')
    printf '%s' "$input" \
      | grep -o "\"$key\"[[:space:]]*:[[:space:]]*\"[^\"]*\"\|\"$key\"[[:space:]]*:[[:space:]]*[0-9.eE+-]*" \
      | head -1 \
      | sed -E 's/^"[^"]+"[[:space:]]*:[[:space:]]*"?//; s/"$//'
  }
fi

model_name=$(get '.model.display_name')

cwd=$(get '.workspace.current_dir')
[ -z "$cwd" ] && cwd=$(get '.cwd')

ctx_used=$(get '.context_window.total_input_tokens')
ctx_max=$(get '.context_window.context_window_size')
ctx_pct=$(get '.context_window.used_percentage')

cost=$(get '.cost.total_cost_usd')
duration_ms=$(get '.cost.total_duration_ms')
lines_added=$(get '.cost.total_lines_added')
lines_removed=$(get '.cost.total_lines_removed')

# ---- colors (terminal renders the status line dimmed, so plain ANSI is fine)
RESET="\033[0m"
YELLOW="\033[33m"
RED="\033[31m"
DIM="\033[2m"

segments=""
add_segment() {
  [ -z "$1" ] && return
  if [ -z "$segments" ]; then
    segments="$1"
  else
    segments="$segments ${DIM}|${RESET} $1"
  fi
}

# ---- 1. model display name --------------------------------------------------
add_segment "$model_name"

# ---- 2. context usage: used/max tokens and percentage -----------------------
fmt_tokens() {
  n="$1"
  [ -z "$n" ] && return
  awk -v n="$n" 'BEGIN {
    if (n >= 1000) printf "%.0fk", n / 1000
    else printf "%d", n
  }'
}

ctx_seg=""
if [ -n "$ctx_used" ] && [ -n "$ctx_max" ] && [ "$ctx_max" != "0" ]; then
  used_h=$(fmt_tokens "$ctx_used")
  max_h=$(fmt_tokens "$ctx_max")
  if [ -n "$ctx_pct" ]; then
    pct_i=$(awk -v p="$ctx_pct" 'BEGIN { printf "%.0f", p }')
  else
    pct_i=$(awk -v u="$ctx_used" -v m="$ctx_max" 'BEGIN { printf "%.0f", (u / m) * 100 }')
  fi
  color=""
  if [ "$pct_i" -gt 90 ] 2>/dev/null; then
    color="$RED"
  elif [ "$pct_i" -gt 70 ] 2>/dev/null; then
    color="$YELLOW"
  fi
  if [ -n "$color" ]; then
    ctx_seg="ctx ${used_h}/${max_h} (${color}${pct_i}%${RESET})"
  else
    ctx_seg="ctx ${used_h}/${max_h} (${pct_i}%)"
  fi
elif [ -n "$ctx_pct" ]; then
  pct_i=$(awk -v p="$ctx_pct" 'BEGIN { printf "%.0f", p }')
  color=""
  if [ "$pct_i" -gt 90 ] 2>/dev/null; then
    color="$RED"
  elif [ "$pct_i" -gt 70 ] 2>/dev/null; then
    color="$YELLOW"
  fi
  if [ -n "$color" ]; then
    ctx_seg="ctx ${color}${pct_i}%${RESET}"
  else
    ctx_seg="ctx ${pct_i}%"
  fi
fi
add_segment "$ctx_seg"

# ---- 3. cwd basename, ~ for home --------------------------------------------
dir_seg=""
if [ -n "$cwd" ]; then
  if [ "$cwd" = "$HOME" ]; then
    dir_seg="~"
  else
    dir_seg=$(basename "$cwd")
  fi
fi
add_segment "$dir_seg"

# ---- 4. git branch + dirty marker (cheap, lock-free checks) -----------------
git_seg=""
if [ -n "$cwd" ] && [ -d "$cwd" ]; then
  branch=$(cd "$cwd" 2>/dev/null && git --no-optional-locks branch --show-current 2>/dev/null)
  if [ -n "$branch" ]; then
    dirty=""
    if (cd "$cwd" 2>/dev/null && git --no-optional-locks status --porcelain 2>/dev/null | head -1 | grep -q .); then
      dirty="*"
    fi
    git_seg="${branch}${dirty}"
  fi
fi
add_segment "$git_seg"

# ---- 5. session cost (USD) and duration -------------------------------------
cost_seg=""
if [ -n "$cost" ]; then
  cost_seg=$(awk -v c="$cost" 'BEGIN { printf "$%.4f", c }')
fi
if [ -n "$duration_ms" ]; then
  dur_h=$(awk -v ms="$duration_ms" 'BEGIN {
    s = ms / 1000
    m = int(s / 60)
    r = int(s % 60)
    if (m > 0) printf "%dm%02ds", m, r
    else printf "%ds", r
  }')
  if [ -n "$cost_seg" ]; then
    cost_seg="$cost_seg ($dur_h)"
  else
    cost_seg="$dur_h"
  fi
fi
add_segment "$cost_seg"

# ---- 6. lines added/removed this session ------------------------------------
lines_seg=""
if [ -n "$lines_added" ] || [ -n "$lines_removed" ]; then
  lines_seg="+${lines_added:-0}/-${lines_removed:-0}"
fi
add_segment "$lines_seg"

printf '%b\n' "$segments"

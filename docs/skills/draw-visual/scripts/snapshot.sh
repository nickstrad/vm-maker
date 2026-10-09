#!/usr/bin/env bash
# Build the pinned PNG snapshot helper and render delivered terminal text.
set -euo pipefail
snapshot_source=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/snapshot" && pwd)
snapshot_home="${DRAW_VISUAL_HOME:-$HOME/.local/share/draw-visual}"
mkdir -p "$snapshot_home/bin"
(
  cd "$snapshot_source"
  go build -buildvcs=false -mod=readonly -o "$snapshot_home/bin/diagram-snapshot" .
)
exec "$snapshot_home/bin/diagram-snapshot" "$@"

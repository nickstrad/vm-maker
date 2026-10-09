#!/usr/bin/env bash
# Install the text-diagram renderers used by the draw-visual skill.
# Idempotent. Installs only beneath DRAW_VISUAL_HOME; touches no system packages.
#
#   setup.sh                  install mermaid-ascii (needs Go)
#   setup.sh --with-plantuml  also install a private JRE and the PlantUML jar
set -euo pipefail

HOME_DIR="${DRAW_VISUAL_HOME:-$HOME/.local/share/draw-visual}"
MERMAID_ASCII_VERSION="v0.0.0-20260908213847-5f00e3d9ac9f"
PLANTUML_VERSION="1.2026.8"
JRE_URL="https://api.adoptium.net/v3/binary/latest/21/ga/linux/x64/jre/hotspot/normal/eclipse"

mkdir -p "$HOME_DIR/bin"

if [ ! -x "$HOME_DIR/bin/mermaid-ascii" ]; then
    command -v go >/dev/null || { echo "setup: Go is required to build mermaid-ascii" >&2; exit 1; }
    echo "setup: building mermaid-ascii $MERMAID_ASCII_VERSION"
    GOBIN="$HOME_DIR/bin" go install "github.com/AlexanderGrooff/mermaid-ascii@$MERMAID_ASCII_VERSION"
fi
echo "setup: mermaid-ascii ready at $HOME_DIR/bin/mermaid-ascii"

if [ "${1:-}" = "--with-plantuml" ]; then
    if [ ! -x "$HOME_DIR/jre/bin/java" ]; then
        echo "setup: downloading a private Java runtime"
        mkdir -p "$HOME_DIR/jre"
        curl -fsSL "$JRE_URL" | tar -xz -C "$HOME_DIR/jre" --strip-components=1
    fi
    if [ ! -f "$HOME_DIR/plantuml.jar" ]; then
        echo "setup: downloading PlantUML $PLANTUML_VERSION"
        curl -fsSL -o "$HOME_DIR/plantuml.jar" \
            "https://github.com/plantuml/plantuml/releases/download/v$PLANTUML_VERSION/plantuml-$PLANTUML_VERSION.jar"
    fi
    echo "setup: PlantUML ready at $HOME_DIR/plantuml.jar"
fi

#!/usr/bin/env bash
#
# Regenere les icones de l'application depuis la source SVG :
#   frontend/src/assets/purgatorius.svg -> build/appicon.png (1024 px, transparent)
# Wails en tire l'icone macOS (.icns) a chaque build et l'icone Windows
# (build/windows/icon.ico) quand elle est absente : on la supprime donc ici.
# Le README affiche build/appicon.png.
#
# Usage : scripts/icone.sh

set -euo pipefail
cd "$(dirname "$0")/.."

go run ./scripts/icone frontend/src/assets/purgatorius.svg build/appicon.png
rm -f build/windows/icon.ico
echo "build/appicon.png regenere ; icon.ico sera recree au prochain wails build"

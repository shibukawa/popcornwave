#!/usr/bin/env bash
# Renders every website/diagrams/*.drawio to an SVG beside the docs that use it.
#
# The SVGs are committed, so building the site and reading the docs need no
# tooling at all. Only editing a diagram needs draw.io, and only the person
# doing the editing runs this.
#
# Fonts are deliberately not embedded: they take an 17 KB drawing to 146 KB, and
# the diagrams use one ordinary sans-serif that every target already has.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source_dir="$root/website/diagrams"
output_dir="$root/website/src/assets/diagrams"

drawio="${DRAWIO:-}"
if [ -z "$drawio" ]; then
  for candidate in \
    "/Applications/draw.io.app/Contents/MacOS/draw.io" \
    "$(command -v drawio || true)"; do
    if [ -n "$candidate" ] && [ -x "$candidate" ]; then
      drawio="$candidate"
      break
    fi
  done
fi
if [ -z "$drawio" ]; then
  echo "draw.io not found. Install https://www.drawio.com/ or set DRAWIO=/path/to/drawio" >&2
  echo "The committed SVGs stay valid; this script is only needed after editing a diagram." >&2
  exit 1
fi

mkdir -p "$output_dir"
for file in "$source_dir"/*.drawio; do
  name="$(basename "$file" .drawio)"
  # -t keeps the background transparent so one file serves the light and dark
  # themes; the exporter also stamps color-scheme: light dark on the root.
  "$drawio" --no-sandbox -x -f svg -t --embed-svg-fonts false \
    -o "$output_dir/$name.svg" "$file"
done

echo "rendered $(ls -1 "$source_dir"/*.drawio | wc -l | tr -d ' ') diagram(s) into website/src/assets/diagrams/"

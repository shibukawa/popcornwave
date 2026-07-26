#!/usr/bin/env bash
# Render the Homebrew formula of decision:homebrew-tap-channel from the release
# archives of data:release-artifact.
#
# Usage: render-formula.sh <version> <archive-dir> <template> <output>
set -euo pipefail

if [ "$#" -ne 4 ]; then
  echo "usage: $0 <version> <archive-dir> <template> <output>" >&2
  exit 2
fi

version="$1"
archive_dir="$2"
template="$3"
output="$4"

case "$version" in
  v*) echo "version must not carry the leading v: ${version}" >&2; exit 1 ;;
  "") echo "version is required" >&2; exit 1 ;;
esac

# The formula covers the platforms brew installs on; windows is out of scope.
targets=(darwin_arm64 darwin_amd64 linux_amd64 linux_arm64)

rendered="$(cat "$template")"
rendered="${rendered//@VERSION@/$version}"

for target in "${targets[@]}"; do
  archive="${archive_dir}/pw_${version}_${target}.tar.gz"
  if [ ! -f "$archive" ]; then
    echo "missing archive: ${archive}" >&2
    exit 1
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    sha256="$(sha256sum "$archive" | cut -d' ' -f1)"
  else
    sha256="$(shasum -a 256 "$archive" | cut -d' ' -f1)"
  fi
  placeholder="@SHA256_$(printf '%s' "$target" | tr '[:lower:]' '[:upper:]')@"
  rendered="${rendered//${placeholder}/${sha256}}"
done

if printf '%s' "$rendered" | grep -q '@[A-Z0-9_]*@'; then
  echo "unresolved placeholder in the rendered formula:" >&2
  printf '%s' "$rendered" | grep -o '@[A-Z0-9_]*@' | sort -u >&2
  exit 1
fi

printf '%s\n' "$rendered" > "$output"
echo "wrote ${output} for pw ${version}"

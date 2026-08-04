#!/bin/sh
# Builds the formatter WebAssembly module the extension ships.
#
# The output is committed, so an install and a CI run need no Go toolchain.
# CI rebuilds it and fails when the result differs from the commit, which is
# what keeps the artifact and its source honest with each other.
#
# The build is reproducible for a fixed TinyGo version: -no-debug drops the
# paths that would otherwise embed this checkout's location.
set -eu

cd "$(dirname "$0")"

if ! command -v tinygo >/dev/null 2>&1; then
	echo "tinygo is required; the committed pwfmt.wasm was built with the version in TOOLCHAIN" >&2
	exit 1
fi

tinygo version | tee TOOLCHAIN

tinygo build \
	-target=wasip1 \
	-opt=z \
	-no-debug \
	-o pwfmt.wasm \
	.

ls -l pwfmt.wasm

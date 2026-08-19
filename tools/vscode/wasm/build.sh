#!/bin/sh
# Builds the formatter WebAssembly module the extension ships.
#
# The output is committed, so an install and a CI run need no Go toolchain.
# CI rebuilds it and fails when the fresh module formats anything differently,
# which is what keeps the artifact and its source honest with each other.
#
# With no argument the committed artifact is rewritten and TOOLCHAIN records
# the toolchain that produced it. With an output path the build goes there and
# TOOLCHAIN is left alone, which is how the freshness check rebuilds without
# leaving a diff behind.
#
# -no-debug drops the paths that would otherwise embed this checkout's
# location. It does not make the build reproducible across machines: TinyGo
# 0.41.1 emits a different module on darwin/arm64 than on linux/amd64, so the
# check compares behavior rather than bytes.
set -eu

cd "$(dirname "$0")"

out=${1:-pwfmt.wasm}

if ! command -v tinygo >/dev/null 2>&1; then
	echo "tinygo is required; the committed pwfmt.wasm was built with the version in TOOLCHAIN" >&2
	exit 1
fi

if [ "$out" = "pwfmt.wasm" ]; then
	tinygo version | tee TOOLCHAIN
else
	tinygo version
fi

tinygo build \
	-target=wasip1 \
	-opt=z \
	-no-debug \
	-o "$out" \
	.

ls -l "$out"

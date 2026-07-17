#!/bin/sh

# Check that documented authentication evidence still exists in the source tree.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"

matrix=docs/contrib-auth-requirement-matrix.md
for file in "$matrix" docs/contrib-auth-implementation-plan.md docs/contrib-auth-verification.md docs/contrib-auth-compatibility.md; do
	if [ ! -f "$file" ]; then
		echo "missing evidence document: $file" >&2
		exit 1
	fi
done

missing=0
while IFS= read -r name; do
	[ -n "$name" ] || continue
	found=0
	for test_file in $(find contrib -type f -name '*_test.go'); do
		if grep -Eq "func[[:space:]]+$name[[:space:]]*\\(" "$test_file"; then
			found=1
			break
		fi
	done
	if [ "$found" -eq 0 ]; then
		echo "missing documented test: $name" >&2
		missing=1
	fi
done <<EOF
$(awk -F'`' '{ for (i = 2; i <= NF; i += 2) if ($i ~ /^Test[A-Za-z0-9_]+$/) print $i }' "$matrix")
EOF

if [ "$missing" -ne 0 ]; then
	exit 1
fi
echo 'authentication evidence references are valid'

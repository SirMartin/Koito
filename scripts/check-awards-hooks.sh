#!/usr/bin/env bash
# Fails if one of the three upstream hook lines the awards feature needs is missing,
# which usually means a rebase dropped it.
set -u
cd "$(dirname "$0")/.."

missing=0
check() {
	if ! grep -q "$1" "$2"; then
		echo "MISSING hook: '$1' in $2" >&2
		missing=1
	fi
}

check 'awards.Handler' engine/routes.go
check 'awards.Start' engine/engine.go
check '<Awards' client/app/routes/MediaItems/MediaLayout.tsx

if [ "$missing" -eq 0 ]; then
	echo "awards hooks OK"
fi
exit "$missing"

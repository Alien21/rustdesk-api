#!/bin/sh
set -eu
cd "$(dirname "$0")"
target_os=${GOOS:-$(go env GOHOSTOS)}
target_arch=${GOARCH:-$(go env GOHOSTARCH)}
target_cgo=${CGO_ENABLED:-1}
export GOCACHE="$PWD/artifacts/cache/go-build"
export GOMODCACHE="$PWD/artifacts/cache/go-mod"
export GOTMPDIR="$PWD/artifacts/tmp"
export TMPDIR="$GOTMPDIR" TMP="$GOTMPDIR" TEMP="$GOTMPDIR"
mkdir -p "$GOCACHE" "$GOMODCACHE" "$GOTMPDIR"
# Run the build helper on this host even when cross-compiling the API.
GOOS="$(go env GOHOSTOS)" GOARCH="$(go env GOHOSTARCH)" CGO_ENABLED=0 \
    go run ./scripts/build/main.go -goos "$target_os" -goarch "$target_arch" \
    -cgo "$target_cgo" -docs "${DOCS:-auto}" -web-dist "${WEB_DIST:-}"

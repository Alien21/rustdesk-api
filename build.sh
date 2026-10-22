#!/bin/sh
set -eu
cd "$(dirname "$0")"
target_os=${GOOS:-$(go env GOHOSTOS)}
target_arch=${GOARCH:-$(go env GOHOSTARCH)}
target_cgo=${CGO_ENABLED:-1}
build_artifacts="$PWD/artifacts"
if [ -n "${RUSTDESK_ARTIFACTS_DIR:-}" ]; then
    case "$RUSTDESK_ARTIFACTS_DIR" in
        /*) build_artifacts="${RUSTDESK_ARTIFACTS_DIR%/}/rustdesk-api" ;;
        *) echo "RUSTDESK_ARTIFACTS_DIR must be an absolute central directory" >&2; exit 2 ;;
    esac
fi
export GOCACHE="$build_artifacts/cache/go-build"
export GOMODCACHE="$build_artifacts/cache/go-mod"
export GOTMPDIR="$build_artifacts/tmp"
export TMPDIR="$GOTMPDIR" TMP="$GOTMPDIR" TEMP="$GOTMPDIR"
mkdir -p "$GOCACHE" "$GOMODCACHE" "$GOTMPDIR"
# Run the build helper on this host even when cross-compiling the API.
GOOS="$(go env GOHOSTOS)" GOARCH="$(go env GOHOSTARCH)" CGO_ENABLED=0 \
    go run ./scripts/build/main.go -goos "$target_os" -goarch "$target_arch" \
    -cgo "$target_cgo" -docs "${DOCS:-auto}" -web-dist "${WEB_DIST:-}"

#!/bin/sh
set -eu
cd "$(dirname "$0")"
export GOENV=off GOTOOLCHAIN=local GOWORK=off
host_os=$(go env GOHOSTOS)
host_arch=$(go env GOHOSTARCH)
target_os=${GOOS:-$host_os}
target_arch=${GOARCH:-$host_arch}
target_cgo=${CGO_ENABLED:-1}
build_base="$(dirname "$PWD")/artifacts"
if [ -n "${RUSTDESK_ARTIFACTS_DIR:-}" ]; then
    case "$RUSTDESK_ARTIFACTS_DIR" in
        /*) build_base="${RUSTDESK_ARTIFACTS_DIR%/}" ;;
        *) echo "RUSTDESK_ARTIFACTS_DIR must be an absolute central directory" >&2; exit 2 ;;
    esac
fi
host_platform="$host_os-$host_arch"
if [ "$host_arch" = amd64 ]; then host_platform="$host_os-x64"; fi
build_tools="$build_base/tools/rustdesk-api"
export GOCACHE="$build_tools/go-build"
export GOMODCACHE="$build_tools/go-mod"
export GOPATH="$build_tools/gopath"
export GOTMPDIR="$build_base/tmp/$host_platform/rustdesk-api"
export TMPDIR="$GOTMPDIR" TMP="$GOTMPDIR" TEMP="$GOTMPDIR"
mkdir -p "$GOCACHE" "$GOMODCACHE" "$GOTMPDIR"
# Run the build helper on this host even when cross-compiling the API.
GOOS="$host_os" GOARCH="$host_arch" CGO_ENABLED=0 \
    go run ./scripts/build/main.go -goos "$target_os" -goarch "$target_arch" \
    -cgo "$target_cgo" -docs "${DOCS:-auto}" -web-dist "${WEB_DIST:-}"

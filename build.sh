#!/usr/bin/env bash
#
# Cross-compile the `vocab` binary for all supported platforms into ./bin.
# Pure-Go dependencies (modernc.org/sqlite) mean CGO is not required.

set -euo pipefail

APP="vocab"
OUT_DIR="bin"

# target platforms: GOOS/GOARCH
PLATFORMS=(
	"linux/amd64"
	"linux/arm64"
	"darwin/amd64"
	"darwin/arm64"
	"windows/amd64"
	"windows/arm64"
)

# version info embedded via ldflags (best-effort; falls back to "dev")
VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
LDFLAGS="-s -w -X main.version=${VERSION}"

export CGO_ENABLED=0

mkdir -p "${OUT_DIR}"

for platform in "${PLATFORMS[@]}"; do
	GOOS="${platform%/*}"
	GOARCH="${platform#*/}"

	output="${OUT_DIR}/${APP}-${GOOS}-${GOARCH}"
	[ "${GOOS}" = "windows" ] && output+=".exe"

	echo "Building ${output} ..."
	GOOS="${GOOS}" GOARCH="${GOARCH}" go build -trimpath -ldflags "${LDFLAGS}" -o "${output}" .
done

echo "Done. Binaries are in ./${OUT_DIR}"

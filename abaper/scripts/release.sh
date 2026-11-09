#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:?Usage: release.sh <version>}"
BINARY="abaper"
OUTPUT_DIR="dist"
PLATFORMS=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
)

mkdir -p "$OUTPUT_DIR"

for platform in "${PLATFORMS[@]}"; do
  IFS='/' read -r os arch <<< "$platform"
  ext=""
  [[ "$os" == "windows" ]] && ext=".exe"

  output="$OUTPUT_DIR/${BINARY}-${os}-${arch}${ext}"
  echo "Building $os/$arch -> $output"

  GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 go build \
    -ldflags "-s -w -X github.com/bluefunda/abaper/internal/commands.version=$VERSION" \
    -o "$output" \
    ./cmd/abaper
done

echo "Release binaries in $OUTPUT_DIR:"
ls -la "$OUTPUT_DIR"

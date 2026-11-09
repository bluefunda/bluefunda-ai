#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-dev}"
BINARY="abaper"
OUTPUT_DIR="bin"

mkdir -p "$OUTPUT_DIR"

echo "Building $BINARY version $VERSION..."
go build \
  -ldflags "-X github.com/bluefunda/abaper/internal/commands.version=$VERSION" \
  -o "$OUTPUT_DIR/$BINARY" \
  ./cmd/abaper

echo "Built: $OUTPUT_DIR/$BINARY"

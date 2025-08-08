#!/bin/bash

# Script pour générer latest.json pour le serveur statique
# Usage: ./scripts/generate-latest-json.sh <version> <release_date>

set -e

VERSION=${1:-"v2.4.0"}
RELEASE_DATE=${2:-$(date +%Y-%m-%d)}
CHANGELOG=${3:-"Latest release"}

echo "Generating latest.json for version $VERSION..."

# Vérifier que les binaires existent
BINARIES=(
    "dist/bcrdf-linux-x64"
    "dist/bcrdf-linux-arm64"
    "dist/bcrdf-darwin-x64" 
    "dist/bcrdf-darwin-arm64"
    "dist/bcrdf-windows-x64.exe"
)

for binary in "${BINARIES[@]}"; do
    if [ ! -f "$binary" ]; then
        echo "❌ Error: $binary not found"
        echo "Please run 'make build-all' first"
        exit 1
    fi
done

# Calculer les checksums
LINUX_X64_CHECKSUM=$(sha256sum dist/bcrdf-linux-x64 | cut -d' ' -f1)
LINUX_ARM64_CHECKSUM=$(sha256sum dist/bcrdf-linux-arm64 | cut -d' ' -f1)
DARWIN_X64_CHECKSUM=$(sha256sum dist/bcrdf-darwin-x64 | cut -d' ' -f1)
DARWIN_ARM64_CHECKSUM=$(sha256sum dist/bcrdf-darwin-arm64 | cut -d' ' -f1)
WINDOWS_CHECKSUM=$(sha256sum dist/bcrdf-windows-x64.exe | cut -d' ' -f1)

# Obtenir les tailles (compatible macOS et Linux)
LINUX_X64_SIZE=$(stat -f%z dist/bcrdf-linux-x64 2>/dev/null || stat -c%s dist/bcrdf-linux-x64)
LINUX_ARM64_SIZE=$(stat -f%z dist/bcrdf-linux-arm64 2>/dev/null || stat -c%s dist/bcrdf-linux-arm64)
DARWIN_X64_SIZE=$(stat -f%z dist/bcrdf-darwin-x64 2>/dev/null || stat -c%s dist/bcrdf-darwin-x64)
DARWIN_ARM64_SIZE=$(stat -f%z dist/bcrdf-darwin-arm64 2>/dev/null || stat -c%s dist/bcrdf-darwin-arm64)
WINDOWS_SIZE=$(stat -f%z dist/bcrdf-windows-x64.exe 2>/dev/null || stat -c%s dist/bcrdf-windows-x64.exe)

# Générer le JSON
cat > latest.json << EOF
{
  "version": "$VERSION",
  "release_date": "$RELEASE_DATE",
  "changelog": "$CHANGELOG",
  "assets": [
    {
      "name": "bcrdf-linux-x64",
      "url": "https://static.crdf.fr/bcrdf/bcrdf-linux-x64",
      "size": $LINUX_X64_SIZE,
      "checksum": "sha256:$LINUX_X64_CHECKSUM"
    },
    {
      "name": "bcrdf-linux-arm64",
      "url": "https://static.crdf.fr/bcrdf/bcrdf-linux-arm64",
      "size": $LINUX_ARM64_SIZE,
      "checksum": "sha256:$LINUX_ARM64_CHECKSUM"
    },
    {
      "name": "bcrdf-darwin-x64",
      "url": "https://static.crdf.fr/bcrdf/bcrdf-darwin-x64",
      "size": $DARWIN_X64_SIZE,
      "checksum": "sha256:$DARWIN_X64_CHECKSUM"
    },
    {
      "name": "bcrdf-darwin-arm64",
      "url": "https://static.crdf.fr/bcrdf/bcrdf-darwin-arm64",
      "size": $DARWIN_ARM64_SIZE,
      "checksum": "sha256:$DARWIN_ARM64_CHECKSUM"
    },
    {
      "name": "bcrdf-windows-x64.exe",
      "url": "https://static.crdf.fr/bcrdf/bcrdf-windows-x64.exe",
      "size": $WINDOWS_SIZE,
      "checksum": "sha256:$WINDOWS_CHECKSUM"
    }
  ]
}
EOF

echo "✅ Generated latest.json:"
echo "   Version: $VERSION"
echo "   Release date: $RELEASE_DATE"
echo "   Linux x64: $LINUX_X64_SIZE bytes"
echo "   Linux arm64: $LINUX_ARM64_SIZE bytes"
echo "   macOS x64: $DARWIN_X64_SIZE bytes"
echo "   macOS arm64: $DARWIN_ARM64_SIZE bytes" 
echo "   Windows: $WINDOWS_SIZE bytes"

echo ""
echo "📋 Next steps:"
echo "1. Upload latest.json to https://static.crdf.fr/bcrdf/"
echo "2. Upload binaries to https://static.crdf.fr/bcrdf/"
echo "3. Test with: ./build/bcrdf update -v"

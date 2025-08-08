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
    "build/bcrdf-linux-x64"
    "build/bcrdf-darwin-x64" 
    "build/bcrdf-windows-x64.exe"
)

for binary in "${BINARIES[@]}"; do
    if [ ! -f "$binary" ]; then
        echo "❌ Error: $binary not found"
        echo "Please run 'make build-all' first"
        exit 1
    fi
done

# Calculer les checksums
LINUX_CHECKSUM=$(sha256sum build/bcrdf-linux-x64 | cut -d' ' -f1)
DARWIN_CHECKSUM=$(sha256sum build/bcrdf-darwin-x64 | cut -d' ' -f1)
WINDOWS_CHECKSUM=$(sha256sum build/bcrdf-windows-x64.exe | cut -d' ' -f1)

# Obtenir les tailles
LINUX_SIZE=$(stat -c%s build/bcrdf-linux-x64)
DARWIN_SIZE=$(stat -c%s build/bcrdf-darwin-x64)
WINDOWS_SIZE=$(stat -c%s build/bcrdf-windows-x64.exe)

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
      "size": $LINUX_SIZE,
      "checksum": "sha256:$LINUX_CHECKSUM"
    },
    {
      "name": "bcrdf-darwin-x64",
      "url": "https://static.crdf.fr/bcrdf/bcrdf-darwin-x64",
      "size": $DARWIN_SIZE,
      "checksum": "sha256:$DARWIN_CHECKSUM"
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
echo "   Linux: $LINUX_SIZE bytes"
echo "   macOS: $DARWIN_SIZE bytes" 
echo "   Windows: $WINDOWS_SIZE bytes"

echo ""
echo "📋 Next steps:"
echo "1. Upload latest.json to https://static.crdf.fr/bcrdf/"
echo "2. Upload binaries to https://static.crdf.fr/bcrdf/"
echo "3. Test with: ./build/bcrdf update -v"

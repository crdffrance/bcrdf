#!/bin/bash

# Script de compilation multi-architectures pour BCRDF
# Usage: ./scripts/build-all.sh [version]

set -e

# Version par défaut
VERSION=${1:-"dev-$(git rev-parse --short HEAD 2>/dev/null || echo 'local')"}

echo "🔨 Compilation BCRDF pour toutes les architectures"
echo "Version: $VERSION"
echo "=================================================="

# Créer le répertoire de sortie
mkdir -p dist

# Fonction de compilation
build_for() {
    local os=$1
    local arch=$2
    local target=$3
    local ext=$4
    
    echo "📦 Compilation pour $target..."
    
    # Variables d'environnement pour cross-compilation
    export GOOS=$os
    export GOARCH=$arch
    export CGO_ENABLED=0
    
    # Compilation
    go build \
        -ldflags "-X main.Version=$VERSION-$target" \
        -o "dist/bcrdf-$target" \
        cmd/bcrdf/main.go
    
    # Créer l'archive
    cd dist
    if [ "$os" = "windows" ]; then
        zip "bcrdf-$target.zip" "bcrdf-$target"
        echo "✅ $target: bcrdf-$target.zip"
    else
        tar -czf "bcrdf-$target.tar.gz" "bcrdf-$target"
        echo "✅ $target: bcrdf-$target.tar.gz"
    fi
    cd ..
    
    # Nettoyer le binaire
    rm "dist/bcrdf-$target"
}

# Architectures à compiler
declare -a targets=(
    "linux:amd64:linux-x64"
    "linux:arm64:linux-arm64"
    "linux:386:linux-x32"
    "windows:amd64:windows-x64"
    "windows:arm64:windows-arm64"
    "windows:386:windows-x32"
    "darwin:amd64:darwin-x64"
    "darwin:arm64:darwin-arm64"
)

# Compiler pour chaque architecture
for target in "${targets[@]}"; do
    IFS=':' read -r os arch name <<< "$target"
    
    if [ "$os" = "windows" ]; then
        build_for "$os" "$arch" "$name" "zip"
    else
        build_for "$os" "$arch" "$name" "tar.gz"
    fi
done

# Afficher le résumé
echo ""
echo "🎉 Compilation terminée !"
echo "📦 Binaries disponibles dans dist/:"
echo ""

# Lister les fichiers créés
ls -la dist/

echo ""
echo "📋 Résumé des architectures:"
echo "  Linux:   x64, ARM64, x32"
echo "  Windows: x64, ARM64, x32"
echo "  macOS:   x64, ARM64"
echo ""
echo "🚀 Pour installer:"
echo "  # Linux/macOS"
echo "  tar -xzf dist/bcrdf-linux-x64.tar.gz"
echo "  sudo mv bcrdf /usr/local/bin/"
echo ""
echo "  # Windows"
echo "  # Extraire le zip et ajouter au PATH" 
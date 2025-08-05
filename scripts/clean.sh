#!/bin/bash

# Script de nettoyage BCRDF
# Usage: ./scripts/clean.sh

echo "🧹 Nettoyage du projet BCRDF"
echo "============================"

# Supprimer les binaires
echo "🗑️  Suppression des binaires..."
rm -f bcrdf
rm -f *.exe
rm -f *.test

# Supprimer les fichiers temporaires
echo "🗑️  Suppression des fichiers temporaires..."
rm -rf restored-*/
rm -rf test-data/
rm -rf temp/
rm -f *.log
rm -f *.tmp
rm -f *.bak

# Supprimer les fichiers de couverture
echo "🗑️  Suppression des rapports de couverture..."
rm -f coverage.txt
rm -f coverage.html

# Supprimer les fichiers de build
echo "🗑️  Suppression des artefacts de build..."
rm -rf build/
rm -rf dist/

# Nettoyer les caches Go
echo "🗑️  Nettoyage des caches Go..."
go clean -cache
go clean -modcache

echo "✅ Nettoyage terminé !" 
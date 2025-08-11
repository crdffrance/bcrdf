#!/bin/bash

# Script pour analyser la structure du stockage S3 et tester la fonction clean

set -e

# Couleurs pour l'affichage
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_status "Analyse de la structure du stockage S3..."

# Test 1: Lister les index disponibles
print_status "Test 1: Liste des index disponibles"
./bcrdf list --verbose 2>&1 | head -20

echo ""
print_status "Test 2: Analyse du nettoyage complet (dry-run)"
echo "y" | ./bcrdf clean --all --dry-run --verbose 2>&1 | head -50

echo ""
print_status "Test 3: Analyse avec suppression des orphelines (dry-run)"
echo "y" | ./bcrdf clean --all --remove-orphaned --dry-run --verbose 2>&1 | head -50

print_status "Analyse terminée. Vérifiez la sortie ci-dessus pour comprendre la structure."

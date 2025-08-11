#!/bin/bash

# Script de test pour la fonction de nettoyage BCRDF
# Ce script teste la commande clean avec différents scénarios

set -e

# Couleurs pour l'affichage
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
CONFIG_FILE="configs/config-development.yaml"
BACKUP_ID="test-clean-$(date +%s)"
SOURCE_DIR="test-clean-source"
VERBOSE=""

# Fonction d'affichage
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

# Fonction de nettoyage
cleanup() {
    print_status "Nettoyage des fichiers de test..."
    rm -rf "$SOURCE_DIR"
    rm -rf "restore-$BACKUP_ID"
    print_success "Nettoyage terminé"
}

# Gestion des erreurs
trap cleanup EXIT

# Vérification des prérequis
if ! command -v bcrdf &> /dev/null; then
    print_error "BCRDF n'est pas installé ou n'est pas dans le PATH"
    exit 1
fi

if [ ! -f "$CONFIG_FILE" ]; then
    print_error "Fichier de configuration $CONFIG_FILE introuvable"
    exit 1
fi

# Test de la configuration
print_status "Test de la configuration..."
if ! bcrdf --config "$CONFIG_FILE" init --test; then
    print_error "Échec du test de configuration"
    exit 1
fi
print_success "Configuration testée avec succès"

# Création du répertoire source de test
print_status "Création du répertoire source de test..."
mkdir -p "$SOURCE_DIR"
echo "Fichier 1" > "$SOURCE_DIR/file1.txt"
echo "Fichier 2" > "$SOURCE_DIR/file2.txt"
mkdir -p "$SOURCE_DIR/subdir"
echo "Fichier 3" > "$SOURCE_DIR/subdir/file3.txt"
print_success "Répertoire source créé avec 3 fichiers"

# Création d'une sauvegarde
print_status "Création d'une sauvegarde de test..."
if ! bcrdf --config "$CONFIG_FILE" backup --source "$SOURCE_DIR" --name "$BACKUP_ID" $VERBOSE; then
    print_error "Échec de la création de la sauvegarde"
    exit 1
fi
print_success "Sauvegarde créée avec succès"

# Test de la commande clean en mode dry-run
print_status "Test de la commande clean en mode dry-run..."
if ! bcrdf --config "$CONFIG_FILE" clean --backup-id "$BACKUP_ID" --dry-run $VERBOSE; then
    print_error "Échec de la commande clean en mode dry-run"
    exit 1
fi
print_success "Commande clean en mode dry-run exécutée avec succès"

# Test de la commande clean en mode live (avec confirmation automatique)
print_status "Test de la commande clean en mode live..."
echo "yes" | bcrdf --config "$CONFIG_FILE" clean --backup-id "$BACKUP_ID" $VERBOSE
print_success "Commande clean en mode live exécutée avec succès"

# Vérification que la sauvegarde fonctionne toujours
print_status "Vérification de la restauration de la sauvegarde..."
if ! bcrdf --config "$CONFIG_FILE" restore --backup-id "$BACKUP_ID" --destination "restore-$BACKUP_ID" $VERBOSE; then
    print_error "Échec de la restauration après nettoyage"
    exit 1
fi
print_success "Restauration réussie après nettoyage"

# Vérification de l'intégrité des fichiers restaurés
print_status "Vérification de l'intégrité des fichiers restaurés..."
if ! diff -r "$SOURCE_DIR" "restore-$BACKUP_ID"; then
    print_error "Différences détectées entre source et restauration"
    exit 1
fi
print_success "Intégrité des fichiers restaurés vérifiée"

# Test de la commande clean sur une sauvegarde vide
print_status "Test de la commande clean sur une sauvegarde vide..."
if ! bcrdf --config "$CONFIG_FILE" clean --backup-id "$BACKUP_ID" --dry-run $VERBOSE; then
    print_error "Échec de la commande clean sur sauvegarde vide"
    exit 1
fi
print_success "Commande clean sur sauvegarde vide exécutée avec succès"

print_success "Tous les tests de nettoyage ont réussi !"
print_status "La fonction clean fonctionne correctement"

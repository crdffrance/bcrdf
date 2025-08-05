#!/bin/bash

# Script de démonstration BCRDF
# Ce script montre l'utilisation basique du système de sauvegarde index-based

set -e

echo "🚀 Démonstration BCRDF - Système de sauvegarde index-based"
echo "=========================================================="
echo ""

# Couleurs pour l'affichage
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Fonction pour afficher les messages
print_info() {
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

# Vérifier que BCRDF est compilé
if [ ! -f "./bcrdf" ]; then
    print_error "BCRDF n'est pas compilé. Exécutez 'make build' d'abord."
    exit 1
fi

# Créer les répertoires de test
print_info "Création des répertoires de test..."
mkdir -p test-data
mkdir -p restored-data

# Créer des fichiers de test
print_info "Création de fichiers de test..."
echo "Contenu du fichier 1" > test-data/file1.txt
echo "Contenu du fichier 2" > test-data/file2.txt
echo "Contenu du fichier 3" > test-data/file3.txt

# Créer un sous-répertoire
mkdir -p test-data/subdir
echo "Contenu du fichier dans le sous-répertoire" > test-data/subdir/file4.txt

print_success "Fichiers de test créés"

# Initialiser la configuration si elle n'existe pas
if [ ! -f "config.yaml" ]; then
    print_info "Initialisation de la configuration..."
    cp configs/config.yaml config.yaml
    print_warning "⚠️  Veuillez configurer vos paramètres S3 et votre clé de chiffrement dans config.yaml"
    print_warning "Pour cette démo, nous utiliserons des valeurs par défaut"
fi

echo ""
print_info "Étape 1: Sauvegarde initiale"
echo "----------------------------------"

# Effectuer la première sauvegarde
print_info "Exécution de la sauvegarde initiale..."
./bcrdf backup --source ./test-data --name "demo-backup-1"

if [ $? -eq 0 ]; then
    print_success "Sauvegarde initiale terminée"
else
    print_error "Échec de la sauvegarde initiale"
    exit 1
fi

echo ""
print_info "Étape 2: Modification des données"
echo "-------------------------------------"

# Modifier un fichier existant
echo "Contenu modifié du fichier 1" > test-data/file1.txt

# Ajouter un nouveau fichier
echo "Nouveau fichier ajouté" > test-data/file5.txt

# Supprimer un fichier
rm test-data/file2.txt

print_success "Données modifiées"

echo ""
print_info "Étape 3: Sauvegarde incrémentale"
echo "-------------------------------------"

# Effectuer une sauvegarde incrémentale
print_info "Exécution de la sauvegarde incrémentale..."
./bcrdf backup --source ./test-data --name "demo-backup-2"

if [ $? -eq 0 ]; then
    print_success "Sauvegarde incrémentale terminée"
else
    print_error "Échec de la sauvegarde incrémentale"
    exit 1
fi

echo ""
print_info "Étape 4: Liste des sauvegardes"
echo "-----------------------------------"

# Lister les sauvegardes disponibles
print_info "Affichage des sauvegardes disponibles..."
./bcrdf list

echo ""
print_info "Étape 5: Restauration"
echo "-------------------------"

# Restaurer la première sauvegarde
print_info "Restauration de la première sauvegarde..."
./bcrdf restore --backup-id "demo-backup-1-$(date +%Y%m%d)-$(date +%H%M%S | cut -c1-4)" --destination ./restored-data

if [ $? -eq 0 ]; then
    print_success "Restauration terminée"
    
    echo ""
    print_info "Contenu du répertoire restauré:"
    ls -la ./restored-data/
    
    echo ""
    print_info "Comparaison des fichiers:"
    echo "Fichier original vs restauré:"
    echo "file1.txt:"
    echo "  Original: $(cat test-data/file1.txt 2>/dev/null || echo 'fichier supprimé')"
    echo "  Restauré: $(cat restored-data/test-data/file1.txt 2>/dev/null || echo 'fichier non trouvé')"
else
    print_error "Échec de la restauration"
fi

echo ""
print_info "Étape 6: Nettoyage"
echo "----------------------"

# Nettoyer les fichiers de test
print_info "Suppression des fichiers de test..."
rm -rf test-data restored-data

print_success "Nettoyage terminé"

echo ""
print_success "🎉 Démonstration terminée avec succès!"
echo ""
echo "Résumé de ce qui a été démontré:"
echo "✅ Sauvegarde initiale avec création d'index"
echo "✅ Sauvegarde incrémentale (seuls les changements)"
echo "✅ Liste des sauvegardes disponibles"
echo "✅ Restauration à partir d'un index"
echo "✅ Gestion des fichiers ajoutés, modifiés et supprimés"
echo ""
echo "Avantages de l'approche index-based:"
echo "🔹 Efficacité: Seuls les fichiers modifiés sont sauvegardés"
echo "🔹 Performance: Traitement parallèle et compression"
echo "🔹 Sécurité: Chiffrement AES-256 et checksums"
echo "🔹 Flexibilité: Restauration précise à des points dans le temps"
echo ""
echo "Pour plus d'informations, consultez:"
echo "📖 README.md - Guide d'utilisation"
echo "📖 docs/ARCHITECTURE.md - Documentation technique"
echo "🔧 Makefile - Commandes de développement" 
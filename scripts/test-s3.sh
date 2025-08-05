#!/bin/bash

# Script de test pour vérifier la connexion S3
# Ce script teste la connectivité et les permissions S3

set -e

echo "🧪 Test de Connexion S3 pour BCRDF"
echo "==================================="
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

# Vérifier que le fichier de configuration existe
if [ ! -f "config.yaml" ]; then
    print_error "Fichier config.yaml non trouvé. Exécutez 'make init-config' d'abord."
    exit 1
fi

print_info "Test 1: Vérification de la configuration"
echo "--------------------------------------------"

# Extraire les paramètres S3 du fichier de configuration
BUCKET=$(grep -A 5 "storage:" config.yaml | grep "bucket:" | awk '{print $2}' | tr -d '"')
REGION=$(grep -A 5 "storage:" config.yaml | grep "region:" | awk '{print $2}' | tr -d '"')
ENDPOINT=$(grep -A 5 "storage:" config.yaml | grep "endpoint:" | awk '{print $2}' | tr -d '"')

echo "Bucket: $BUCKET"
echo "Région: $REGION"
echo "Endpoint: $ENDPOINT"

if [ "$BUCKET" = "bcrdf-backups" ] || [ "$BUCKET" = "YOUR_BUCKET_NAME" ]; then
    print_warning "⚠️  Le bucket semble être la valeur par défaut. Vérifiez votre configuration."
else
    print_success "✅ Configuration bucket détectée"
fi

print_info "Test 2: Test de sauvegarde simple"
echo "-------------------------------------"

# Créer un fichier de test
mkdir -p test-s3
echo "Test S3 - $(date)" > test-s3/test-file.txt

# Effectuer une sauvegarde de test
print_info "Exécution d'une sauvegarde de test..."
if ./bcrdf backup --source ./test-s3 --name "s3-test"; then
    print_success "✅ Sauvegarde de test réussie"
else
    print_error "❌ Échec de la sauvegarde de test"
    print_info "Vérifiez vos paramètres S3 dans config.yaml"
    exit 1
fi

print_info "Test 3: Liste des sauvegardes"
echo "--------------------------------"

# Lister les sauvegardes
if ./bcrdf list; then
    print_success "✅ Liste des sauvegardes réussie"
else
    print_error "❌ Échec de la liste des sauvegardes"
fi

print_info "Test 4: Nettoyage"
echo "-------------------"

# Supprimer les fichiers de test
rm -rf test-s3

print_success "🧹 Nettoyage terminé"

echo ""
print_success "🎉 Tests S3 terminés avec succès!"
echo ""
echo "Si tous les tests ont réussi, votre configuration S3 est correcte."
echo ""
echo "Prochaines étapes :"
echo "1. Configurez vos paramètres S3 réels dans config.yaml"
echo "2. Testez avec vos vraies données"
echo "3. Consultez docs/SETUP.md pour plus d'informations"
echo ""
echo "Pour plus d'aide :"
echo "📖 docs/SETUP.md - Guide de configuration S3"
echo "📖 docs/ARCHITECTURE.md - Documentation technique" 
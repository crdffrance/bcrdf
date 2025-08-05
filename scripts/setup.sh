#!/bin/bash

# Script d'installation et de configuration BCRDF
# Usage: ./scripts/setup.sh

set -e

echo "🚀 Installation et configuration de BCRDF"
echo "========================================"

# Vérifier que Go est installé
if ! command -v go &> /dev/null; then
    echo "❌ Go n'est pas installé. Veuillez installer Go 1.21+"
    exit 1
fi

echo "✅ Go détecté: $(go version)"

# Compiler le projet
echo "🔨 Compilation du projet..."
go build -o bcrdf cmd/bcrdf/main.go

if [ $? -eq 0 ]; then
    echo "✅ Compilation réussie"
else
    echo "❌ Erreur lors de la compilation"
    exit 1
fi

# Créer la configuration si elle n'existe pas
if [ ! -f config.yaml ]; then
    echo "📝 Création du fichier de configuration..."
    cp configs/config.example.yaml config.yaml
    echo "✅ Configuration créée: config.yaml"
    echo "⚠️  Veuillez configurer vos paramètres S3 et votre clé de chiffrement"
else
    echo "✅ Configuration existante détectée"
fi

# Vérifier les tests
echo "🧪 Exécution des tests..."
go test ./...

if [ $? -eq 0 ]; then
    echo "✅ Tous les tests passent"
else
    echo "❌ Certains tests ont échoué"
    exit 1
fi

echo ""
echo "🎉 Installation terminée !"
echo ""
echo "📋 Prochaines étapes:"
echo "1. Configurez config.yaml avec vos paramètres S3"
echo "2. Testez avec: ./bcrdf info"
echo "3. Créez votre première sauvegarde: ./bcrdf backup -n test -s /chemin/vers/donnees"
echo ""
echo "📚 Documentation: README.md" 
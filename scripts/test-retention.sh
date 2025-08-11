#!/bin/bash

# Script de test pour la rétention BCRDF
# Teste le comportement de la rétention avec différents noms de backup

set -e

echo "🧪 Test de la rétention BCRDF"
echo "=============================="

# Vérifier que bcrdf est installé
if ! command -v bcrdf &> /dev/null; then
    echo "❌ BCRDF n'est pas installé ou pas dans le PATH"
    exit 1
fi

echo "✅ BCRDF trouvé: $(bcrdf --version 2>/dev/null || echo 'version inconnue')"
echo ""

# Test 1: Vérifier la configuration de rétention
echo "📋 Test 1: Configuration de rétention"
echo "-------------------------------------"
if [ -f "config.yaml" ]; then
    echo "Configuration trouvée:"
    grep -A 5 "retention:" config.yaml || echo "   Aucune section retention trouvée"
else
    echo "❌ Aucun fichier config.yaml trouvé"
    echo "   Copiez une configuration depuis configs/ et modifiez-la"
    exit 1
fi
echo ""

# Test 2: Lister les backups existants
echo "📋 Test 2: Backups existants"
echo "-----------------------------"
echo "Backups dans le stockage:"
bcrdf list 2>/dev/null || echo "   Aucun backup trouvé ou erreur de connexion"
echo ""

# Test 3: Tester la rétention en mode verbose
echo "📋 Test 3: Test de rétention (mode verbose)"
echo "-------------------------------------------"
echo "Test de rétention pour le backup 'home':"
bcrdf retention --info --verbose 2>/dev/null || echo "   Erreur lors du test de rétention"
echo ""

# Test 4: Vérifier les logs de rétention
echo "📋 Test 4: Logs de rétention"
echo "-----------------------------"
if [ -f "bcrdf.log" ]; then
    echo "Dernières lignes du log:"
    tail -10 bcrdf.log | grep -E "(retention|deletion|backup)" || echo "   Aucun log de rétention trouvé"
else
    echo "Aucun fichier de log trouvé"
fi
echo ""

echo "🎯 Recommandations:"
echo "=================="
echo "1. Vérifiez que votre config.yaml a les bonnes valeurs de rétention"
echo "2. Assurez-vous que les noms de backup sont cohérents"
echo "3. Testez avec --verbose pour voir le comportement détaillé"
echo "4. Si le problème persiste, augmentez temporairement les limites:"
echo "   retention:"
echo "     days: 90        # Au lieu de 30"
echo "     max_backups: 20 # Au lieu de 10"
echo ""

echo "✅ Test terminé"

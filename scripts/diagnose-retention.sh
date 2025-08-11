#!/bin/bash

# Script de diagnostic pour le problème de rétention BCRDF
# Analyse pourquoi des backups récents sont supprimés

set -e

echo "🔍 Diagnostic du problème de rétention BCRDF"
echo "============================================"

# Couleurs pour l'affichage
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Fonction pour afficher les informations
print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Vérifier que bcrdf est installé
if ! command -v bcrdf &> /dev/null; then
    print_error "BCRDF n'est pas installé ou pas dans le PATH"
    exit 1
fi

print_success "BCRDF trouvé: $(bcrdf --version 2>/dev/null || echo 'version inconnue')"
echo ""

# Étape 1: Vérifier la configuration
print_info "Étape 1: Analyse de la configuration"
echo "----------------------------------------"

if [ -f "config.yaml" ]; then
    print_success "Fichier config.yaml trouvé"
    
    # Extraire les paramètres de rétention
    retention_days=$(grep -A 1 "retention:" config.yaml | grep "days:" | awk '{print $2}' | head -1)
    retention_max=$(grep -A 1 "retention:" config.yaml | grep "max_backups:" | awk '{print $2}' | head -1)
    
    if [ -n "$retention_days" ]; then
        print_info "Rétention configurée: $retention_days jours, $retention_max backups max"
        
        if [ "$retention_days" -lt 30 ]; then
            print_warning "⚠️  Rétention très courte ($retention_days jours) - risque de suppression rapide"
        fi
        
        if [ "$retention_max" -lt 10 ]; then
            print_warning "⚠️  Nombre max de backups très limité ($retention_max) - risque de suppression fréquente"
        fi
    else
        print_error "❌ Paramètres de rétention non trouvés dans config.yaml"
    fi
else
    print_error "❌ Aucun fichier config.yaml trouvé"
    echo "   Copiez une configuration depuis configs/ et modifiez-la"
    exit 1
fi
echo ""

# Étape 2: Analyser les backups existants
print_info "Étape 2: Analyse des backups existants"
echo "-------------------------------------------"

echo "Liste des backups dans le stockage:"
backup_list=$(bcrdf list 2>/dev/null || echo "ERREUR")

if [ "$backup_list" = "ERREUR" ]; then
    print_error "❌ Impossible de lister les backups - vérifiez la connexion au stockage"
else
    if [ -n "$backup_list" ]; then
        echo "$backup_list"
        
        # Compter les backups par nom
        echo ""
        print_info "Analyse des noms de backup:"
        echo "$backup_list" | grep -E "^[a-zA-Z]" | while read -r line; do
            if [[ $line =~ ^([a-zA-Z0-9._-]+)-([0-9]{8})-([0-9]{6}) ]]; then
                backup_name="${BASH_REMATCH[1]}"
                backup_date="${BASH_REMATCH[2]}"
                backup_time="${BASH_REMATCH[3]}"
                
                # Convertir la date en timestamp
                backup_timestamp=$(date -d "${backup_date} ${backup_time}" +%s 2>/dev/null || echo "0")
                current_timestamp=$(date +%s)
                age_hours=$(( (current_timestamp - backup_timestamp) / 3600 ))
                
                if [ $age_hours -lt 24 ]; then
                    print_success "   $backup_name: $backup_date $backup_time (récent: ${age_hours}h)"
                elif [ $age_hours -lt 168 ]; then
                    print_info "   $backup_name: $backup_date $backup_time (${age_hours}h)"
                else
                    print_warning "   $backup_name: $backup_date $backup_time (ancien: ${age_hours}h)"
                fi
            fi
        done
    else
        print_info "Aucun backup trouvé dans le stockage"
    fi
fi
echo ""

# Étape 3: Test de rétention en mode verbose
print_info "Étape 3: Test de rétention (mode verbose)"
echo "----------------------------------------------"

echo "Test de rétention pour le backup 'home':"
retention_test=$(bcrdf retention --info --verbose 2>/dev/null || echo "ERREUR_RETENTION")

if [ "$retention_test" = "ERREUR_RETENTION" ]; then
    print_error "❌ Erreur lors du test de rétention"
else
    echo "$retention_test"
    
    # Analyser les résultats
    if echo "$retention_test" | grep -q "deletion"; then
        print_warning "⚠️  Des backups sont marqués pour suppression"
        
        # Extraire les backups à supprimer
        echo ""
        print_info "Backups marqués pour suppression:"
        echo "$retention_test" | grep -E "(Deleting|Marking|deletion)" | while read -r line; do
            print_warning "   $line"
        done
    else
        print_success "✅ Aucun backup marqué pour suppression"
    fi
fi
echo ""

# Étape 4: Recommandations
print_info "Étape 4: Recommandations"
echo "----------------------------"

echo "🎯 Actions recommandées:"
echo ""

if [ -n "$retention_days" ] && [ "$retention_days" -lt 30 ]; then
    echo "1. 🔧 Augmenter la rétention dans config.yaml:"
    echo "   retention:"
    echo "     days: 90        # Au lieu de $retention_days"
    echo "     max_backups: 20 # Au lieu de $retention_max"
    echo ""
fi

echo "2. 🔍 Tester avec la configuration de test:"
echo "   cp configs/config-test-retention.yaml config.yaml"
echo "   ./scripts/test-retention.sh"
echo ""

echo "3. 📝 Vérifier les logs détaillés:"
echo "   bcrdf retention --info --verbose"
echo ""

echo "4. 🚫 Désactiver temporairement la rétention automatique:"
echo "   # Commentez ou supprimez la section retention dans config.yaml"
echo ""

echo "5. 🔄 Recompiler avec les corrections:"
echo "   make build"
echo ""

print_success "Diagnostic terminé"
echo ""
print_info "Pour plus d'aide, consultez la documentation ou créez une issue sur GitHub"

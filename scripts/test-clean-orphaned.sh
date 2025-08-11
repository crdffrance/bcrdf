#!/bin/bash

# Script de test simple pour la fonction clean --all
# Teste la détection des fichiers orphelins

set -e

print_status() {
    echo -e "\033[0;34m[INFO]\033[0m $1"
}

print_success() {
    echo -e "\033[0;32m[SUCCESS]\033[0m $1"
}

print_warning() {
    echo -e "\033[1;33m[WARNING]\033[0m $1"
}

print_error() {
    echo -e "\033[0;31m[ERROR]\033[0m $1"
}

print_status "Test de la fonction clean --all avec détection des fichiers orphelins"

# Test 1: Analyse en mode dry-run
print_status "Test 1: Analyse du nettoyage complet (dry-run)"
echo "y" | ./bcrdf clean --all --dry-run --verbose

if [ $? -eq 0 ]; then
    print_success "Analyse du nettoyage complet réussie"
else
    print_error "Échec de l'analyse du nettoyage complet"
    exit 1
fi

echo ""
print_status "Test 2: Analyse avec suppression des orphelines (dry-run)"
echo "y" | ./bcrdf clean --all --remove-orphaned --dry-run --verbose

if [ $? -eq 0 ]; then
    print_success "Analyse avec suppression des orphelines réussie"
else
    print_error "Échec de l'analyse avec suppression des orphelines"
    exit 1
fi

print_success "Tous les tests de nettoyage ont réussi !"
print_status "La fonction clean --all fonctionne correctement"

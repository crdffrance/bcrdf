# Makefile pour BCRDF - Système de sauvegarde index-based
# Usage: make [target]

# Variables
BINARY_NAME=bcrdf
BUILD_DIR=build
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.Version=${VERSION}"

# Couleurs pour les messages
GREEN=\033[0;32m
YELLOW=\033[1;33m
RED=\033[0;31m
NC=\033[0m # No Color

.PHONY: help build clean test install dev-build lint format init-config example-backup example-restore example-list setup clean-all build-all build-release

# Cible par défaut
all: build

# Afficher l'aide
help:
	@echo "$(GREEN)BCRDF - Système de sauvegarde index-based$(NC)"
	@echo ""
	@echo "$(YELLOW)Commandes disponibles:$(NC)"
	@echo "  build          - Compiler le projet"
	@echo "  clean          - Nettoyer les fichiers temporaires"
	@echo "  test           - Exécuter les tests"
	@echo "  install        - Installer BCRDF"
	@echo "  dev-build      - Compilation rapide pour développement"
	@echo "  lint           - Vérifier le code avec golangci-lint"
	@echo "  format         - Formater le code avec gofmt"
	@echo "  init-config    - Initialiser la configuration"
	@echo "  setup          - Installation complète"
	@echo "  clean-all      - Nettoyage complet"
	@echo "  example-*      - Exemples d'utilisation"
	@echo "  build-all      - Compilation multi-architectures"
	@echo "  build-release  - Compilation pour release"

# Compiler le projet
build:
	@echo "$(GREEN)🔨 Compilation de BCRDF...$(NC)"
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) cmd/bcrdf/main.go
	@echo "$(GREEN)✅ Compilation réussie: $(BUILD_DIR)/$(BINARY_NAME)$(NC)"

# Compilation rapide pour développement
dev-build:
	@echo "$(YELLOW)⚡ Compilation rapide...$(NC)"
	go build -o $(BINARY_NAME) cmd/bcrdf/main.go
	@echo "$(GREEN)✅ Binaire créé: $(BINARY_NAME)$(NC)"

# Nettoyer les fichiers temporaires
clean:
	@echo "$(YELLOW)🧹 Nettoyage...$(NC)"
	rm -f $(BINARY_NAME)
	rm -f *.exe
	rm -f *.test
	rm -rf restored-*/
	rm -f *.log
	rm -f *.tmp
	@echo "$(GREEN)✅ Nettoyage terminé$(NC)"

# Nettoyage complet
clean-all: clean
	@echo "$(YELLOW)🧹 Nettoyage complet...$(NC)"
	rm -rf $(BUILD_DIR)
	rm -f coverage.txt
	rm -f coverage.html
	go clean -cache
	go clean -modcache
	@echo "$(GREEN)✅ Nettoyage complet terminé$(NC)"

# Exécuter les tests
test:
	@echo "$(YELLOW)🧪 Exécution des tests...$(NC)"
	go test -v ./...
	@echo "$(GREEN)✅ Tests terminés$(NC)"

# Tests avec couverture
test-coverage:
	@echo "$(YELLOW)🧪 Tests avec couverture...$(NC)"
	go test -v -coverprofile=coverage.txt ./...
	go tool cover -html=coverage.txt -o coverage.html
	@echo "$(GREEN)✅ Rapport de couverture généré: coverage.html$(NC)"

# Installer BCRDF
install: build
	@echo "$(YELLOW)📦 Installation...$(NC)"
	cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@echo "$(GREEN)✅ BCRDF installé dans /usr/local/bin/$(NC)"

# Vérifier le code avec golangci-lint
lint:
	@echo "$(YELLOW)🔍 Vérification du code...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "$(RED)⚠️  golangci-lint non installé. Installation...$(NC)"; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
		golangci-lint run; \
	fi
	@echo "$(GREEN)✅ Vérification terminée$(NC)"

# Formater le code
format:
	@echo "$(YELLOW)🎨 Formatage du code...$(NC)"
	go fmt ./...
	@echo "$(GREEN)✅ Formatage terminé$(NC)"

# Initialiser la configuration
init-config:
	@echo "$(YELLOW)📝 Initialisation de la configuration...$(NC)"
	@if [ ! -f config.yaml ]; then \
		cp configs/config.example.yaml config.yaml; \
		echo "$(GREEN)✅ Configuration créée: config.yaml$(NC)"; \
		echo "$(YELLOW)⚠️  Veuillez configurer vos paramètres S3 et votre clé de chiffrement$(NC)"; \
	else \
		echo "$(GREEN)✅ Configuration existante détectée$(NC)"; \
	fi

# Installation complète
setup: clean-all build test init-config
	@echo "$(GREEN)🎉 Installation BCRDF terminée !$(NC)"
	@echo ""
	@echo "$(YELLOW)📋 Prochaines étapes:$(NC)"
	@echo "1. Configurez config.yaml avec vos paramètres S3"
	@echo "2. Testez avec: ./$(BINARY_NAME) info"
	@echo "3. Créez votre première sauvegarde: ./$(BINARY_NAME) backup -n test -s /chemin/vers/donnees"

# Exemples d'utilisation
example-backup:
	@echo "$(YELLOW)📋 Exemple de sauvegarde:$(NC)"
	@echo "./$(BINARY_NAME) backup -n ma-sauvegarde -s /chemin/vers/donnees -c config.yaml -v"

example-restore:
	@echo "$(YELLOW)📋 Exemple de restauration:$(NC)"
	@echo "./$(BINARY_NAME) restore --backup-id backup-id --destination ./restored -c config.yaml -v"

example-list:
	@echo "$(YELLOW)📋 Exemple de liste:$(NC)"
	@echo "./$(BINARY_NAME) list -c config.yaml -v"

# Compilation multi-architectures
build-all:
	@echo "$(YELLOW)🔨 Compilation multi-architectures...$(NC)"
	./scripts/build-all.sh
	@echo "$(GREEN)✅ Compilation multi-architectures terminée$(NC)"

# Compilation pour release
build-release:
	@echo "$(YELLOW)🔨 Compilation pour release...$(NC)"
	@if [ -z "$(TAG)" ]; then \
		echo "$(RED)❌ Spécifiez un tag: make build-release TAG=v1.0.0$(NC)"; \
		exit 1; \
	fi
	./scripts/build-all.sh $(TAG)
	@echo "$(GREEN)✅ Release $(TAG) compilée$(NC)"

# Afficher les informations du projet
info:
	@echo "$(GREEN)BCRDF - Système de sauvegarde index-based$(NC)"
	@echo "Version: $(VERSION)"
	@echo "Go version: $(shell go version)"
	@echo "Architecture: $(shell go env GOOS)/$(shell go env GOARCH)" 
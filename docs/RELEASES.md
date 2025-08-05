# Releases et CI/CD BCRDF

## 🚀 Système de CI/CD

BCRDF utilise GitHub Actions pour automatiser la compilation, les tests et les releases sur toutes les architectures supportées.

### 📋 Workflows disponibles

#### 1. Build (`build.yml`)
- **Déclencheurs** : Push sur main/master, Pull Requests, Releases
- **Fonctionnalités** :
  - Tests unitaires et linting
  - Compilation multi-architectures
  - Création automatique de releases

#### 2. Security & Quality (`security.yml`)
- **Déclencheurs** : Push, PR, Schedule (hebdomadaire)
- **Fonctionnalités** :
  - Scan de sécurité avec gosec
  - Vérification des dépendances
  - Analyse de qualité du code
  - Vérification des licences

#### 3. Integration Tests (`integration.yml`)
- **Déclencheurs** : Push, PR
- **Fonctionnalités** :
  - Tests d'intégration avec MinIO (S3-compatible)
  - Vérification complète du workflow backup/restore

#### 4. Release (`release.yml`)
- **Déclencheurs** : Tags v*
- **Fonctionnalités** :
  - Compilation automatique pour toutes les architectures
  - Création de release GitHub avec binaries

## 🏗️ Architectures supportées

### Linux
- **x64** : `bcrdf-linux-x64.tar.gz`
- **ARM64** : `bcrdf-linux-arm64.tar.gz`
- **x32** : `bcrdf-linux-x32.tar.gz`

### Windows
- **x64** : `bcrdf-windows-x64.zip`
- **ARM64** : `bcrdf-windows-arm64.zip`
- **x32** : `bcrdf-windows-x32.zip`

### macOS
- **x64** : `bcrdf-darwin-x64.tar.gz`
- **ARM64** : `bcrdf-darwin-arm64.tar.gz`

## 📦 Compilation locale

### Compilation multi-architectures
```bash
# Compilation pour toutes les architectures
make build-all

# Ou directement
./scripts/build-all.sh
```

### Compilation pour release
```bash
# Compilation avec version spécifique
make build-release TAG=v1.0.0

# Ou directement
./scripts/build-all.sh v1.0.0
```

### Compilation manuelle
```bash
# Linux x64
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bcrdf-linux-x64 cmd/bcrdf/main.go

# Windows x64
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o bcrdf-windows-x64.exe cmd/bcrdf/main.go

# macOS ARM64
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o bcrdf-darwin-arm64 cmd/bcrdf/main.go
```

## 🏷️ Création de releases

### Release automatique (recommandé)
```bash
# 1. Créer un tag
git tag v1.0.0

# 2. Pousser le tag
git push origin v1.0.0

# 3. GitHub Actions compile et crée automatiquement la release
```

### Release manuelle
```bash
# 1. Compiler pour toutes les architectures
make build-release TAG=v1.0.0

# 2. Créer une release GitHub manuellement
# 3. Uploader les fichiers depuis dist/
```

## 🔧 Configuration des workflows

### Variables d'environnement
Les workflows utilisent les variables d'environnement suivantes :
- `GO_VERSION` : Version de Go (défaut: 1.21)
- `CGO_ENABLED` : Désactivé pour cross-compilation

### Cache
- **Go modules** : Cache des dépendances
- **Build cache** : Cache de compilation
- **Dépendances** : Cache des outils (golangci-lint, etc.)

## 📊 Métriques de build

### Temps de compilation (approximatif)
- **Linux x64** : ~30s
- **Windows x64** : ~35s
- **macOS ARM64** : ~40s
- **Total multi-arch** : ~5-10 minutes

### Taille des binaires (approximative)
- **Linux x64** : ~10MB
- **Windows x64** : ~10MB
- **macOS ARM64** : ~9MB

## 🧪 Tests automatisés

### Tests unitaires
```bash
# Exécution locale
make test

# Avec couverture
make test-coverage
```

### Tests d'intégration
- **MinIO** : Serveur S3-compatible pour les tests
- **Workflow complet** : Backup → List → Restore → Verify
- **Multi-algorithmes** : Tests AES-256-GCM et XChaCha20-Poly1305

### Tests de sécurité
- **gosec** : Scan de vulnérabilités
- **Dépendances** : Vérification des vulnérabilités connues
- **Licences** : Vérification des headers de licence

## 🔍 Monitoring des builds

### Badges disponibles
```markdown
[![Build Status](https://github.com/username/bcrdf/workflows/Build/badge.svg)](https://github.com/username/bcrdf/actions)
[![Security](https://github.com/username/bcrdf/workflows/Security%20&%20Quality/badge.svg)](https://github.com/username/bcrdf/actions)
[![Integration Tests](https://github.com/username/bcrdf/workflows/Integration%20Tests/badge.svg)](https://github.com/username/bcrdf/actions)
```

### Artifacts générés
- **Binaries** : Toutes les architectures
- **Coverage** : Rapport de couverture HTML
- **SARIF** : Rapports de sécurité

## 🚨 Dépannage

### Erreurs courantes

#### Build échoue
```bash
# Vérifier les dépendances
go mod tidy
go mod download

# Vérifier la syntaxe
make lint
```

#### Cross-compilation échoue
```bash
# Vérifier les variables d'environnement
echo $GOOS $GOARCH $CGO_ENABLED

# Recompiler avec CGO désactivé
CGO_ENABLED=0 go build ...
```

#### Tests d'intégration échouent
```bash
# Vérifier MinIO
docker ps | grep minio

# Redémarrer MinIO
docker restart minio
```

### Logs et debugging
- **Actions GitHub** : Logs détaillés dans chaque workflow
- **Artifacts** : Téléchargement des binaires et rapports
- **Notifications** : Email/Slack pour les échecs

## 📈 Améliorations futures

### Fonctionnalités planifiées
- [ ] **Signing** : Signature des binaires avec GPG
- [ ] **Docker** : Images Docker multi-architectures
- [ ] **Homebrew** : Formula pour macOS
- [ ] **Snap** : Package Snap pour Linux
- [ ] **Chocolatey** : Package pour Windows

### Optimisations
- [ ] **Cache** : Amélioration du cache de compilation
- [ ] **Parallélisation** : Builds parallèles par architecture
- [ ] **Compression** : Optimisation de la taille des binaires
- [ ] **CDN** : Distribution via CDN pour les downloads

## 📚 Ressources

- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Go Cross Compilation](https://golang.org/doc/install/source#environment)
- [MinIO Documentation](https://docs.min.io/)
- [gosec Security Scanner](https://github.com/securecodewarrior/gosec) 
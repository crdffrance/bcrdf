# BCRDF - Système de sauvegarde index-based

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Build Status](https://github.com/username/bcrdf/workflows/Build/badge.svg)](https://github.com/username/bcrdf/actions)
[![Security](https://github.com/username/bcrdf/workflows/Security%20&%20Quality/badge.svg)](https://github.com/username/bcrdf/actions)
[![Integration Tests](https://github.com/username/bcrdf/workflows/Integration%20Tests/badge.svg)](https://github.com/username/bcrdf/actions)

BCRDF (Backup Cloud Ready Data Format) est un système de sauvegarde moderne qui utilise une approche index-based pour optimiser le stockage et les performances. Conçu pour offrir une efficacité maximale avec une sécurité de niveau militaire.

## 🚀 Fonctionnalités

### ✨ Caractéristiques principales
- **Sauvegarde incrémentale** : Seuls les changements sont sauvegardés
- **Chiffrement multi-algorithmes** : AES-256-GCM et XChaCha20-Poly1305
- **Compression GZIP** : Réduction significative de l'espace
- **Déduplication** : Basée sur les checksums SHA-256
- **Stockage S3** : Compatible avec tous les providers S3
- **Restauration précise** : À des points spécifiques dans le temps
- **Performance optimale** : Traitement parallèle et streaming

### 🔐 Sécurité
- **Chiffrement de bout en bout** : AES-256-GCM ou XChaCha20-Poly1305
- **Authentification** : Tags d'intégrité inclus
- **Clés sécurisées** : Génération cryptographiquement sécurisée
- **Transport sécurisé** : TLS 1.2+ pour toutes les communications

### 📊 Performance
- **Sauvegarde complète** : ~10s pour 111MB
- **Sauvegarde incrémentale** : ~1s quand aucun changement
- **Traitement parallèle** : Workers configurables
- **Streaming** : Gestion efficace de la mémoire

## 📦 Installation

### Prérequis
- Go 1.21 ou supérieur
- Compte S3 (AWS, Scaleway, OVH, etc.)

### Installation rapide
```bash
# Cloner le projet
git clone https://github.com/votre-username/bcrdf.git
cd bcrdf

# Installation complète
make setup

# Ou installation manuelle
go build -o bcrdf cmd/bcrdf/main.go
```

### Configuration
```bash
# Copier le fichier de configuration
cp configs/config.example.yaml config.yaml

# Éditer la configuration
nano config.yaml
```

## ⚙️ Configuration

### Fichier config.yaml
```yaml
storage:
  type: "s3"
  bucket: "mon-bucket-sauvegarde"
  region: "eu-west-3"
  endpoint: "https://s3.eu-west-3.amazonaws.com"
  access_key: "VOTRE_CLE_ACCESS"
  secret_key: "VOTRE_CLE_SECRETE"

backup:
  source_path: "/chemin/vers/sauvegarde"
  encryption_key: "VOTRE_CLE_CHIFFREMENT_32_BYTES"
  encryption_algo: "aes-256-gcm"  # ou "xchacha20-poly1305"
  compression_level: 3  # Niveau GZIP (1-9)
  max_workers: 10  # Nombre de workers parallèles

retention:
  days: 30  # Durée de rétention en jours
  max_backups: 10  # Nombre maximum de sauvegardes
```

### Variables d'environnement
```bash
export BCRDF_S3_ACCESS_KEY="votre-cle-access"
export BCRDF_S3_SECRET_KEY="votre-cle-secrete"
export BCRDF_ENCRYPTION_KEY="votre-cle-chiffrement"
```

## 🛠️ Utilisation

### Commandes principales

#### Sauvegarde
```bash
# Sauvegarde complète
./bcrdf backup -n ma-sauvegarde -s /chemin/vers/donnees -c config.yaml -v

# Sauvegarde avec algorithme spécifique
./bcrdf backup -n test-xchacha -s /chemin/vers/donnees -c config.yaml -v
```

#### Restauration
```bash
# Restauration complète
./bcrdf restore --backup-id backup-20241206-143022 --destination ./restored -c config.yaml -v

# Restauration d'un fichier spécifique
./bcrdf restore --backup-id backup-20241206-143022 --file /chemin/vers/fichier --destination ./restored -c config.yaml -v
```

#### Gestion des sauvegardes
```bash
# Liste des sauvegardes
./bcrdf list -c config.yaml -v

# Détails d'une sauvegarde
./bcrdf list backup-20241206-143022 -c config.yaml -v

# Suppression d'une sauvegarde
./bcrdf delete --backup-id backup-20241206-143022 -c config.yaml -v
```

#### Informations
```bash
# Informations sur les algorithmes
./bcrdf info
```

### Exemples d'utilisation

#### Première sauvegarde
```bash
# Configuration
cp configs/config.example.yaml config.yaml
# Éditer config.yaml avec vos paramètres S3

# Test de connexion
./bcrdf info

# Première sauvegarde
./bcrdf backup -n premiere-sauvegarde -s /home/user/documents -c config.yaml -v
```

#### Sauvegarde régulière
```bash
# Sauvegarde incrémentale (seuls les changements)
./bcrdf backup -n sauvegarde-quotidienne -s /home/user/documents -c config.yaml -v
```

#### Restauration
```bash
# Restauration complète
./bcrdf restore --backup-id premiere-sauvegarde-20241206-143022 --destination ./restored -c config.yaml -v

# Vérification
ls -la ./restored/
```

## 🔧 Développement

### Structure du projet
```
bcrdf/
├── cmd/bcrdf/          # Point d'entrée CLI
├── internal/           # Logique métier
│   ├── backup/        # Gestionnaire de sauvegarde
│   ├── restore/       # Gestionnaire de restauration
│   ├── index/         # Gestionnaire d'index
│   ├── crypto/        # Chiffrement multi-algorithmes
│   └── compression/   # Compression GZIP
├── pkg/               # Utilitaires partagés
│   ├── utils/         # Utilitaires généraux
│   └── s3/           # Client S3
├── configs/           # Fichiers de configuration
├── docs/             # Documentation
├── scripts/          # Scripts utilitaires
└── examples/         # Exemples d'utilisation
```

### Commandes de développement
```bash
# Compilation
make build

# Tests
make test

# Linting
make lint

# Formatage
make format

# Nettoyage
make clean

# Installation complète
make setup
```

### Tests
```bash
# Tests unitaires
go test ./...

# Tests avec couverture
make test-coverage

# Tests spécifiques
go test ./internal/backup/...
```

## 📚 Documentation

- [Architecture](docs/ARCHITECTURE.md) - Architecture détaillée du système
- [Configuration S3](docs/SETUP.md) - Guide de configuration S3
- [Intégration S3](docs/S3_INTEGRATION.md) - Détails de l'intégration S3
- [Exemples](docs/EXAMPLES.md) - Exemples d'utilisation avancés
- [Releases](docs/RELEASES.md) - CI/CD et releases automatiques

## 🤝 Contribution

### Prérequis pour contribuer
- Go 1.21+
- Connaissance de Git
- Tests pour les nouvelles fonctionnalités

### Processus de contribution
1. Fork le projet
2. Créer une branche feature (`git checkout -b feature/nouvelle-fonctionnalite`)
3. Commit les changements (`git commit -am 'Ajout nouvelle fonctionnalité'`)
4. Push vers la branche (`git push origin feature/nouvelle-fonctionnalite`)
5. Créer une Pull Request

### Standards de code
- Formatage avec `gofmt`
- Tests pour toutes les nouvelles fonctionnalités
- Documentation des fonctions publiques
- Linting avec `golangci-lint`

## 📄 Licence

Ce projet est sous licence MIT. Voir le fichier [LICENSE](LICENSE) pour plus de détails.

## 🙏 Remerciements

- [Cobra](https://github.com/spf13/cobra) - Framework CLI
- [Viper](https://github.com/spf13/viper) - Gestion de configuration
- [AWS SDK for Go](https://github.com/aws/aws-sdk-go) - Client S3
- [golang.org/x/crypto](https://golang.org/x/crypto) - Algorithmes de chiffrement

## 📞 Support

- **Issues** : [GitHub Issues](https://github.com/votre-username/bcrdf/issues)
- **Documentation** : [docs/](docs/)
- **Exemples** : [examples/](examples/)

---

**BCRDF** - Sauvegarde moderne, sécurisée et efficace 🚀 
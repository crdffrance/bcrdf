# Architecture BCRDF

## Vue d'ensemble

BCRDF (Backup Cloud Ready Data Format) est un système de sauvegarde index-based moderne, conçu pour offrir une efficacité maximale avec une sécurité de niveau militaire.

## Architecture générale

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   CLI (Cobra)   │    │   Configuration │    │   Stockage S3   │
│                 │    │   (Viper)       │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│  Gestionnaires  │    │   Utilitaires   │    │   Index JSON    │
│                 │    │                 │    │                 │
│ • Backup        │    │ • Logger        │    │ • Métadonnées   │
│ • Restore       │    │ • File Utils    │    │ • Checksums     │
│ • Index         │    │ • Config        │    │ • Timestamps    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Composants    │    │   Chiffrement   │    │   Compression   │
│                 │    │                 │    │                 │
│ • S3 Client     │    │ • AES-256-GCM   │    │ • GZIP          │
│ • Index Manager │    │ • XChaCha20     │    │ • Niveaux 1-9   │
│ • File Scanner  │    │ • Poly1305      │    │ • Streaming     │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## Composants principaux

### 1. Interface CLI (Cobra)

**Responsabilités :**
- Parsing des commandes et flags
- Gestion des erreurs utilisateur
- Affichage des résultats formatés

**Commandes principales :**
- `backup` : Sauvegarde incrémentale
- `restore` : Restauration à un point précis
- `list` : Liste et inspection des sauvegardes
- `delete` : Suppression de sauvegardes
- `info` : Informations sur les algorithmes

### 2. Gestionnaires

#### Backup Manager
- Orchestration du processus de sauvegarde
- Gestion des workers parallèles
- Intégration des composants (chiffrement, compression, S3)

#### Restore Manager
- Restauration des fichiers
- Gestion des erreurs de déchiffrement
- Préservation de la structure des répertoires

#### Index Manager
- Création et comparaison d'index
- Détection des changements
- Métadonnées des sauvegardes

### 3. Composants de sécurité

#### Chiffrement Multi-Algorithmes
```go
type EncryptorV2 struct {
    key        []byte
    algorithm  EncryptionAlgorithm
    aesGCM     cipher.AEAD
    xchacha    cipher.AEAD
}
```

**Algorithmes supportés :**
- **AES-256-GCM** : Standard NIST, accélération matérielle
- **XChaCha20-Poly1305** : RFC 8439, optimisé logiciel

#### Compression GZIP
- Niveaux configurables (1-9)
- Streaming pour les gros fichiers
- Intégration transparente

### 4. Stockage S3

#### Structure des données
```
bucket/
├── indexes/
│   ├── backup-20241206-143022.json
│   └── backup-20241206-150000.json
└── data/
    └── backup-20241206-143022/
        ├── sha256-hash-1
        ├── sha256-hash-2
        └── sha256-hash-3
```

#### Clés de stockage
- **Indexes** : `indexes/{backup-id}.json`
- **Données** : `data/{backup-id}/{sha256-hash}`

## Flux de données

### Sauvegarde
1. **Scan** : Parcours récursif du répertoire source
2. **Index** : Création de l'index avec métadonnées
3. **Comparaison** : Différence avec la sauvegarde précédente
4. **Traitement** : Chiffrement et compression des fichiers modifiés
5. **Upload** : Envoi vers S3 avec clés dédupliquées
6. **Métadonnées** : Sauvegarde de l'index final

### Restauration
1. **Téléchargement** : Récupération de l'index depuis S3
2. **Parsing** : Extraction des métadonnées et clés
3. **Download** : Téléchargement des données chiffrées
4. **Déchiffrement** : Décryptage avec l'algorithme approprié
5. **Décompression** : Décompression GZIP
6. **Écriture** : Restauration des fichiers avec structure

## Sécurité

### Chiffrement
- **Clés** : 32 bytes pour tous les algorithmes
- **Nonces** : Génération cryptographiquement sécurisée
- **Authentification** : Tags d'intégrité inclus

### Déduplication
- **Algorithme** : SHA-256 pour les checksums
- **Avantage** : Évite la duplication des blocs identiques
- **Sécurité** : Même contenu = même clé de stockage

### Stockage
- **S3** : Chiffrement au repos (optionnel)
- **Transport** : TLS 1.2+ pour toutes les communications
- **IAM** : Permissions minimales requises

## Performance

### Optimisations
- **Parallélisme** : Workers configurables (défaut: 10)
- **Streaming** : Traitement des gros fichiers sans mémoire excessive
- **Cache** : Réutilisation des connexions S3
- **Compression** : GZIP avec niveaux adaptatifs

### Métriques
- **Sauvegarde complète** : ~10s pour 111MB
- **Sauvegarde incrémentale** : ~1s quand aucun changement
- **Restauration** : Dépend de la bande passante S3

## Configuration

### Fichier config.yaml
```yaml
storage:
  type: "s3"
  bucket: "mon-bucket"
  region: "eu-west-3"
  endpoint: "https://s3.eu-west-3.amazonaws.com"
  access_key: "VOTRE_CLE"
  secret_key: "VOTRE_SECRET"

backup:
  encryption_algo: "aes-256-gcm"
  encryption_key: "32_BYTES_KEY"
  compression_level: 3
  max_workers: 10

retention:
  days: 30
  max_backups: 10
```

## Extensibilité

### Points d'extension
- **Stockage** : Interface pour d'autres backends (Azure, GCP)
- **Chiffrement** : Support d'autres algorithmes
- **Compression** : Plugins pour d'autres formats
- **Index** : Formats d'index alternatifs

### API interne
- **Interfaces** : Définies pour tous les composants
- **Tests** : Couverture complète
- **Documentation** : Godoc pour toutes les fonctions publiques 
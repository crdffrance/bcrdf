# Exemples d'utilisation BCRDF

## 🚀 Démarrage rapide

### 1. Configuration initiale

```bash
# Cloner et compiler
git clone https://github.com/votre-username/bcrdf.git
cd bcrdf
make setup

# Configuration
cp configs/config.example.yaml config.yaml
nano config.yaml  # Configurer vos paramètres S3
```

### 2. Première sauvegarde

```bash
# Test de connexion
./bcrdf info

# Première sauvegarde complète
./bcrdf backup -n premiere-sauvegarde -s /home/user/documents -c config.yaml -v
```

### 3. Sauvegarde incrémentale

```bash
# Modifier quelques fichiers
echo "Nouveau contenu" >> /home/user/documents/test.txt

# Sauvegarde incrémentale (seuls les changements)
./bcrdf backup -n sauvegarde-quotidienne -s /home/user/documents -c config.yaml -v
```

### 4. Restauration

```bash
# Liste des sauvegardes disponibles
./bcrdf list -c config.yaml -v

# Restauration complète
./bcrdf restore --backup-id premiere-sauvegarde-20241206-143022 --destination ./restored -c config.yaml -v

# Vérification
ls -la ./restored/
```

## 🔧 Exemples avancés

### Sauvegarde avec algorithme spécifique

```bash
# Configuration pour XChaCha20-Poly1305
cat > config-xchacha.yaml << EOF
storage:
  type: "s3"
  bucket: "mon-bucket"
  region: "eu-west-3"
  endpoint: "https://s3.eu-west-3.amazonaws.com"
  access_key: "VOTRE_CLE"
  secret_key: "VOTRE_SECRET"

backup:
  source_path: "/chemin/vers/donnees"
  encryption_key: "01234567890123456789012345678901"
  encryption_algo: "xchacha20-poly1305"
  compression_level: 5
  max_workers: 8
EOF

# Sauvegarde avec XChaCha20
./bcrdf backup -n test-xchacha -s /chemin/vers/donnees -c config-xchacha.yaml -v
```

### Sauvegarde avec compression optimisée

```bash
# Configuration pour compression maximale
cat > config-compression.yaml << EOF
storage:
  type: "s3"
  bucket: "mon-bucket"
  region: "eu-west-3"
  endpoint: "https://s3.eu-west-3.amazonaws.com"
  access_key: "VOTRE_CLE"
  secret_key: "VOTRE_SECRET"

backup:
  source_path: "/chemin/vers/donnees"
  encryption_key: "01234567890123456789012345678901"
  encryption_algo: "aes-256-gcm"
  compression_level: 9  # Compression maximale
  max_workers: 4        # Moins de workers pour plus de CPU
EOF

# Sauvegarde avec compression maximale
./bcrdf backup -n backup-compresse -s /chemin/vers/donnees -c config-compression.yaml -v
```

### Sauvegarde avec parallélisme optimisé

```bash
# Configuration pour performance maximale
cat > config-performance.yaml << EOF
storage:
  type: "s3"
  bucket: "mon-bucket"
  region: "eu-west-3"
  endpoint: "https://s3.eu-west-3.amazonaws.com"
  access_key: "VOTRE_CLE"
  secret_key: "VOTRE_SECRET"

backup:
  source_path: "/chemin/vers/donnees"
  encryption_key: "01234567890123456789012345678901"
  encryption_algo: "aes-256-gcm"
  compression_level: 1  # Compression minimale pour vitesse
  max_workers: 20       # Plus de workers pour parallélisme
EOF

# Sauvegarde optimisée pour la vitesse
./bcrdf backup -n backup-rapide -s /chemin/vers/donnees -c config-performance.yaml -v
```

## 📊 Monitoring et inspection

### Inspection détaillée d'une sauvegarde

```bash
# Liste des sauvegardes
./bcrdf list -c config.yaml -v

# Détails d'une sauvegarde spécifique
./bcrdf list backup-20241206-143022 -c config.yaml -v
```

### Comparaison de sauvegardes

```bash
# Créer deux sauvegardes avec des modifications
echo "Contenu initial" > test.txt
./bcrdf backup -n test-1 -s . -c config.yaml -v

echo "Contenu modifié" > test.txt
./bcrdf backup -n test-2 -s . -c config.yaml -v

# Comparer les détails
./bcrdf list test-1-20241206-143022 -c config.yaml -v
./bcrdf list test-2-20241206-143022 -c config.yaml -v
```

## 🔄 Automatisation

### Script de sauvegarde quotidienne

```bash
#!/bin/bash
# backup-daily.sh

CONFIG_FILE="/path/to/config.yaml"
SOURCE_PATH="/home/user/documents"
BACKUP_NAME="sauvegarde-quotidienne-$(date +%Y%m%d)"

echo "🚀 Début de la sauvegarde quotidienne: $BACKUP_NAME"

# Exécuter la sauvegarde
./bcrdf backup -n "$BACKUP_NAME" -s "$SOURCE_PATH" -c "$CONFIG_FILE" -v

if [ $? -eq 0 ]; then
    echo "✅ Sauvegarde réussie: $BACKUP_NAME"
    
    # Nettoyer les anciennes sauvegardes (garder les 7 derniers jours)
    ./bcrdf list -c "$CONFIG_FILE" | grep "sauvegarde-quotidienne" | head -n -7 | while read backup_id; do
        echo "🗑️  Suppression de l'ancienne sauvegarde: $backup_id"
        ./bcrdf delete --backup-id "$backup_id" -c "$CONFIG_FILE" -v
    done
else
    echo "❌ Échec de la sauvegarde"
    exit 1
fi
```

### Cron job pour sauvegarde automatique

```bash
# Ajouter au crontab (sauvegarde quotidienne à 2h du matin)
0 2 * * * /path/to/backup-daily.sh >> /var/log/bcrdf-backup.log 2>&1
```

## 🛠️ Dépannage

### Test de connexion S3

```bash
# Vérifier la configuration
./bcrdf info

# Test de sauvegarde minimale
./bcrdf backup -n test-connexion -s /tmp -c config.yaml -v
```

### Vérification de l'intégrité

```bash
# Restaurer une sauvegarde de test
./bcrdf restore --backup-id test-connexion-20241206-143022 --destination ./test-restored -c config.yaml -v

# Comparer avec l'original
diff -r /tmp ./test-restored
```

### Logs détaillés

```bash
# Sauvegarde avec logs très détaillés
./bcrdf backup -n test-debug -s /chemin/vers/donnees -c config.yaml -v 2>&1 | tee backup.log

# Analyser les logs
grep "ERROR" backup.log
grep "WARN" backup.log
```

## 📈 Métriques et performance

### Mesure des performances

```bash
# Mesurer le temps de sauvegarde
time ./bcrdf backup -n test-performance -s /chemin/vers/donnees -c config.yaml -v

# Mesurer le temps de restauration
time ./bcrdf restore --backup-id test-performance-20241206-143022 --destination ./restored -c config.yaml -v
```

### Optimisation selon le type de données

#### Données textuelles (logs, code)
```yaml
compression_level: 9  # Compression maximale
max_workers: 4        # Moins de workers
```

#### Données binaires (images, vidéos)
```yaml
compression_level: 1  # Compression minimale
max_workers: 20       # Plus de workers
```

#### Données mixtes
```yaml
compression_level: 5  # Compression moyenne
max_workers: 10       # Workers par défaut
```

## 🔐 Sécurité avancée

### Génération de clés sécurisées

```bash
# Générer une clé AES-256-GCM (32 bytes)
openssl rand -base64 32

# Générer une clé XChaCha20-Poly1305 (32 bytes)
openssl rand -base64 32
```

### Rotation des clés

```bash
# Sauvegarde avec nouvelle clé
./bcrdf backup -n backup-nouvelle-cle -s /chemin/vers/donnees -c config-nouvelle-cle.yaml -v

# Supprimer les anciennes sauvegardes avec l'ancienne clé
./bcrdf delete --backup-id backup-ancienne-cle-20241206-143022 -c config.yaml -v
```

## 🌐 Intégration cloud

### AWS S3
```yaml
storage:
  type: "s3"
  bucket: "mon-bucket-sauvegarde"
  region: "eu-west-3"
  endpoint: "https://s3.eu-west-3.amazonaws.com"
```

### Scaleway Object Storage
```yaml
storage:
  type: "s3"
  bucket: "mon-bucket"
  region: "fr-par"
  endpoint: "https://s3.fr-par.scw.cloud"
```

### OVH Object Storage
```yaml
storage:
  type: "s3"
  bucket: "mon-bucket"
  region: "gra"
  endpoint: "https://gra.io.cloud.ovh.net"
```

### MinIO (auto-hébergé)
```yaml
storage:
  type: "s3"
  bucket: "mon-bucket"
  region: "us-east-1"
  endpoint: "http://localhost:9000"
``` 
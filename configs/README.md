# Guide de Configuration BCRDF

Utilisez désormais un seul template: `config-example.yaml`.

Copiez-le et adaptez-le:

```bash
cp configs/config-example.yaml configs/config.yaml
$EDITOR configs/config.yaml
```

Assistant interactif (recommandé):

```bash
./bcrdf init -i -c configs/config.yaml
```

## 🚀 **Démarrage Rapide**

### 1. **AWS S3** - Recommandé pour la production
```bash
# Copiez la configuration AWS
cp config-s3-aws.yaml config.yaml

# Modifiez avec vos paramètres AWS
nano config.yaml

# Lancez la sauvegarde
./bcrdf backup /path/to/backup
```

### 2. **Scaleway S3** - Optimisé pour l'Europe
```bash
# Copiez la configuration Scaleway
cp config-scaleway-s3.yaml config.yaml

# Modifiez avec vos paramètres Scaleway
nano config.yaml

# Lancez la sauvegarde
./bcrdf backup /path/to/backup
```

### 3. **WebDAV** - Pour Nextcloud/OwnCloud
```bash
# Copiez la configuration WebDAV
cp config-webdav.yaml config.yaml

# Modifiez avec vos paramètres WebDAV
nano config.yaml

# Lancez la sauvegarde
./bcrdf backup /path/to/backup
```

## 📋 **Fichier de Référence Unique**

- `config-example.yaml` : modèle unique pour S3/WebDAV avec valeurs par défaut et commentaires. 
  Copiez-le et modifiez-le selon votre environnement.

## 🔑 **Génération de Clé de Chiffrement**

**IMPORTANT** : Remplacez toujours la clé d'exemple par une clé sécurisée !

```bash
# Générez une nouvelle clé sécurisée
./scripts/generate-key.sh

# Ou utilisez OpenSSL
openssl rand -hex 32
```

## ⚙️ **Paramètres Principaux à Modifier**

### Stockage
- `bucket` : Nom de votre bucket S3
- `region` : Région de votre bucket
- `access_key` : Votre clé d'accès
- `secret_key` : Votre clé secrète

### Sauvegarde
- `encryption_key` : Clé de chiffrement (64 caractères hex)
- `compression_level` : Niveau de compression (1-9)
- `max_workers` : Nombre de workers parallèles

### Rétention
- `days` : Nombre de jours de conservation
- `max_backups` : Nombre maximum de sauvegardes

## 🔧 **Optimisations par Environnement**

### **Production (AWS/Scaleway)**
- `max_workers: 8-16`
- `compression_level: 3-5`
- `checksum_mode: "fast"`

### **WebDAV**
- `max_workers: 4-6`
- `compression_level: 1-3`
- `network_timeout: 180`

### **Faible CPU**
- `max_workers: 2-4`
- `compression_level: 1`
- `checksum_mode: "metadata"`

## 📝 **Exemple de Configuration Personnalisée**

```yaml
storage:
  type: "s3"
  bucket: "mon-bucket-backup"
  region: "eu-west-3"
  access_key: "AKIA..."
  secret_key: "..."

backup:
  encryption_key: "ma-cle-securisee-64-caracteres-hex"
  compression_level: 5
  max_workers: 12
  checksum_mode: "fast"

retention:
  days: 90
  max_backups: 15
```

## 🚨 **Sécurité**

1. **Ne commitez jamais** vos vraies clés dans Git
2. **Utilisez des variables d'environnement** en production
3. **Générez des clés uniques** pour chaque environnement
4. **Testez d'abord** avec des données non critiques

## 📞 **Support**

- **Documentation** : Voir le README principal du projet
- **Issues** : GitHub Issues pour les bugs
- **Discussions** : GitHub Discussions pour les questions


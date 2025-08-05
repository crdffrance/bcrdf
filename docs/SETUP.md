# Guide de Configuration BCRDF

## Configuration S3

BCRDF utilise AWS S3 pour stocker les sauvegardes. Voici comment configurer votre environnement :

### 1. Créer un Bucket S3

1. Connectez-vous à la [Console AWS](https://console.aws.amazon.com/)
2. Allez dans le service S3
3. Cliquez sur "Create bucket"
4. Choisissez un nom unique pour votre bucket (ex: `bcrdf-backups`)
5. Sélectionnez votre région préférée
6. Configurez les options de sécurité selon vos besoins
7. Cliquez sur "Create bucket"

### 2. Créer un Utilisateur IAM

1. Allez dans le service IAM
2. Cliquez sur "Users" puis "Create user"
3. Donnez un nom à l'utilisateur (ex: `bcrdf-backup-user`)
4. Attachez la politique suivante :

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "s3:GetObject",
                "s3:PutObject",
                "s3:DeleteObject",
                "s3:ListBucket"
            ],
            "Resource": [
                "arn:aws:s3:::bcrdf-backups",
                "arn:aws:s3:::bcrdf-backups/*"
            ]
        }
    ]
}
```

### 3. Générer les Clés d'Accès

1. Sélectionnez l'utilisateur créé
2. Allez dans l'onglet "Security credentials"
3. Cliquez sur "Create access key"
4. Choisissez "Application running outside AWS"
5. Copiez l'Access Key ID et la Secret Access Key

### 4. Configurer BCRDF

1. Copiez le fichier de configuration :
```bash
cp configs/config.yaml config.yaml
```

2. Modifiez `config.yaml` avec vos paramètres :
```yaml
storage:
  type: "s3"
  bucket: "bcrdf-backups"  # Votre nom de bucket
  region: "us-east-1"      # Votre région
  endpoint: "https://s3.us-east-1.amazonaws.com"
  access_key: "AKIA..."    # Votre Access Key ID
  secret_key: "..."        # Votre Secret Access Key

backup:
  source_path: "/path/to/backup"
  encryption_key: "your-very-secure-encryption-key-32-chars"
  compression_level: 3
  max_workers: 10
```

### 5. Variables d'Environnement (Alternative)

Au lieu de mettre les clés dans le fichier de configuration, vous pouvez utiliser des variables d'environnement :

```bash
export AWS_ACCESS_KEY_ID="AKIA..."
export AWS_SECRET_ACCESS_KEY="..."
```

Dans ce cas, laissez `access_key` et `secret_key` vides dans `config.yaml`.

## Test de Configuration

Après avoir configuré S3, testez la connexion :

```bash
# Compiler l'application
make build

# Tester une sauvegarde
./bcrdf backup --source ./test-data --name "test-backup"
```

## Sécurité

### Clé de Chiffrement

La clé de chiffrement doit être :
- **Longue** : Au moins 32 caractères
- **Complexe** : Mélange de lettres, chiffres et symboles
- **Secrète** : Ne la partagez jamais

Exemple de génération :
```bash
# Générer une clé aléatoire
openssl rand -base64 32
```

### Permissions S3

Le bucket S3 doit être configuré avec :
- **Chiffrement** : SSE-S3 ou SSE-KMS
- **Versioning** : Activé pour la récupération
- **Lifecycle** : Politique de rétention (optionnel)

## Dépannage

### Erreur de Connexion S3

```
Error: NoCredentialProviders: no valid providers in chain
```

**Solution :** Vérifiez vos clés AWS dans `config.yaml` ou les variables d'environnement.

### Erreur de Bucket

```
Error: NoSuchBucket: The specified bucket does not exist
```

**Solution :** Vérifiez le nom du bucket dans `config.yaml`.

### Erreur de Permissions

```
Error: AccessDenied: Access Denied
```

**Solution :** Vérifiez les permissions IAM de l'utilisateur.

### Erreur de Région

```
Error: PermanentRedirect: The bucket you are attempting to access must be addressed using the specified endpoint
```

**Solution :** Vérifiez la région dans `config.yaml`.

## Support des Endpoints Personnalisés

BCRDF supporte les endpoints S3 personnalisés pour :
- **MinIO** : `http://localhost:9000`
- **Ceph** : `http://ceph-cluster:7480`
- **Backblaze B2** : `https://s3.us-west-002.backblazeb2.com`

Exemple pour MinIO :
```yaml
storage:
  type: "s3"
  bucket: "bcrdf-backups"
  region: "us-east-1"
  endpoint: "http://localhost:9000"
  access_key: "minioadmin"
  secret_key: "minioadmin"
``` 
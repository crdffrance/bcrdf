# Intégration S3 - BCRDF

## Vue d'ensemble

BCRDF utilise AWS S3 comme backend de stockage pour les sauvegardes. L'intégration S3 est maintenant **complètement fonctionnelle** et permet de stocker et récupérer les données de sauvegarde de manière sécurisée.

## Fonctionnalités S3

### ✅ Upload de Données
- **Fichiers de données** : Upload vers `data/{backup-id}/{file-key}`
- **Index JSON** : Upload vers `indexes/{backup-id}.json`
- **Gestion d'erreurs** : Retry automatique et logging détaillé

### ✅ Download de Données
- **Récupération de fichiers** : Download depuis les clés de stockage
- **Chargement d'index** : Récupération des métadonnées de sauvegarde
- **Validation** : Vérification de l'intégrité des données

### ✅ Gestion des Objets
- **Liste des sauvegardes** : Scan du préfixe `indexes/`
- **Suppression** : Nettoyage des objets et index
- **Vérification d'existence** : HeadObject pour valider les objets

### ✅ Support Multi-Endpoint
- **AWS S3** : Endpoints régionaux standard
- **MinIO** : `http://localhost:9000`
- **Ceph** : `http://ceph-cluster:7480`
- **Backblaze B2** : `https://s3.us-west-002.backblazeb2.com`

## Architecture S3

### Structure de Stockage
```
bucket/
├── indexes/
│   ├── backup-20241206-143022.json
│   ├── backup-20241206-150000.json
│   └── ...
└── data/
    ├── backup-20241206-143022/
    │   ├── 72e864d261324b8c9bfd4fd41daccb04edda9a12eb9dd2b8e5a577f3edb25fc8
    │   ├── 36ccc87c507506e921062908b56cf417f7ce34af80724752f31b259a31244178
    │   └── ...
    └── backup-20241206-150000/
        ├── 72e864d261324b8c9bfd4fd41daccb04edda9a12eb9dd2b8e5a577f3edb25fc8
        └── ...
```

### Clés de Stockage
- **Index** : `indexes/{backup-id}.json`
- **Données** : `data/{backup-id}/{file-hash}`
- **Format** : SHA-256 du contenu pour la déduplication

## Client S3

### Configuration
```go
client, err := s3.NewClient(
    accessKey,    // Clé d'accès AWS
    secretKey,    // Clé secrète AWS
    region,       // Région S3
    endpoint,     // Endpoint personnalisé (optionnel)
    bucket,       // Nom du bucket
)
```

### Opérations Principales
```go
// Upload
err := client.Upload("key", data)

// Download
data, err := client.Download("key")

// Liste
keys, err := client.ListObjects("prefix/")

// Suppression
err := client.DeleteObject("key")

// Vérification
exists, err := client.Exists("key")
```

## Sécurité

### Chiffrement
- **AES-256-GCM** : Chiffrement de bout en bout
- **Nonce aléatoire** : Généré pour chaque opération
- **Authentification** : GCM fournit l'intégrité

### Permissions IAM
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

### Configuration Bucket
- **Chiffrement SSE-S3** : Chiffrement côté serveur
- **Versioning** : Protection contre suppression accidentelle
- **Lifecycle** : Politique de rétention automatique

## Performance

### Optimisations
- **Upload Manager** : Upload parallèle et multipart
- **Download Manager** : Download optimisé avec buffer
- **Connexions réutilisées** : Session AWS partagée
- **Retry automatique** : Gestion des erreurs réseau

### Métriques
- **Débit** : Dépend de la bande passante
- **Latence** : Minimisée par les endpoints régionaux
- **Fiabilité** : 99.99% de disponibilité S3

## Dépannage

### Erreurs Courantes

#### NoCredentialProviders
```
Error: NoCredentialProviders: no valid providers in chain
```
**Solution :** Vérifiez les clés AWS dans `config.yaml`

#### NoSuchBucket
```
Error: NoSuchBucket: The specified bucket does not exist
```
**Solution :** Vérifiez le nom du bucket

#### AccessDenied
```
Error: AccessDenied: Access Denied
```
**Solution :** Vérifiez les permissions IAM

#### PermanentRedirect
```
Error: PermanentRedirect: The bucket you are attempting to access must be addressed using the specified endpoint
```
**Solution :** Vérifiez la région et l'endpoint

### Debug
```bash
# Mode verbeux
./bcrdf --verbose backup --source ./data --name "debug-test"

# Test de connexion
make test-s3

# Vérification des logs
tail -f /var/log/bcrdf.log
```

## Tests

### Test de Connexion
```bash
# Exécuter le script de test
make test-s3

# Ou manuellement
./scripts/test-s3.sh
```

### Test de Sauvegarde
```bash
# Créer des données de test
mkdir -p test-data
echo "Test S3" > test-data/file1.txt

# Sauvegarde
./bcrdf backup --source ./test-data --name "s3-test"

# Vérifier dans S3
aws s3 ls s3://bcrdf-backups/indexes/
aws s3 ls s3://bcrdf-backups/data/
```

### Test de Restauration
```bash
# Restaurer
./bcrdf restore --backup-id "s3-test-20241206-143022" --destination ./restored

# Vérifier
ls -la ./restored/
cat ./restored/test-data/file1.txt
```

## Monitoring

### Métriques S3
- **Objets uploadés** : Nombre de fichiers sauvegardés
- **Taille des données** : Volume de données transféré
- **Temps de transfert** : Latence des opérations
- **Erreurs** : Taux d'échec des opérations

### Logs
```bash
# Activer le mode debug
export BCRDF_DEBUG=1

# Voir les logs détaillés
./bcrdf --verbose backup --source ./data --name "test"
```

## Évolutions Futures

### Fonctionnalités Planifiées
- **Multipart Upload** : Pour les gros fichiers
- **Accélération de transfert** : S3 Transfer Acceleration
- **Chiffrement KMS** : SSE-KMS pour plus de sécurité
- **Monitoring CloudWatch** : Métriques détaillées

### Optimisations
- **Cache local** : Réduction des appels S3
- **Compression adaptative** : Selon le type de données
- **Déduplication globale** : Partage entre sauvegardes
- **Rétention intelligente** : Politiques avancées

## Support

### Documentation
- **Guide de configuration** : `docs/SETUP.md`
- **Architecture** : `docs/ARCHITECTURE.md`
- **API S3** : [Documentation AWS](https://docs.aws.amazon.com/s3/)

### Outils
- **Script de test** : `scripts/test-s3.sh`
- **Configuration** : `configs/config.yaml`
- **Makefile** : `make test-s3`

L'intégration S3 de BCRDF est maintenant **complète et fonctionnelle**, permettant un stockage sécurisé et fiable de vos sauvegardes index-based. 
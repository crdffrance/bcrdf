# Fonction de Nettoyage BCRDF

## Vue d'ensemble

La fonction de nettoyage (`clean`) de BCRDF permet de vérifier la cohérence entre les fichiers stockés sur le stockage (S3, WebDAV, etc.) et l'index de sauvegarde, puis de supprimer les fichiers orphelins qui ne sont plus référencés.

## Problème résolu

Lors de l'utilisation de BCRDF sur de longues périodes, il peut arriver que :
- Des fichiers soient supprimés manuellement du stockage
- Des opérations de sauvegarde soient interrompues
- Des fichiers temporaires ou de test restent sur le stockage
- Des incohérences se développent entre l'index et le stockage réel

La fonction `clean` identifie et supprime ces fichiers orphelins pour maintenir la cohérence et libérer de l'espace de stockage.

## Utilisation

### Commande de base

```bash
bcrdf clean --backup-id <BACKUP_ID>
```

### Options disponibles

- `--backup-id, -b` : ID de la sauvegarde à nettoyer (obligatoire)
- `--dry-run, -d` : Mode simulation (affiche ce qui serait supprimé sans supprimer)
- `--verbose, -v` : Mode verbeux pour plus de détails
- `--config, -c` : Fichier de configuration à utiliser

### Exemples d'utilisation

#### 1. Vérification en mode simulation

```bash
bcrdf clean --backup-id "backup-2024-01-15" --dry-run --verbose
```

Cette commande :
- Charge l'index de la sauvegarde "backup-2024-01-15"
- Scanne le stockage pour identifier les fichiers orphelins
- Affiche la liste des fichiers qui seraient supprimés
- **Ne supprime aucun fichier** (mode simulation)

#### 2. Nettoyage effectif

```bash
bcrdf clean --backup-id "backup-2024-01-15" --verbose
```

Cette commande :
- Effectue la même analyse qu'en mode simulation
- Demande confirmation avant de supprimer
- Supprime effectivement les fichiers orphelins
- Affiche un rapport détaillé des opérations

#### 3. Nettoyage silencieux

```bash
bcrdf clean --backup-id "backup-2024-01-15"
```

Cette commande :
- Effectue le nettoyage sans affichage détaillé
- Utilise une barre de progression pour le suivi
- Demande confirmation avant suppression

## Fonctionnement technique

### 1. Chargement de l'index

La fonction charge l'index de sauvegarde spécifié pour obtenir la liste des fichiers valides.

### 2. Initialisation du client de stockage

Un client de stockage est initialisé selon la configuration (S3, WebDAV, etc.).

### 3. Scan du stockage

Tous les objets avec le préfixe `backups/<BACKUP_ID>/` sont listés depuis le stockage.

### 4. Identification des fichiers orphelins

Les fichiers sont comparés avec l'index :
- **Fichiers valides** : Présents dans l'index et sur le stockage
- **Fichiers orphelins** : Présents sur le stockage mais absents de l'index
- **Fichiers d'index** : Ignorés (`.index`, `.metadata`)

### 5. Suppression sécurisée

Avant toute suppression :
- Confirmation utilisateur requise (sauf en mode dry-run)
- Affichage du résumé des opérations
- Gestion des erreurs individuelles

## Sécurité

### Protection contre les suppressions accidentelles

- **Mode dry-run par défaut** : Toujours tester avant d'appliquer
- **Confirmation obligatoire** : L'utilisateur doit confirmer explicitement
- **Vérification de l'index** : Seuls les fichiers non référencés sont supprimés
- **Préservation des métadonnées** : Les fichiers `.index` et `.metadata` sont protégés

### Vérification post-nettoyage

Après le nettoyage, il est recommandé de :
- Tester la restauration de la sauvegarde
- Vérifier l'intégrité des données
- Comparer avec une sauvegarde de référence

## Cas d'usage

### 1. Maintenance régulière

```bash
# Vérification mensuelle
bcrdf clean --backup-id "monthly-backup-2024-01" --dry-run
```

### 2. Nettoyage après erreur

```bash
# Après une sauvegarde interrompue
bcrdf clean --backup-id "interrupted-backup" --verbose
```

### 3. Libération d'espace

```bash
# Identifier l'espace récupérable
bcrdf clean --backup-id "old-backup" --dry-run
```

## Gestion des erreurs

### Types d'erreurs courantes

1. **Index introuvable** : La sauvegarde n'existe pas
2. **Erreur de connexion** : Problème de connectivité au stockage
3. **Erreur de suppression** : Fichier protégé ou inaccessible
4. **Permissions insuffisantes** : Droits d'écriture limités

### Recommandations

- Toujours commencer par un `--dry-run`
- Utiliser `--verbose` pour diagnostiquer les problèmes
- Tester sur une sauvegarde de test avant la production
- Sauvegarder l'index avant le nettoyage si nécessaire

## Intégration avec d'autres commandes

### Workflow recommandé

```bash
# 1. Vérifier l'état des sauvegardes
bcrdf list

# 2. Identifier les sauvegardes à nettoyer
bcrdf list "backup-id"

# 3. Tester le nettoyage
bcrdf clean --backup-id "backup-id" --dry-run --verbose

# 4. Appliquer le nettoyage
bcrdf clean --backup-id "backup-id" --verbose

# 5. Vérifier l'intégrité
bcrdf health --backup-id "backup-id"
```

### Scripts d'automatisation

La fonction peut être intégrée dans des scripts de maintenance :

```bash
#!/bin/bash
# Script de maintenance automatique

BACKUP_ID="daily-backup-$(date +%Y-%m-%d)"

# Vérification et nettoyage
bcrdf clean --backup-id "$BACKUP_ID" --dry-run > clean-report.txt

# Si des fichiers orphelins sont trouvés, notifier l'administrateur
if grep -q "orphaned files found" clean-report.txt; then
    echo "Orphaned files detected in $BACKUP_ID" | mail -s "BCRDF Clean Alert" admin@example.com
fi
```

## Limitations et considérations

### Limitations actuelles

- Un seul backup ID par opération
- Pas de nettoyage en lot sur plusieurs sauvegardes
- Suppression immédiate (pas de corbeille)

### Considérations de performance

- **Temps de scan** : Proportionnel au nombre d'objets sur le stockage
- **Bande passante** : Utilise l'API ListObjects du stockage
- **Mémoire** : Charge l'index complet en mémoire

### Recommandations de performance

- Exécuter pendant les heures creuses
- Utiliser le mode non-verbeux pour les gros volumes
- Surveiller l'utilisation des ressources

## Support et dépannage

### Logs et diagnostics

En mode verbeux, la fonction affiche :
- Progression des opérations
- Détails des erreurs
- Statistiques de nettoyage
- Résumé final détaillé

### Problèmes courants

1. **"No orphaned files found"** : Le stockage est déjà propre
2. **"Failed to load index"** : Vérifier l'existence de la sauvegarde
3. **"Failed to initialize storage client"** : Vérifier la configuration
4. **"Failed to list objects"** : Vérifier la connectivité au stockage

### Obtenir de l'aide

```bash
# Aide générale
bcrdf --help

# Aide spécifique à la commande clean
bcrdf clean --help

# Mode verbose pour plus de détails
bcrdf clean --backup-id "backup-id" --verbose
```

## Conclusion

La fonction de nettoyage BCRDF offre un moyen sûr et efficace de maintenir la cohérence entre l'index et le stockage. En utilisant le mode dry-run et en vérifiant l'intégrité après nettoyage, vous pouvez maintenir un système de sauvegarde propre et optimisé.

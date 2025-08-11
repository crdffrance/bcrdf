# Exemples d'utilisation de la commande clean

## Scénario 1 : Vérification de sécurité (mode dry-run)

Avant de nettoyer quoi que ce soit, il est toujours recommandé de vérifier ce qui serait supprimé :

```bash
# Vérifier les fichiers orphelins sans les supprimer
bcrdf clean --backup-id "backup-2024-01-15" --dry-run --verbose

# Sortie attendue :
# 🔄 Loading backup index...
# ✅ Index loaded successfully
# 🔄 Initializing storage client...
# ✅ Storage client initialized
# 🔄 Scanning storage for orphaned files...
# ✅ Found 3 orphaned files
# 
# 🔍 Orphaned files found:
# --------------------------------------------------------------------------------
# Total files: 3
# Total size: 15.2 MB
# Backup ID: backup-2024-01-15
# Mode: DRY RUN (no files will be deleted)
# --------------------------------------------------------------------------------
# 
# 📋 Orphaned files list:
#   1. backups/backup-2024-01-15/temp-file.tmp (2.1 MB)
#   2. backups/backup-2024-01-15/partial-upload.dat (8.7 MB)
#   3. backups/backup-2024-01-15/test-data.bin (4.4 MB)
# 
# 🧹 CLEAN OPERATION COMPLETED (DRY RUN)
# ================================================================================
# Backup ID: backup-2024-01-15
# Files processed: 3
# Files deleted: 3
# Size freed: 15.2 MB
# 
# ℹ️  This was a dry run. No files were actually deleted.
```

## Scénario 2 : Nettoyage effectif avec confirmation

Une fois que vous êtes satisfait du rapport dry-run, vous pouvez procéder au nettoyage réel :

```bash
# Nettoyer les fichiers orphelins
bcrdf clean --backup-id "backup-2024-01-15" --verbose

# Sortie attendue :
# 🔄 Loading backup index...
# ✅ Index loaded successfully
# 🔄 Initializing storage client...
# ✅ Storage client initialized
# 🔄 Scanning storage for orphaned files...
# ✅ Found 3 orphaned files
# 
# 🔍 Orphaned files found:
# --------------------------------------------------------------------------------
# Total files: 3
# Total size: 15.2 MB
# Backup ID: backup-2024-01-15
# Mode: LIVE (files will be deleted)
# --------------------------------------------------------------------------------
# 
# ⚠️  WARNING: This will permanently delete 3 orphaned files (15.2 MB)
# Are you sure you want to continue? (yes/no): yes
# 
# 🔄 Deleting orphaned files...
# 🔄 Deleting backups/backup-2024-01-15/temp-file.tmp (2.1 MB)...
# ✅ Deleted backups/backup-2024-01-15/temp-file.tmp
# 🔄 Deleting backups/backup-2024-01-15/partial-upload.dat (8.7 MB)...
# ✅ Deleted backups/backup-2024-01-15/partial-upload.dat
# 🔄 Deleting backups/backup-2024-01-15/test-data.bin (4.4 MB)...
# ✅ Deleted backups/backup-2024-01-15/test-data.bin
# 
# 🧹 CLEAN OPERATION COMPLETED
# ================================================================================
# Backup ID: backup-202up-2024-01-15
# Files processed: 3
# Files deleted: 3
# Size freed: 15.2 MB
# 
# ✅ Orphaned files cleanup completed successfully!
```

## Scénario 3 : Nettoyage silencieux avec barre de progression

Pour les gros volumes ou les opérations automatisées :

```bash
# Nettoyage sans affichage détaillé
bcrdf clean --backup-id "backup-2024-01-15"

# Sortie attendue :
# [██████████████████████████████████████████████████] 3/3 (100%) 2.1/s
# 
# 🔍 Orphaned files found:
# --------------------------------------------------------------------------------
# Total files: 3
# Total size: 15.2 MB
# Backup ID: backup-2024-01-15
# Mode: LIVE (files will be deleted)
# --------------------------------------------------------------------------------
# 
# ⚠️  WARNING: This will permanently delete 3 orphaned files (15.2 MB)
# Are you sure you want to continue? (yes/no): yes
# 
# [██████████████████████████████████████████████████] 3/3 (100%) 1.8/s
# 
# 🧹 CLEAN OPERATION COMPLETED
# ================================================================================
# Backup ID: backup-2024-01-15
# Files processed: 3
# Files deleted: 3
# Size freed: 15.2 MB
# 
# ✅ Orphaned files cleanup completed successfully!
```

## Scénario 4 : Aucun fichier orphelin trouvé

Si le stockage est déjà propre :

```bash
bcrdf clean --backup-id "clean-backup" --verbose

# Sortie attendue :
# 🔄 Loading backup index...
# ✅ Index loaded successfully
# 🔄 Initializing storage client...
# ✅ Storage client initialized
# 🔄 Scanning storage for orphaned files...
# ✅ Found 0 orphaned files
# ✅ No orphaned files found. Storage is clean!
```

## Scénario 5 : Gestion des erreurs

En cas de problème lors de la suppression :

```bash
bcrdf clean --backup-id "problematic-backup" --verbose

# Sortie attendue (avec erreurs) :
# 🔄 Loading backup index...
# ✅ Index loaded successfully
# 🔄 Initializing storage client...
# ✅ Storage client initialized
# 🔄 Scanning storage for orphaned files...
# ✅ Found 2 orphaned files
# 
# 🔍 Orphaned files found:
# --------------------------------------------------------------------------------
# Total files: 2
# Total size: 8.7 MB
# Backup ID: problematic-backup
# Mode: LIVE (files will be deleted)
# --------------------------------------------------------------------------------
# 
# ⚠️  WARNING: This will permanently delete 2 orphaned files (8.7 MB)
# Are you sure you want to continue? (yes/no): yes
# 
# 🔄 Deleting orphaned files...
# 🔄 Deleting backups/problematic-backup/file1.dat (4.2 MB)...
# ✅ Deleted backups/problematic-backup/file1.dat
# 🔄 Deleting backups/problematic-backup/file2.dat (4.5 MB)...
# ❌ failed to delete backups/problematic-backup/file2.dat: access denied
# 
# 🧹 CLEAN OPERATION COMPLETED
# ================================================================================
# Backup ID: problematic-backup
# Files processed: 2
# Files deleted: 1
# Size freed: 4.2 MB
# Errors encountered: 1
#   • failed to delete backups/problematic-backup/file2.dat: access denied
# 
# ✅ Orphaned files cleanup completed successfully!
```

## Scénario 6 : Utilisation dans un script automatisé

```bash
#!/bin/bash
# Script de maintenance automatique

BACKUP_ID="daily-backup-$(date +%Y-%m-%d)"
CONFIG_FILE="/etc/bcrdf/config.yaml"
LOG_FILE="/var/log/bcrdf/cleanup.log"

echo "$(date): Starting cleanup for $BACKUP_ID" >> "$LOG_FILE"

# Vérifier s'il y a des fichiers orphelins
if bcrdf --config "$CONFIG_FILE" clean --backup-id "$BACKUP_ID" --dry-run > /tmp/clean-report.txt 2>&1; then
    # Vérifier s'il y a des fichiers à nettoyer
    if grep -q "orphaned files found" /tmp/clean-report.txt; then
        echo "$(date): Orphaned files detected, proceeding with cleanup" >> "$LOG_FILE"
        
        # Effectuer le nettoyage (avec confirmation automatique)
        echo "yes" | bcrdf --config "$CONFIG_FILE" clean --backup-id "$BACKUP_ID" >> "$LOG_FILE" 2>&1
        
        if [ $? -eq 0 ]; then
            echo "$(date): Cleanup completed successfully" >> "$LOG_FILE"
        else
            echo "$(date): Cleanup failed" >> "$LOG_FILE"
        fi
    else
        echo "$(date): No orphaned files found" >> "$LOG_FILE"
    fi
else
    echo "$(date): Failed to analyze backup $BACKUP_ID" >> "$LOG_FILE"
fi

# Nettoyer les fichiers temporaires
rm -f /tmp/clean-report.txt
```

## Scénario 7 : Intégration avec d'autres commandes BCRDF

```bash
# Workflow complet de maintenance
echo "=== BCRDF Maintenance Workflow ==="

# 1. Lister les sauvegardes disponibles
echo "1. Listing available backups..."
bcrdf list

# 2. Vérifier la santé d'une sauvegarde
echo "2. Checking backup health..."
BACKUP_ID="important-backup-2024-01"
bcrdf health --backup-id "$BACKUP_ID"

# 3. Identifier les fichiers orphelins
echo "3. Identifying orphaned files..."
bcrdf clean --backup-id "$BACKUP_ID" --dry-run --verbose

# 4. Nettoyer les fichiers orphelins
echo "4. Cleaning orphaned files..."
echo "yes" | bcrdf clean --backup-id "$BACKUP_ID" --verbose

# 5. Vérifier l'intégrité après nettoyage
echo "5. Verifying integrity after cleanup..."
bcrdf health --backup-id "$BACKUP_ID"

echo "=== Maintenance completed ==="
```

## Conseils d'utilisation

### Sécurité
- **Toujours commencer par `--dry-run`** pour voir ce qui serait supprimé
- **Utiliser `--verbose`** pour diagnostiquer les problèmes
- **Tester sur une sauvegarde de test** avant la production

### Performance
- **Mode silencieux** pour les gros volumes (sans `--verbose`)
- **Exécuter pendant les heures creuses** pour minimiser l'impact
- **Surveiller l'utilisation des ressources** pendant l'opération

### Maintenance
- **Intégrer dans les scripts de maintenance** réguliers
- **Documenter les opérations** de nettoyage
- **Vérifier l'intégrité** après chaque nettoyage

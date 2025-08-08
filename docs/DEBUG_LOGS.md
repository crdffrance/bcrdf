# 🔍 Logs de Debug Détaillés - BCRDF

## 📋 Vue d'ensemble

BCRDF dispose maintenant de **logs de debug détaillés** qui indiquent clairement les tâches en cours pendant les opérations de backup et restore. Ces logs permettent de :

- **Suivre la progression** en temps réel
- **Identifier les goulots d'étranglement**
- **Diagnostiquer les problèmes**
- **Comprendre le fonctionnement interne**

## 🚀 Activation des Logs Détaillés

### **Mode Verbose pour Backup**
```bash
./bcrdf backup -n "mon-backup" -s "/path/to/data" --config config.yaml -v
```

### **Mode Verbose pour Restore**
```bash
./bcrdf restore -b "backup-id" -d "/restore/path" --config config.yaml -v
```

## 📊 Logs de Backup Détaillés

### **1. Initialisation**
```
🔄 🚀 Starting backup: mon-backup
📋 Tasks to perform:
   1. Initialize backup manager
   2. Create current file index
   3. Find previous backup (if exists)
   4. Calculate file differences
   5. Backup new/modified files
   6. Create and save backup index
   7. Apply retention policy
```

### **2. Tâche 1 : Initialisation du Manager**
```
🔧 Task: Initializing backup manager
✅ Task completed: Backup manager initialized
```

### **3. Tâche 2 : Création de l'Index**
```
📋 Task 2: Creating current file index
   - Scanning directory: /path/to/data
   - Calculating checksums
   - Building file index
✅ Task 2 completed: Index created with 1234 files
```

### **4. Tâche 3 : Recherche de Sauvegarde Précédente**
```
📋 Task 3: Finding previous backup
   - Searching for existing backups
   - Loading previous index (if exists)
```

### **5. Tâche 4 : Calcul des Différences**
```
📋 Task 4: Calculating file differences
   - Comparing current vs previous index
   - Identifying new files
   - Identifying modified files
   - Identifying deleted files
✅ Task 4 completed: Found 50 new, 10 modified, 5 deleted files
```

### **6. Tâche 5 : Sauvegarde des Fichiers**
```
📋 Task 5: Backing up files
   - Total files to backup: 60
   - New files: 50
   - Modified files: 10
   - Processing files in parallel
   - Encrypting and compressing
   - Uploading to storage
   - Starting parallel processing with 16 workers
   - Processing file: document.pdf (2.5 MB)
   - All files processed successfully
✅ Task 5 completed: All files backed up
```

### **7. Tâche 6 : Finalisation**
```
📋 Task 6: Finalizing backup
   - Calculating backup statistics
   - Creating backup index
   - Saving index to storage
✅ Task 6 completed: Backup index saved
```

### **8. Tâche 7 : Politique de Rétention**
```
📋 Task 7: Applying retention policy
   - Loading retention configuration
   - Finding old backups
   - Deleting expired backups
✅ Task 7 completed: Retention policy applied
```

### **9. Finalisation**
```
🎯 Final tasks completed:
   ✅ All files backed up successfully
   ✅ Backup index saved
   ✅ Retention policy applied
```

## 📥 Logs de Restore Détaillés

### **1. Initialisation**
```
🔄 🚀 Starting restore: backup-id
📋 Tasks to perform:
   1. Initialize restore manager
   2. Load backup index
   3. Prepare destination directory
   4. Download and restore files
   5. Verify restored files
   6. Finalize restore operation
```

### **2. Tâche 1 : Initialisation du Restore Manager**
```
📋 Task 1: Initializing restore manager
   - Loading configuration
   - Initializing components
✅ Task 1 completed: Restore manager initialized
```

### **3. Tâche 2 : Chargement de l'Index**
```
📋 Task 2: Loading backup index
   - Connecting to storage
   - Downloading backup index
   - Parsing index data
✅ Task 2 completed: Index loaded with 1234 files
```

### **4. Tâche 3 : Préparation du Répertoire**
```
📋 Task 3: Preparing destination directory
   - Checking destination: /restore/path
   - Creating directory structure
✅ Task 3 completed: Destination directory ready
```

### **5. Tâche 4 : Téléchargement et Restauration**
```
📋 Task 4: Downloading and restoring files
   - Total files to restore: 1234
   - Processing files in parallel
   - Downloading from storage
   - Decrypting and decompressing
   - Writing to destination
   - Starting parallel processing with 16 workers
   - Processing file: document.pdf (2.5 MB)
   - All files restored successfully
✅ Task 4 completed: All files restored
```

### **6. Tâche 5 : Vérification**
```
📋 Task 5: Verifying restored files
   - Checking file integrity
   - Validating file sizes
   - Verifying file permissions
```

### **7. Tâche 6 : Finalisation**
```
📋 Task 6: Finalizing restore operation
   - Cleaning up temporary files
   - Finalizing restore
✅ Task 6 completed: Restore operation finalized
🎯 Restore completed successfully!
   ✅ All files restored to: /restore/path
   ✅ File integrity verified
   ✅ Restore operation completed
```

## 🔧 Logs de Chunking Détaillés

### **Backup de Gros Fichiers**
```
🔄 Processing ultra-large file: /path/to/large-file.bin (5242880000 bytes, 5000.00 MB)
📋 Starting chunked upload for ultra-large file: /path/to/large-file.bin
🔧 Using chunk size: 50MB (52428800 bytes) for ultra-large file
📊 File processing plan:
   - Total file size: 5000.00 MB
   - Chunk size: 50.00 MB
   - Total chunks: 100
   - Storage key: data/backup-id/hash
🔄 Processing chunk 1: 52428800 bytes (50.00 MB), total: 50.00 MB / 5000.00 MB
🔐 Encrypting chunk 1...
✅ Chunk 1 encrypted successfully
📤 Uploading chunk 1 to storage: data/backup-id/hash.chunk.000
✅ Chunk 1 uploaded successfully
...
📝 Creating metadata file for chunked file...
📤 Uploading metadata file: data/backup-id/hash.metadata
✅ Metadata file uploaded successfully
🎯 Ultra-large file backed up in 100 chunks: /path/to/large-file.bin -> data/backup-id/hash
```

### **Restore de Gros Fichiers**
```
🔄 Restoring chunked file: /path/to/large-file.bin (5000.00 MB)
📥 Downloading metadata file: data/backup-id/hash.metadata
📊 Chunked file restoration plan:
   - Total chunks: 100
   - Storage key: data/backup-id/hash
   - Destination: /restore/path/large-file.bin
📝 Creating destination file: /restore/path/large-file.bin
📥 Downloading chunk 1/100: data/backup-id/hash.chunk.000
✅ Chunk 1 downloaded successfully (52428840 bytes)
🔓 Decrypting chunk 1...
✅ Chunk 1 decrypted successfully
📝 Writing chunk 1 to file...
✅ Chunk 1 written to file successfully
📊 Progress: 50.00 MB / 5000.00 MB
...
🎯 Chunked file restoration completed: data/backup-id/hash -> /restore/path/large-file.bin
```

## 🎯 Avantages des Logs Détaillés

### **1. Diagnostic des Problèmes**
- **Identification précise** de l'étape où un problème survient
- **Informations détaillées** sur les opérations en cours
- **Traçabilité complète** du processus

### **2. Optimisation des Performances**
- **Suivi du temps** passé sur chaque étape
- **Identification des goulots d'étranglement**
- **Monitoring de l'usage CPU/mémoire**

### **3. Compréhension du Fonctionnement**
- **Visibilité** sur l'algorithme interne
- **Explication** des décisions prises
- **Documentation** en temps réel

### **4. Support et Débogage**
- **Logs structurés** pour faciliter l'analyse
- **Informations contextuelles** pour chaque opération
- **Traçabilité** complète des erreurs

## 🧪 Test des Logs

### **Script de Test Automatisé**
```bash
# Exécuter le test complet
./test-debug-logs.sh
```

### **Test Manuel**
```bash
# Backup avec logs détaillés
./bcrdf backup -n "test" -s "/tmp/test-data" --config config.yaml -v

# Restore avec logs détaillés
./bcrdf restore -b "test-backup-id" -d "/tmp/restore" --config config.yaml -v
```

## 📈 Exemples de Sortie

### **Backup Réussi**
```
🔄 🚀 Starting backup: test-backup
📋 Tasks to perform:
   1. Initialize backup manager
   2. Create current file index
   3. Find previous backup (if exists)
   4. Calculate file differences
   5. Backup new/modified files
   6. Create and save backup index
   7. Apply retention policy
🔧 Task: Initializing backup manager
✅ Task completed: Backup manager initialized
📋 Task 2: Creating current file index
   - Scanning directory: /tmp/test-data
   - Calculating checksums
   - Building file index
✅ Task 2 completed: Index created with 5 files
📋 Task 3: Finding previous backup
   - Searching for existing backups
   - Loading previous index (if exists)
📋 Task 4: Calculating file differences
   - Comparing current vs previous index
   - Identifying new files
   - Identifying modified files
   - Identifying deleted files
✅ Task 4 completed: Found 5 new, 0 modified, 0 deleted files
📋 Task 5: Backing up files
   - Total files to backup: 5
   - New files: 5
   - Modified files: 0
   - Processing files in parallel
   - Encrypting and compressing
   - Uploading to storage
   - Starting parallel processing with 16 workers
   - Processing file: document.txt (2.5 MB)
   - All files processed successfully
✅ Task 5 completed: All files backed up
📋 Task 6: Finalizing backup
   - Calculating backup statistics
   - Creating backup index
   - Saving index to storage
✅ Task 6 completed: Backup index saved
📋 Task 7: Applying retention policy
   - Loading retention configuration
   - Finding old backups
   - Deleting expired backups
✅ Task 7 completed: Retention policy applied
🎯 Final tasks completed:
   ✅ All files backed up successfully
   ✅ Backup index saved
   ✅ Retention policy applied
```

### **Restore Réussi**
```
🔄 🚀 Starting restore: test-backup-20240808-143022
📋 Tasks to perform:
   1. Initialize restore manager
   2. Load backup index
   3. Prepare destination directory
   4. Download and restore files
   5. Verify restored files
   6. Finalize restore operation
📋 Task 1: Initializing restore manager
   - Loading configuration
   - Initializing components
✅ Task 1 completed: Restore manager initialized
📋 Task 2: Loading backup index
   - Connecting to storage
   - Downloading backup index
   - Parsing index data
✅ Task 2 completed: Index loaded with 5 files
📋 Task 3: Preparing destination directory
   - Checking destination: /tmp/restore
   - Creating directory structure
✅ Task 3 completed: Destination directory ready
📋 Task 4: Downloading and restoring files
   - Total files to restore: 5
   - Processing files in parallel
   - Downloading from storage
   - Decrypting and decompressing
   - Writing to destination
   - Starting parallel processing with 16 workers
   - Processing file: document.txt (2.5 MB)
   - All files restored successfully
✅ Task 4 completed: All files restored
📋 Task 5: Verifying restored files
   - Checking file integrity
   - Validating file sizes
   - Verifying file permissions
📋 Task 6: Finalizing restore operation
   - Cleaning up temporary files
   - Finalizing restore
✅ Task 6 completed: Restore operation finalized
🎯 Restore completed successfully!
   ✅ All files restored to: /tmp/restore
   ✅ File integrity verified
   ✅ Restore operation completed
```

## 🎉 Conclusion

Les **logs de debug détaillés** de BCRDF offrent une **visibilité complète** sur les opérations de backup et restore, permettant de :

- **Diagnostiquer** rapidement les problèmes
- **Optimiser** les performances
- **Comprendre** le fonctionnement interne
- **Supporter** efficacement les utilisateurs

Utilisez le mode `-v` (verbose) pour activer ces logs détaillés et bénéficier d'une **transparence totale** sur les opérations de BCRDF ! 🚀

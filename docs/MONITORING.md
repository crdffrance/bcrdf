# 📊 Monitoring Automatique - BCRDF

## 📋 Vue d'ensemble

BCRDF dispose maintenant d'un **monitoring automatique** qui affiche les statistiques détaillées toutes les **5 minutes** pendant les opérations de backup et restore. Cette fonctionnalité permet de :

- **Détecter les blocages** en surveillant la dernière activité
- **Suivre la progression** en temps réel
- **Identifier les goulots d'étranglement** via la vitesse de traitement
- **Diagnostiquer les problèmes** avec des statistiques précises

## 🔧 Corrections Apportées

### **Problème Identifié**
Le monitoring initial présentait des **conflits** entre les statistiques globales et individuelles des fichiers chunkés, causant :
- **Affichage multiple** des statistiques pour le même fichier
- **Confusion** dans l'interprétation des données
- **Détection de blocage** incorrecte

### **Solutions Implémentées**

#### **1. Monitoring Séparé**
- **Monitoring global** : 5 minutes pour l'ensemble des fichiers
- **Monitoring spécifique** : 2 minutes pour les fichiers chunkés
- **Pas de conflits** entre les deux types de monitoring

#### **2. Arrêt Propre**
- **Canal de stop** : Arrêt propre du monitoring à la fin des opérations
- **Pas de fuites** de goroutines
- **Nettoyage automatique** des ressources

#### **3. Statistiques Précises**
- **Par fichier** : Statistiques individuelles pour chaque fichier chunké
- **Global** : Vue d'ensemble de l'opération complète
- **Thread-safe** : Mutex pour éviter les conflits d'accès

#### **4. Gestion des Fichiers Problématiques**
- **Fichiers vides** : Détection et ignorance automatique
- **Fichiers supprimés** : Détection et gestion
- **Timeouts** : Éviter les blocages infinis

## 🗂️ Application Automatique de la Rétention

### **Nouvelle Fonctionnalité**
BCRDF applique maintenant **automatiquement** la politique de rétention à la fin de chaque backup, permettant :

- **Nettoyage automatique** des anciens backups
- **Économie d'espace** de stockage
- **Respect des politiques** de rétention configurées
- **Logs détaillés** pour le suivi

### **Intégration dans le Workflow**
```
📋 Task 1: Initialize backup manager
📋 Task 2: Create current file index
📋 Task 3: Find previous backup (if exists)
📋 Task 4: Calculate file differences
📋 Task 5: Backup new/modified files
📋 Task 6: Create and save backup index
📋 Task 7: Apply retention policy  ← NOUVEAU
```

### **Logs de Rétention**
```
📋 Task 7: Applying retention policy
   - Loading retention configuration
   - Finding old backups
   - Deleting expired backups
✅ Task 7 completed: Retention policy applied
```

### **Gestion d'Erreurs**
- **Backup continue** même si la rétention échoue
- **Logs d'erreur** détaillés en mode verbose
- **Avertissements** sans interruption du processus

## 🚀 Activation du Monitoring

### **Mode Verbose avec Monitoring**
```bash
# Backup avec monitoring automatique
./bcrdf backup -n "mon-backup" -s "/path/to/data" --config config.yaml -v

# Restore avec monitoring automatique
./bcrdf restore -b "backup-id" -d "/restore/path" --config config.yaml -v
```

## 📊 Statistiques de Monitoring

### **Informations Affichées**

#### **1. Statut Général**
```
📊 BACKUP MONITORING - Processing files in parallel
```

#### **2. Temps Écoulé**
```
⏱️  Elapsed time: 15m30s
```

#### **3. Progression des Fichiers**
```
📁 Files: 3/5 (60.0%)
```

#### **4. Taille Traitée**
```
📦 Size: 150.25 MB / 250.50 MB
```

#### **5. Fichier en Cours**
```
🔄 Current file: large-file.bin (100.00 MB)
```

#### **6. Progression des Chunks** (pour gros fichiers)
```
📦 Chunks: 45/100 (45.0%)
```

#### **7. Dernière Activité**
```
🕐 Last activity: 2s ago
```

#### **8. Vitesse de Traitement**
```
📈 Processing speed: 9.68 MB/s
```

## 🔍 Détection des Blocages

### **Indicateurs de Blocage**

#### **1. Dernière Activité Ancienne**
```
🕐 Last activity: 5m30s ago  # ⚠️ Possible blocage
```

#### **2. Vitesse de Traitement Faible**
```
📈 Processing speed: 0.5 MB/s  # ⚠️ Performance dégradée
```

#### **3. Progression Figée**
```
📁 Files: 3/5 (60.0%)  # ⚠️ Pas de progression depuis 10 minutes
```

### **Actions Recommandées**

#### **Si Blocage Détecté :**
1. **Vérifier la connectivité réseau**
2. **Contrôler l'espace disque**
3. **Surveiller l'usage CPU/mémoire**
4. **Vérifier les logs d'erreur**

## 📈 Exemples de Monitoring

### **Backup Normal**
```
📊 BACKUP MONITORING - Processing files in parallel
   ⏱️  Elapsed time: 5m15s
   📁 Files: 12/25 (48.0%)
   📦 Size: 45.67 MB / 95.23 MB
   🔄 Current file: document.pdf (2.5 MB)
   🕐 Last activity: 30s ago
   📈 Processing speed: 8.67 MB/s
```

### **Backup avec Chunking**
```
📊 BACKUP MONITORING - Processing ultra-large file: large-file.bin
   ⏱️  Elapsed time: 25m45s
   📁 Files: 1/1 (100.0%)
   📦 Size: 4500.00 MB / 5000.00 MB
   🔄 Current file: large-file.bin (5000.00 MB)
   📦 Chunks: 90/100 (90.0%)
   🕐 Last activity: 15s ago
   📈 Processing speed: 3.25 MB/s
```

### **Restore Normal**
```
📊 RESTORE MONITORING - Processing files in parallel
   ⏱️  Elapsed time: 3m20s
   📁 Files: 8/15 (53.3%)
   📦 Size: 67.89 MB / 127.45 MB
   🔄 Current file: image.jpg (15.2 MB)
   🕐 Last activity: 45s ago
   📈 Processing speed: 20.34 MB/s
```

### **Restore avec Chunking**
```
📊 RESTORE MONITORING - Restoring chunked file: large-file.bin
   ⏱️  Elapsed time: 18m30s
   📁 Files: 1/1 (100.0%)
   📦 Size: 4200.00 MB / 5000.00 MB
   🔄 Current file: large-file.bin (5000.00 MB)
   📦 Chunks: 84/100 (84.0%)
   🕐 Last activity: 8s ago
   📈 Processing speed: 4.56 MB/s
```

## 🎯 Avantages du Monitoring

### **1. Détection Précoce des Problèmes**
- **Surveillance continue** sans intervention manuelle
- **Alertes automatiques** via la dernière activité
- **Diagnostic rapide** des goulots d'étranglement

### **2. Optimisation des Performances**
- **Suivi de la vitesse** de traitement
- **Identification** des facteurs limitants
- **Ajustement** des paramètres en temps réel

### **3. Transparence Opérationnelle**
- **Visibilité complète** sur les opérations
- **Statistiques précises** pour l'analyse
- **Historique** des performances

### **4. Support et Débogage**
- **Informations détaillées** pour le support
- **Traçabilité** des problèmes
- **Documentation** automatique des opérations

### **5. Gestion Automatique de la Rétention**
- **Nettoyage automatique** des anciens backups
- **Économie d'espace** de stockage
- **Respect des politiques** de rétention
- **Logs détaillés** pour le suivi

## 🧪 Test du Monitoring

### **Script de Test Automatisé**
```bash
# Exécuter le test complet
./test-monitoring-fixed.sh

# Test de la rétention automatique
./test-retention-auto.sh
```

### **Test Manuel**
```bash
# Créer des données de test
mkdir -p /tmp/test-monitoring
dd if=/dev/urandom of=/tmp/test-monitoring/large-file.bin bs=1M count=100

# Backup avec monitoring
./bcrdf backup -n "test-monitoring" -s "/tmp/test-monitoring" --config config.yaml -v

# Restore avec monitoring
./bcrdf restore -b "test-monitoring-backup-id" -d "/tmp/restore-monitoring" --config config.yaml -v
```

## 📊 Interprétation des Statistiques

### **Vitesse de Traitement Normale**
- **Backup** : 5-15 MB/s (selon la compression et le réseau)
- **Restore** : 15-30 MB/s (décompression + réseau)

### **Signaux d'Alerte**
- **Vitesse < 1 MB/s** : Problème réseau ou disque
- **Dernière activité > 5 minutes** : Blocage probable
- **Progression figée** : Erreur dans le traitement

### **Optimisations Possibles**
- **Augmenter `max_workers`** si CPU sous-utilisé
- **Réduire `chunk_size`** si mémoire limitée
- **Ajuster `compression_level`** selon les performances

## 🔧 Configuration Avancée

### **Intervalle de Monitoring**
Le monitoring s'affiche toutes les **5 minutes** par défaut pour le global et **2 minutes** pour les chunks. Pour modifier ces intervalles, éditez le code source :

```go
// Dans internal/backup/manager.go et internal/restore/manager.go
ticker := time.NewTicker(5 * time.Minute) // Monitoring global
ticker := time.NewTicker(2 * time.Minute) // Monitoring chunks
```

### **Activation Conditionnelle**
Le monitoring ne s'active qu'en mode **verbose** (`-v`) pour éviter le spam de logs.

### **Configuration de la Rétention**
La rétention automatique utilise les paramètres de votre fichier de configuration :

```yaml
retention:
  max_backups: 10
  max_age_days: 30
  keep_daily: 7
  keep_weekly: 4
  keep_monthly: 12
```

## 📈 Métriques de Performance

### **Backup Performance**
- **Fichiers petits** : 10-50 MB/s
- **Fichiers moyens** : 5-15 MB/s
- **Fichiers gros** : 2-8 MB/s (avec chunking)

### **Restore Performance**
- **Fichiers petits** : 20-100 MB/s
- **Fichiers moyens** : 15-50 MB/s
- **Fichiers gros** : 5-20 MB/s (avec chunking)

### **Facteurs Influençant la Performance**
- **Réseau** : Bande passante et latence
- **Stockage** : Type de disque et IOPS
- **CPU** : Puissance de chiffrement/compression
- **Mémoire** : Taille des buffers

## 🎉 Conclusion

Le **monitoring automatique** de BCRDF offre une **visibilité complète** sur les opérations de backup et restore, permettant de :

- **Détecter** rapidement les blocages
- **Optimiser** les performances
- **Diagnostiquer** les problèmes
- **Supporter** efficacement les utilisateurs
- **Gérer automatiquement** la rétention

Utilisez le mode `-v` (verbose) pour activer le monitoring et bénéficier d'un **suivi automatique** de vos opérations BCRDF ! 🚀

## 💡 Conseils d'Utilisation

### **Pour les Administrateurs**
- **Surveillez** la "Last activity" pour détecter les blocages
- **Analysez** la vitesse de traitement pour optimiser
- **Utilisez** les statistiques pour planifier les ressources
- **Configurez** la rétention selon vos besoins

### **Pour le Support**
- **Collectez** les logs de monitoring pour le diagnostic
- **Identifiez** les patterns de performance
- **Documentez** les cas d'usage spécifiques
- **Surveillez** les logs de rétention

### **Pour les Développeurs**
- **Intégrez** le monitoring dans vos tests
- **Analysez** les métriques pour l'optimisation
- **Étendez** les statistiques selon vos besoins
- **Personnalisez** les règles de rétention

## 🔧 Dépannage

### **Monitoring Ne S'Affiche Pas**
- **Vérifiez** que le mode `-v` est activé
- **Contrôlez** que les logs ne sont pas filtrés
- **Assurez-vous** que l'opération est en cours

### **Statistiques Incorrectes**
- **Redémarrez** l'opération si nécessaire
- **Vérifiez** la configuration du monitoring
- **Consultez** les logs de debug pour plus de détails

### **Blocage Détecté**
- **Vérifiez** la connectivité réseau
- **Contrôlez** l'espace disque disponible
- **Surveillez** l'usage CPU et mémoire
- **Consultez** les logs d'erreur système

### **Rétention Ne S'Applique Pas**
- **Vérifiez** la configuration de rétention
- **Contrôlez** les permissions de stockage
- **Consultez** les logs d'erreur de rétention
- **Assurez-vous** que les règles sont correctes

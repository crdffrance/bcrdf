package retention

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"bcrdf/internal/index"
	"bcrdf/pkg/storage"
	"bcrdf/pkg/utils"
)

// Manager gère les politiques de rétention des sauvegardes
type Manager struct {
	config        *utils.Config
	indexMgr      *index.Manager
	storageClient storage.Client
}

// BackupInfo contient les informations d'une sauvegarde pour la rétention
type BackupInfo struct {
	ID        string
	Timestamp time.Time
	Index     *index.BackupIndex
}

// NewManager crée un nouveau gestionnaire de rétention
func NewManager(config *utils.Config, indexMgr *index.Manager, storageClient storage.Client) *Manager {
	return &Manager{
		config:        config,
		indexMgr:      indexMgr,
		storageClient: storageClient,
	}
}

// ApplyRetentionPolicy applique la politique de rétention configurée
func (m *Manager) ApplyRetentionPolicy(verbose bool) error {
	return m.ApplyRetentionPolicyForBackup("", verbose)
}

// ApplyRetentionPolicyForBackup applique la politique de rétention pour un nom de backup spécifique
func (m *Manager) ApplyRetentionPolicyForBackup(backupName string, verbose bool) error {
	if verbose {
		utils.Info("🧹 Applying retention policy...")
		if backupName != "" {
			utils.Info("   - Backup name: %s", backupName)
		}
		utils.Info("   - Max age: %d days", m.config.Retention.Days)
		utils.Info("   - Max backups: %d", m.config.Retention.MaxBackups)
	} else {
		utils.ProgressStep("🧹 Applying retention policy")
	}

	// Récupérer toutes les sauvegardes disponibles
	allBackups, err := m.getAllBackups(verbose)
	if err != nil {
		return fmt.Errorf("error getting backups: %w", err)
	}

	if verbose {
		utils.Info("Found %d total backups in storage", len(allBackups))
	}

	// Si un nom de backup est spécifié, filtrer uniquement pour ce nom
	var backupsToProcess []BackupInfo
	if backupName != "" {
		backupsToProcess = m.filterBackupsByName(allBackups, backupName)
		if verbose {
			utils.Info("Filtered backups for name '%s': %d found", backupName, len(backupsToProcess))
		}

		// Si aucun backup trouvé pour ce nom, ne rien faire
		if len(backupsToProcess) == 0 {
			if verbose {
				utils.Info("No backups found for name '%s', nothing to clean up", backupName)
			} else {
				utils.ProgressSuccess("No backups to clean up")
			}
			return nil
		}
	} else {
		// Si aucun nom spécifié, traiter tous les backups (comportement par défaut)
		backupsToProcess = allBackups
		if verbose {
			utils.Info("Processing all backups (no specific name filter)")
		}
	}

	// Trier les sauvegardes par date (plus récent en premier)
	sort.Slice(backupsToProcess, func(i, j int) bool {
		return backupsToProcess[i].Timestamp.After(backupsToProcess[j].Timestamp)
	})

	if verbose {
		utils.Info("Backups sorted by date (newest first):")
		for i, backup := range backupsToProcess {
			if i < 5 { // Afficher seulement les 5 premiers pour éviter le spam
				age := time.Since(backup.Timestamp).Round(time.Hour)
				utils.Info("   %d. %s (age: %v)", i+1, backup.ID, age)
			}
		}
		if len(backupsToProcess) > 5 {
			utils.Info("   ... and %d more backups", len(backupsToProcess)-5)
		}
	}

	// Identifier les sauvegardes à supprimer
	toDelete := m.identifyBackupsToDelete(backupsToProcess, verbose)

	if len(toDelete) == 0 {
		if verbose {
			utils.Info("✅ No backups need to be deleted")
		} else {
			utils.ProgressSuccess("Retention policy satisfied")
		}
		return nil
	}

	if verbose {
		utils.Info("🗑️  Backups marked for deletion:")
		for _, backup := range toDelete {
			age := time.Since(backup.Timestamp).Round(time.Hour)
			utils.Info("   - %s (age: %v)", backup.ID, age)
		}
	}

	// Supprimer les sauvegardes identifiées
	return m.deleteBackups(toDelete, verbose)
}

// getAllBackups récupère toutes les sauvegardes disponibles
func (m *Manager) getAllBackups(verbose bool) ([]BackupInfo, error) {
	// Lister tous les objets dans le préfixe indexes/
	objects, err := m.storageClient.ListObjects("indexes/")
	if err != nil {
		return nil, fmt.Errorf("error listing backup indexes: %w", err)
	}

	var backups []BackupInfo

	for _, obj := range objects {
		// Extraire l'ID de sauvegarde du nom de fichier
		if !strings.HasSuffix(obj.Key, ".json") {
			continue
		}

		backupID := strings.TrimPrefix(obj.Key, "indexes/")
		backupID = strings.TrimSuffix(backupID, ".json")

		// Parser la date à partir de l'ID de sauvegarde (sans télécharger l'index)
		timestamp, err := m.parseBackupTimestamp(backupID)
		if err != nil {
			if verbose {
				utils.Warn("Cannot parse timestamp from backup ID %s: %v", backupID, err)
			}
			continue
		}

		// Optimisation : Ne pas charger l'index complet pour la rétention
		// On utilise seulement l'ID et le timestamp pour l'analyse
		backups = append(backups, BackupInfo{
			ID:        backupID,
			Timestamp: timestamp,
			Index:     nil, // Index chargé seulement si nécessaire
		})
	}

	if verbose {
		utils.Info("Found %d backups for retention analysis (without downloading indexes)", len(backups))
	}

	return backups, nil
}

// parseBackupTimestamp extrait la date d'un ID de sauvegarde
func (m *Manager) parseBackupTimestamp(backupID string) (time.Time, error) {
	// Format attendu: backup-name-20060102-150405
	parts := strings.Split(backupID, "-")
	if len(parts) < 3 {
		return time.Time{}, fmt.Errorf("invalid backup ID format: %s", backupID)
	}

	// Prendre les deux dernières parties (date et heure)
	dateStr := parts[len(parts)-2]
	timeStr := parts[len(parts)-1]

	timestampStr := fmt.Sprintf("%s-%s", dateStr, timeStr)

	return time.Parse("20060102-150405", timestampStr)
}

// identifyBackupsToDelete identifie les sauvegardes à supprimer selon la politique
func (m *Manager) identifyBackupsToDelete(backups []BackupInfo, verbose bool) []BackupInfo {
	var toDelete []BackupInfo
	now := time.Now()
	maxAge := time.Duration(m.config.Retention.Days) * 24 * time.Hour
	maxBackups := m.config.Retention.MaxBackups

	// Appliquer la politique de nombre maximum de sauvegardes
	if len(backups) > maxBackups {
		// Les sauvegardes sont triées par date (plus récent en premier)
		// Donc on doit supprimer les plus anciennes (dernières du slice)
		excessCount := len(backups) - maxBackups
		excessBackups := backups[len(backups)-excessCount:]
		toDelete = append(toDelete, excessBackups...)
		if verbose {
			utils.Info("Marking %d backups for deletion (exceeds max_backups: %d)",
				len(excessBackups), maxBackups)
		}
	}

	// Appliquer la politique d'âge maximum
	cutoffTime := now.Add(-maxAge)
	for _, backup := range backups {
		if backup.Timestamp.Before(cutoffTime) {
			// Vérifier si cette sauvegarde n'est pas déjà marquée pour suppression
			alreadyMarked := false
			for _, marked := range toDelete {
				if marked.ID == backup.ID {
					alreadyMarked = true
					break
				}
			}

			if !alreadyMarked {
				toDelete = append(toDelete, backup)
				if verbose {
					age := now.Sub(backup.Timestamp)
					utils.Info("Marking backup %s for deletion (age: %v, max: %v)",
						backup.ID, age.Round(time.Hour), maxAge.Round(time.Hour))
				}
			}
		}
	}

	return toDelete
}

// deleteBackups supprime une liste de sauvegardes
func (m *Manager) deleteBackups(backups []BackupInfo, verbose bool) error {
	deletedCount := 0
	var errors []string

	for _, backup := range backups {
		if err := m.deleteSingleBackup(backup, verbose); err != nil {
			errors = append(errors, err.Error())
			continue
		}
		deletedCount++
	}

	return m.reportDeletionResults(deletedCount, errors, verbose)
}

// filterBackupsByName filtre les sauvegardes par nom
func (m *Manager) filterBackupsByName(backups []BackupInfo, backupName string) []BackupInfo {
	if backupName == "" {
		return backups
	}

	var filtered []BackupInfo
	for _, backup := range backups {
		// Extraire le nom de backup de l'ID (format: backup-name-20060102-150405)
		parts := strings.Split(backup.ID, "-")
		if len(parts) < 3 {
			continue
		}

		// Reconstruire le nom de backup (tous les éléments sauf les 2 derniers)
		extractedBackupName := strings.Join(parts[:len(parts)-2], "-")
		if extractedBackupName == backupName {
			filtered = append(filtered, backup)
		}
	}

	return filtered
}

// deleteSingleBackup deletes a single backup
func (m *Manager) deleteSingleBackup(backup BackupInfo, verbose bool) error {
	m.logDeletionStart(backup, verbose)

	backupIndex, err := m.loadBackupIndexIfNeeded(backup)
	if err != nil {
		return fmt.Errorf("error loading index for %s: %v", backup.ID, err)
	}

	if err := m.deleteBackupData(backup, backupIndex, verbose); err != nil {
		return fmt.Errorf("error deleting files for %s: %v", backup.ID, err)
	}

	if err := m.deleteBackupIndex(backup.ID); err != nil {
		return fmt.Errorf("error deleting index for %s: %v", backup.ID, err)
	}

	m.logDeletionSuccess(backup, verbose)
	return nil
}

// logDeletionStart logs the start of backup deletion
func (m *Manager) logDeletionStart(backup BackupInfo, verbose bool) {
	if verbose {
		utils.Info("Deleting backup: %s (age: %v)", backup.ID, time.Since(backup.Timestamp).Round(time.Hour))
	} else {
		utils.ProgressStep(fmt.Sprintf("Deleting backup: %s", backup.ID))
	}
}

// loadBackupIndexIfNeeded loads the backup index if not already loaded
func (m *Manager) loadBackupIndexIfNeeded(backup BackupInfo) (*index.BackupIndex, error) {
	if backup.Index != nil {
		return backup.Index, nil
	}

	return m.indexMgr.LoadIndex(backup.ID)
}

// deleteBackupData deletes the backup data files
func (m *Manager) deleteBackupData(backup BackupInfo, backupIndex *index.BackupIndex, verbose bool) error {
	if verbose {
		utils.Info("Deleting %d files for backup %s", len(backupIndex.Files), backup.ID)
	}

	if err := m.deleteBackupFiles(backupIndex); err != nil {
		return err
	}

	if verbose {
		utils.Info("✅ All files deleted for backup %s", backup.ID)
	}

	return nil
}

// logDeletionSuccess logs successful deletion
func (m *Manager) logDeletionSuccess(backup BackupInfo, verbose bool) {
	if verbose {
		utils.Info("✅ Index deleted for backup %s", backup.ID)
		utils.Info("✅ Backup %s deleted successfully", backup.ID)
	}
}

// reportDeletionResults reports the final deletion results
func (m *Manager) reportDeletionResults(deletedCount int, errors []string, verbose bool) error {
	if len(errors) > 0 {
		m.reportErrors(deletedCount, errors, verbose)
		return fmt.Errorf("retention cleanup completed with %d errors", len(errors))
	}

	m.reportSuccess(deletedCount, verbose)
	return nil
}

// reportErrors reports deletion errors
func (m *Manager) reportErrors(deletedCount int, errors []string, verbose bool) {
	if verbose {
		utils.Warn("Retention cleanup completed with %d errors:", len(errors))
		for _, err := range errors {
			utils.Warn("  - %s", err)
		}
	} else {
		utils.ProgressWarning(fmt.Sprintf("Deleted %d backups with %d errors", deletedCount, len(errors)))
	}
}

// reportSuccess reports successful deletion
func (m *Manager) reportSuccess(deletedCount int, verbose bool) {
	if verbose {
		utils.Info("✅ Retention cleanup completed: %d backups deleted", deletedCount)
	} else {
		utils.ProgressSuccess(fmt.Sprintf("Retention cleanup: %d backups deleted", deletedCount))
	}
}

// deleteBackupFiles supprime les fichiers de données d'une sauvegarde
func (m *Manager) deleteBackupFiles(backupIndex *index.BackupIndex) error {
	var errors []string

	for _, file := range backupIndex.Files {
		if file.StorageKey != "" {
			// Reconstruct the full storage key with prefix
			fullStorageKey := fmt.Sprintf("data/%s/%s", backupIndex.BackupID, file.StorageKey)

			// Check if this is a chunked file by trying to download metadata
			metadataKey := fmt.Sprintf("%s.metadata", fullStorageKey)
			_, err := m.storageClient.Download(metadataKey)
			if err == nil {
				// This is a chunked file, delete chunks and metadata
				if err := m.deleteChunkedFile(fullStorageKey); err != nil {
					errors = append(errors, fmt.Sprintf("failed to delete chunked file %s: %v", fullStorageKey, err))
				} else {
					utils.Debug("Chunked file deleted: %s (original: %s)", fullStorageKey, file.Path)
				}
			} else {
				// Standard file, delete directly
				if err := m.deleteWithRetry(fullStorageKey); err != nil {
					errors = append(errors, fmt.Sprintf("failed to delete %s: %v", fullStorageKey, err))
				} else {
					utils.Debug("File deleted: %s (original: %s)", fullStorageKey, file.Path)
				}
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors deleting files: %s", strings.Join(errors, "; "))
	}

	return nil
}

// deleteBackupIndex supprime l'index d'une sauvegarde
func (m *Manager) deleteBackupIndex(backupID string) error {
	indexKey := fmt.Sprintf("indexes/%s.json", backupID)
	if err := m.deleteWithRetry(indexKey); err != nil {
		return fmt.Errorf("error deleting index: %w", err)
	}

	utils.Debug("Index deleted: %s", indexKey)
	return nil
}

// GetRetentionInfo retourne des informations sur la politique de rétention actuelle
func (m *Manager) GetRetentionInfo(verbose bool) error {
	backups, err := m.getAllBackups(verbose)
	if err != nil {
		return fmt.Errorf("error getting backups: %w", err)
	}

	// Trier par date
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Timestamp.After(backups[j].Timestamp)
	})

	now := time.Now()
	maxAge := time.Duration(m.config.Retention.Days) * 24 * time.Hour
	cutoffTime := now.Add(-maxAge)

	fmt.Printf("\n📊 Retention Policy Status\n")
	fmt.Printf("==========================\n")
	fmt.Printf("Max backups: %d\n", m.config.Retention.MaxBackups)
	fmt.Printf("Max age: %d days\n", m.config.Retention.Days)
	fmt.Printf("Current backups: %d\n", len(backups))
	fmt.Printf("Cutoff date: %s\n\n", cutoffTime.Format("2006-01-02 15:04:05"))

	if len(backups) == 0 {
		fmt.Printf("No backups found.\n")
		return nil
	}

	fmt.Printf("Backup List:\n")
	fmt.Printf("------------\n")

	for i, backup := range backups {
		age := now.Sub(backup.Timestamp)
		status := "✅ Keep"

		if i >= m.config.Retention.MaxBackups {
			status = "🗑️  Delete (exceeds max count)"
		} else if backup.Timestamp.Before(cutoffTime) {
			status = "🗑️  Delete (too old)"
		}

		fmt.Printf("%d. %s (%s ago) - %s\n",
			i+1, backup.ID, age.Round(time.Hour), status)
	}

	fmt.Printf("\n")
	return nil
}

// deleteChunkedFile supprime un fichier chunké et ses métadonnées
func (m *Manager) deleteChunkedFile(fullStorageKey string) error {
	// Download metadata to get chunk count
	metadataKey := fmt.Sprintf("%s.metadata", fullStorageKey)
	metadataBytes, err := m.storageClient.Download(metadataKey)
	if err != nil {
		return fmt.Errorf("error downloading metadata: %w", err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return fmt.Errorf("error parsing metadata: %w", err)
	}

	chunks, ok := metadata["chunks"].(float64)
	if !ok {
		return fmt.Errorf("invalid metadata: chunks field not found")
	}

	totalChunks := int(chunks)

	// Delete all chunks
	for chunkNum := 0; chunkNum < totalChunks; chunkNum++ {
		chunkKey := fmt.Sprintf("%s.chunk.%03d", fullStorageKey, chunkNum)
		if err := m.deleteWithRetry(chunkKey); err != nil {
			utils.Debug("Warning: failed to delete chunk %s: %v", chunkKey, err)
		}
	}

	// Delete metadata file
	if err := m.deleteWithRetry(metadataKey); err != nil {
		return fmt.Errorf("error deleting metadata: %w", err)
	}

	return nil
}

// deleteWithRetry supprime un objet avec retry et timeout
func (m *Manager) deleteWithRetry(key string) error {
	// Timeout pour éviter les blocages infinis
	timeout := time.Duration(m.config.Backup.NetworkTimeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second // Default 30 seconds
	}

	// Configuration du retry
	maxRetries := m.config.Backup.RetryAttempts
	if maxRetries <= 0 {
		maxRetries = 1 // Au moins 1 tentative
	}

	baseDelay := time.Duration(m.config.Backup.RetryDelay) * time.Second
	if baseDelay <= 0 {
		baseDelay = 2 * time.Second // Default 2 seconds
	}

	var lastError error

	// Boucle de retry avec backoff exponentiel
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Calculer le délai avec backoff exponentiel
			delay := baseDelay * time.Duration(1<<(attempt-1))
			if delay > 60*time.Second { // Cap à 60 secondes
				delay = 60 * time.Second
			}

			utils.Debug("🔄 Delete retry attempt %d/%d for %s after %v delay",
				attempt+1, maxRetries, key, delay)

			time.Sleep(delay)
		}

		// Créer un contexte avec timeout pour cette tentative
		ctx, cancel := context.WithTimeout(context.Background(), timeout)

		// Canal pour le résultat
		resultChan := make(chan error, 1)

		// Exécuter la suppression en arrière-plan
		go func() {
			resultChan <- m.storageClient.DeleteObject(key)
		}()

		// Attendre avec timeout
		select {
		case err := <-resultChan:
			cancel()
			if err == nil {
				// Succès !
				if attempt > 0 {
					utils.Debug("✅ Delete succeeded on retry attempt %d for %s", attempt+1, key)
				}
				return nil
			}

			// Erreur, la stocker pour le log final
			lastError = err

			// Log de l'erreur
			if attempt < maxRetries-1 {
				utils.Debug("⚠️  Delete failed for %s (attempt %d/%d): %v",
					key, attempt+1, maxRetries, err)
			}

		case <-ctx.Done():
			cancel()
			lastError = fmt.Errorf("delete timeout after %v", timeout)

			if attempt < maxRetries-1 {
				utils.Debug("⚠️  Delete timeout for %s (attempt %d/%d) after %v",
					key, attempt+1, maxRetries, timeout)
			}
		}
	}

	// Toutes les tentatives ont échoué
	utils.Warn("❌ Delete failed for %s after %d attempts. Last error: %v",
		key, maxRetries, lastError)

	return fmt.Errorf("delete failed after %d attempts for %s: %w", maxRetries, key, lastError)
}

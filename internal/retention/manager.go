package retention

import (
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
	if verbose {
		utils.Info("🧹 Applying retention policy...")
		utils.Info("   - Max age: %d days", m.config.Retention.Days)
		utils.Info("   - Max backups: %d", m.config.Retention.MaxBackups)
	} else {
		utils.ProgressStep("🧹 Applying retention policy")
	}

	// Récupérer toutes les sauvegardes
	backups, err := m.getAllBackups(verbose)
	if err != nil {
		return fmt.Errorf("error getting backups: %w", err)
	}

	if len(backups) == 0 {
		if verbose {
			utils.Info("No backups found, nothing to clean up")
		}
		return nil
	}

	// Trier les sauvegardes par date (plus récent en premier)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Timestamp.After(backups[j].Timestamp)
	})

	// Identifier les sauvegardes à supprimer
	toDelete := m.identifyBackupsToDelete(backups, verbose)

	if len(toDelete) == 0 {
		if verbose {
			utils.Info("✅ No backups need to be deleted")
		} else {
			utils.ProgressSuccess("Retention policy satisfied")
		}
		return nil
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
		excessBackups := backups[maxBackups:]
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
		if verbose {
			utils.Info("Deleting backup: %s (age: %v)", backup.ID, time.Since(backup.Timestamp).Round(time.Hour))
		} else {
			utils.ProgressStep(fmt.Sprintf("Deleting backup: %s", backup.ID))
		}

		// Charger l'index seulement si nécessaire pour la suppression
		var backupIndex *index.BackupIndex
		if backup.Index == nil {
			// Charger l'index seulement pour la suppression
			loadedIndex, err := m.indexMgr.LoadIndex(backup.ID)
			if err != nil {
				errorMsg := fmt.Sprintf("error loading index for %s: %v", backup.ID, err)
				errors = append(errors, errorMsg)
				if verbose {
					utils.Warn("%s", errorMsg)
				}
				continue
			}
			backupIndex = loadedIndex
		} else {
			backupIndex = backup.Index
		}

		// Supprimer les fichiers de données
		if err := m.deleteBackupFiles(backupIndex); err != nil {
			errorMsg := fmt.Sprintf("error deleting files for %s: %v", backup.ID, err)
			errors = append(errors, errorMsg)
			if verbose {
				utils.Warn("%s", errorMsg)
			}
			continue
		}

		// Supprimer l'index
		if err := m.deleteBackupIndex(backup.ID); err != nil {
			errorMsg := fmt.Sprintf("error deleting index for %s: %v", backup.ID, err)
			errors = append(errors, errorMsg)
			if verbose {
				utils.Warn("%s", errorMsg)
			}
			continue
		}

		deletedCount++
		if verbose {
			utils.Info("✅ Backup %s deleted successfully", backup.ID)
		}
	}

	// Rapporter les résultats
	if len(errors) > 0 {
		if verbose {
			utils.Warn("Retention cleanup completed with %d errors:", len(errors))
			for _, err := range errors {
				utils.Warn("  - %s", err)
			}
		} else {
			utils.ProgressWarning(fmt.Sprintf("Deleted %d backups with %d errors", deletedCount, len(errors)))
		}
		return fmt.Errorf("retention cleanup completed with %d errors", len(errors))
	}

	if verbose {
		utils.Info("✅ Retention cleanup completed: %d backups deleted", deletedCount)
	} else {
		utils.ProgressSuccess(fmt.Sprintf("Retention cleanup: %d backups deleted", deletedCount))
	}

	return nil
}

// deleteBackupFiles supprime les fichiers de données d'une sauvegarde
func (m *Manager) deleteBackupFiles(backupIndex *index.BackupIndex) error {
	var errors []string

	for _, file := range backupIndex.Files {
		if file.StorageKey != "" {
			if err := m.storageClient.DeleteObject(file.StorageKey); err != nil {
				errors = append(errors, fmt.Sprintf("failed to delete %s: %v", file.StorageKey, err))
			} else {
				utils.Debug("File deleted: %s", file.StorageKey)
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
	if err := m.storageClient.DeleteObject(indexKey); err != nil {
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

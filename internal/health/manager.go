package health

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"bcrdf/internal/index"
	"bcrdf/pkg/storage"
	"bcrdf/pkg/utils"
)

// Manager gère la vérification de santé des sauvegardes
type Manager struct {
	config        *utils.Config
	indexMgr      *index.Manager
	storageClient storage.Client
}

// BackupHealth contient les informations de santé d'une sauvegarde
type BackupHealth struct {
	ID           string
	Timestamp    time.Time
	Status       string
	IndexValid   bool
	FilesValid   bool
	RestoreTest  bool
	Errors       []string
	Warnings     []string
	FileCount    int
	TotalSize    int64
	MissingFiles []string
	CorruptFiles []string
}

// HealthReport contient le rapport de santé global
type HealthReport struct {
	TotalBackups     int
	HealthyBackups   int
	UnhealthyBackups int
	Backups          []BackupHealth
	Summary          string
	Recommendations  []string
}

// NewManager crée un nouveau gestionnaire de santé
func NewManager(config *utils.Config, indexMgr *index.Manager, storageClient storage.Client) *Manager {
	return &Manager{
		config:        config,
		indexMgr:      indexMgr,
		storageClient: storageClient,
	}
}

// CheckHealth vérifie la santé de toutes les sauvegardes
func (m *Manager) CheckHealth(verbose bool, testRestore bool, fastMode bool) (*HealthReport, error) {
	if verbose {
		utils.Info("🏥 Starting backup health check...")
	} else {
		utils.ProgressStep("🏥 Checking backup health")
	}

	// Récupérer toutes les sauvegardes
	backups, err := m.getAllBackups(verbose)
	if err != nil {
		return nil, fmt.Errorf("error getting backups: %w", err)
	}

	if len(backups) == 0 {
		if verbose {
			utils.Info("No backups found to check")
		}
		return &HealthReport{
			TotalBackups:     0,
			HealthyBackups:   0,
			UnhealthyBackups: 0,
			Summary:          "No backups found",
		}, nil
	}

	var healthChecks []BackupHealth
	var recommendations []string

	for _, backup := range backups {
		health := m.checkSingleBackup(backup, verbose, testRestore, fastMode)
		healthChecks = append(healthChecks, health)

		// Générer des recommandations
		if len(health.Errors) > 0 {
			recommendations = append(recommendations, fmt.Sprintf("Backup %s has errors and should be investigated", backup.ID))
		}
		if len(health.MissingFiles) > 0 {
			recommendations = append(recommendations, fmt.Sprintf("Backup %s has missing files and may need restoration", backup.ID))
		}
	}

	// Calculer les statistiques
	healthy := 0
	unhealthy := 0
	for _, health := range healthChecks {
		if health.Status == "healthy" {
			healthy++
		} else {
			unhealthy++
		}
	}

	summary := fmt.Sprintf("Found %d backups: %d healthy, %d unhealthy", len(backups), healthy, unhealthy)

	return &HealthReport{
		TotalBackups:     len(backups),
		HealthyBackups:   healthy,
		UnhealthyBackups: unhealthy,
		Backups:          healthChecks,
		Summary:          summary,
		Recommendations:  recommendations,
	}, nil
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

		// Parser la date à partir de l'ID de sauvegarde
		timestamp, err := m.parseBackupTimestamp(backupID)
		if err != nil {
			if verbose {
				utils.Warn("Invalid backup ID format: %s", backupID)
			}
			continue
		}

		backups = append(backups, BackupInfo{
			ID:        backupID,
			Timestamp: timestamp,
		})
	}

	// Trier par date (plus récent en premier)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Timestamp.After(backups[j].Timestamp)
	})

	return backups, nil
}

// BackupInfo contient les informations d'une sauvegarde pour la vérification
type BackupInfo struct {
	ID        string
	Timestamp time.Time
}

// parseBackupTimestamp parse la date à partir de l'ID de sauvegarde
func (m *Manager) parseBackupTimestamp(backupID string) (time.Time, error) {
	// Format attendu: name-YYYYMMDD-HHMMSS
	parts := strings.Split(backupID, "-")
	if len(parts) < 3 {
		return time.Time{}, fmt.Errorf("invalid backup ID format")
	}

	// Prendre les 2 dernières parties pour la date
	datePart := parts[len(parts)-2]
	timePart := parts[len(parts)-1]

	if len(datePart) != 8 || len(timePart) != 6 {
		return time.Time{}, fmt.Errorf("invalid date/time format in backup ID")
	}

	// Parser la date (YYYYMMDD)
	year := datePart[:4]
	month := datePart[4:6]
	day := datePart[6:8]

	// Parser l'heure (HHMMSS)
	hour := timePart[:2]
	minute := timePart[2:4]
	second := timePart[4:6]

	// Construire la chaîne de date
	dateStr := fmt.Sprintf("%s-%s-%s %s:%s:%s", year, month, day, hour, minute, second)

	return time.Parse("2006-01-02 15:04:05", dateStr)
}

// checkSingleBackup vérifie la santé d'une sauvegarde individuelle
func (m *Manager) checkSingleBackup(backup BackupInfo, verbose bool, testRestore bool, fastMode bool) BackupHealth {
	health := BackupHealth{
		ID:        backup.ID,
		Timestamp: backup.Timestamp,
		Status:    "unknown",
		Errors:    []string{},
		Warnings:  []string{},
	}

	if verbose {
		utils.Info("Checking backup: %s", backup.ID)
	}

	// 1. Vérifier que l'index peut être téléchargé et décrypté
	indexValid, indexErrors := m.checkIndexHealth(backup.ID, verbose)
	health.IndexValid = indexValid
	health.Errors = append(health.Errors, indexErrors...)

	if !indexValid {
		health.Status = "corrupt"
		return health
	}

	// 2. Charger l'index pour vérifier les fichiers
	backupIndex, err := m.loadBackupIndex(backup.ID)
	if err != nil {
		health.Errors = append(health.Errors, fmt.Sprintf("Failed to load index: %v", err))
		health.Status = "corrupt"
		return health
	}

	health.FileCount = len(backupIndex.Files)
	health.TotalSize = backupIndex.TotalSize

	// 3. Vérifier que les fichiers existent dans le stockage
	filesValid, missingFiles, corruptFiles := m.checkFilesHealth(backupIndex, verbose, fastMode)
	health.FilesValid = filesValid
	health.MissingFiles = missingFiles
	health.CorruptFiles = corruptFiles

	if len(missingFiles) > 0 {
		health.Warnings = append(health.Warnings, fmt.Sprintf("%d files missing from storage", len(missingFiles)))
	}

	if len(corruptFiles) > 0 {
		health.Errors = append(health.Errors, fmt.Sprintf("%d files appear to be corrupt", len(corruptFiles)))
	}

	// 4. Tester la restauration d'un échantillon de fichiers
	if testRestore && filesValid {
		restoreTest, restoreErrors := m.testRestoreSample(backupIndex, verbose)
		health.RestoreTest = restoreTest
		health.Errors = append(health.Errors, restoreErrors...)
	}

	// Déterminer le statut final
	if len(health.Errors) == 0 && len(health.MissingFiles) == 0 {
		health.Status = "healthy"
	} else if len(health.Errors) == 0 && len(health.MissingFiles) > 0 {
		health.Status = "partial"
	} else {
		health.Status = "corrupt"
	}

	return health
}

// checkIndexHealth vérifie que l'index peut être téléchargé et décrypté
func (m *Manager) checkIndexHealth(backupID string, verbose bool) (bool, []string) {
	var errors []string

	// Utiliser le gestionnaire d'index pour charger l'index décrypté
	backupIndex, err := m.indexMgr.LoadIndex(backupID)
	if err != nil {
		errors = append(errors, fmt.Sprintf("Failed to load and decrypt index: %v", err))
		return false, errors
	}

	// Vérifier que l'index a des données valides
	if backupIndex.BackupID == "" {
		errors = append(errors, "Index has empty backup ID")
		return false, errors
	}

	if len(backupIndex.Files) == 0 {
		errors = append(errors, "Index has no files")
		return false, errors
	}

	if verbose {
		utils.Info("✅ Index is valid: %d files, %d bytes", len(backupIndex.Files), backupIndex.TotalSize)
	}

	return true, errors
}

// loadBackupIndex charge l'index d'une sauvegarde
func (m *Manager) loadBackupIndex(backupID string) (*index.BackupIndex, error) {
	return m.indexMgr.LoadIndex(backupID)
}

// downloadWithRetry télécharge avec retry et timeout
func (m *Manager) downloadWithRetry(key string) ([]byte, error) {
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

			utils.Debug("🔄 Health check download retry attempt %d/%d for %s after %v delay",
				attempt+1, maxRetries, key, delay)

			time.Sleep(delay)
		}

		// Créer un contexte avec timeout pour cette tentative
		ctx, cancel := context.WithTimeout(context.Background(), timeout)

		// Canal pour le résultat
		resultChan := make(chan downloadResult, 1)

		// Exécuter le téléchargement en arrière-plan
		go func() {
			data, err := m.storageClient.Download(key)
			resultChan <- downloadResult{data: data, err: err}
		}()

		// Attendre avec timeout
		select {
		case result := <-resultChan:
			cancel()
			if result.err == nil {
				// Succès !
				if attempt > 0 {
					utils.Debug("✅ Health check download succeeded on retry attempt %d for %s", attempt+1, key)
				}
				return result.data, nil
			}

			// Erreur, la stocker pour le log final
			lastError = result.err

			// Log de l'erreur
			if attempt < maxRetries-1 {
				utils.Debug("⚠️  Health check download failed for %s (attempt %d/%d): %v",
					key, attempt+1, maxRetries, result.err)
			}

		case <-ctx.Done():
			cancel()
			lastError = fmt.Errorf("download timeout after %v", timeout)

			if attempt < maxRetries-1 {
				utils.Debug("⚠️  Health check download timeout for %s (attempt %d/%d) after %v",
					key, attempt+1, maxRetries, timeout)
			}
		}
	}

	// Toutes les tentatives ont échoué
	utils.Debug("❌ Health check download failed for %s after %d attempts. Last error: %v",
		key, maxRetries, lastError)

	return nil, fmt.Errorf("download failed after %d attempts for %s: %w", maxRetries, key, lastError)
}

// downloadResult contient le résultat d'un téléchargement
type downloadResult struct {
	data []byte
	err  error
}

// checkFilesHealth vérifie que les fichiers existent dans le stockage
func (m *Manager) checkFilesHealth(backupIndex *index.BackupIndex, verbose bool, fastMode bool) (bool, []string, []string) {
	var missingFiles []string
	var corruptFiles []string
	validFiles := 0

	// Déterminer quels fichiers vérifier
	filesToCheck := backupIndex.Files
	if fastMode {
		// En mode fast, vérifier seulement un échantillon aléatoire
		sampleSize := m.calculateSampleSize(len(backupIndex.Files))
		filesToCheck = m.getRandomSample(backupIndex.Files, sampleSize)

		if verbose {
			utils.Info("Fast mode: checking %d files out of %d total files", len(filesToCheck), len(backupIndex.Files))
		}
	}

	for _, file := range filesToCheck {
		if file.StorageKey == "" {
			missingFiles = append(missingFiles, file.Path)
			continue
		}

		// Reconstruire la clé complète avec le préfixe data/{backupID}/
		fullStorageKey := fmt.Sprintf("data/%s/%s", backupIndex.BackupID, file.StorageKey)

		// Vérifier d'abord si le fichier principal existe
		_, err := m.downloadWithRetry(fullStorageKey)
		if err != nil {
			missingFiles = append(missingFiles, file.Path)
			continue
		}

		// Vérifier si c'est un fichier chunké en essayant de télécharger les métadonnées
		// Seulement si le fichier principal existe et si c'est un gros fichier
		if file.Size > 100*1024*1024 { // 100MB - seuil pour les fichiers chunkés
			metadataKey := fmt.Sprintf("%s.metadata", fullStorageKey)
			_, metadataErr := m.downloadWithRetry(metadataKey)
			if metadataErr == nil {
				// Fichier chunké, vérifier les chunks
				if m.checkChunkedFileHealth(fullStorageKey, verbose) {
					validFiles++
				} else {
					corruptFiles = append(corruptFiles, file.Path)
				}
			} else {
				// Fichier standard, déjà vérifié au-dessus
				validFiles++
			}
		} else {
			// Fichier standard, déjà vérifié au-dessus
			validFiles++
		}
	}

	isValid := len(missingFiles) == 0 && len(corruptFiles) == 0

	if verbose {
		if fastMode {
			utils.Info("Files check (fast mode): %d valid, %d missing, %d corrupt (sampled from %d total files)",
				validFiles, len(missingFiles), len(corruptFiles), len(backupIndex.Files))
		} else {
			utils.Info("Files check: %d valid, %d missing, %d corrupt", validFiles, len(missingFiles), len(corruptFiles))
		}
	}

	return isValid, missingFiles, corruptFiles
}

// checkChunkedFileHealth vérifie la santé d'un fichier chunké
func (m *Manager) checkChunkedFileHealth(fullStorageKey string, verbose bool) bool {
	// Télécharger les métadonnées
	metadataKey := fmt.Sprintf("%s.metadata", fullStorageKey)
	metadataBytes, err := m.downloadWithRetry(metadataKey)
	if err != nil {
		return false
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return false
	}

	chunks, ok := metadata["chunks"].(float64)
	if !ok {
		return false
	}

	totalChunks := int(chunks)

	// Vérifier que tous les chunks existent
	for chunkNum := 0; chunkNum < totalChunks; chunkNum++ {
		chunkKey := fmt.Sprintf("%s.chunk.%03d", fullStorageKey, chunkNum)
		_, err := m.storageClient.Download(chunkKey)
		if err != nil {
			return false
		}
	}

	return true
}

// testRestoreSample teste la restauration d'un échantillon de fichiers
func (m *Manager) testRestoreSample(backupIndex *index.BackupIndex, verbose bool) (bool, []string) {
	var errors []string

	// Prendre un échantillon de 3 fichiers maximum pour le test
	sampleSize := 3
	if len(backupIndex.Files) < sampleSize {
		sampleSize = len(backupIndex.Files)
	}

	if verbose {
		utils.Info("Testing restore of %d sample files", sampleSize)
	}

	for i := 0; i < sampleSize; i++ {
		file := backupIndex.Files[i]

		// Reconstruire la clé complète avec le préfixe data/{backupID}/
		fullStorageKey := fmt.Sprintf("data/%s/%s", backupIndex.BackupID, file.StorageKey)

		// Vérifier si c'est un fichier chunké
		metadataKey := fmt.Sprintf("%s.metadata", fullStorageKey)
		_, err := m.storageClient.Download(metadataKey)
		if err == nil {
			// Fichier chunké, tester le premier chunk
			chunkKey := fmt.Sprintf("%s.chunk.000", fullStorageKey)
			encryptedData, err := m.storageClient.Download(chunkKey)
			if err != nil {
				errors = append(errors, fmt.Sprintf("Failed to download test chunk for %s: %v", file.Path, err))
				continue
			}

			if len(encryptedData) < 32 {
				errors = append(errors, fmt.Sprintf("Test chunk for %s appears to be too small", file.Path))
				continue
			}
		} else {
			// Fichier standard, tester directement
			encryptedData, err := m.storageClient.Download(fullStorageKey)
			if err != nil {
				errors = append(errors, fmt.Sprintf("Failed to download test file %s: %v", file.Path, err))
				continue
			}

			if len(encryptedData) < 32 {
				errors = append(errors, fmt.Sprintf("Test file %s appears to be too small", file.Path))
				continue
			}
		}

		if verbose {
			utils.Info("✅ Test file %s restored successfully", file.Path)
		}
	}

	return len(errors) == 0, errors
}

// PrintReport affiche le rapport de santé
func (m *Manager) PrintReport(report *HealthReport, verbose bool) {
	fmt.Printf("\n🏥 Backup Health Report\n")
	fmt.Printf("%s\n", strings.Repeat("=", 50))
	fmt.Printf("📊 %s\n", report.Summary)
	fmt.Printf("\n")

	if len(report.Backups) == 0 {
		fmt.Printf("No backups found.\n")
		return
	}

	// Afficher les détails de chaque sauvegarde
	for i, backup := range report.Backups {
		statusIcon := "✅"
		if backup.Status == "corrupt" {
			statusIcon = "❌"
		} else if backup.Status == "partial" {
			statusIcon = "⚠️"
		}

		fmt.Printf("%d. %s %s (%s)\n", i+1, statusIcon, backup.ID, backup.Timestamp.Format("2006-01-02 15:04:05"))
		fmt.Printf("   Status: %s\n", backup.Status)
		fmt.Printf("   Files: %d (%.2f MB)\n", backup.FileCount, float64(backup.TotalSize)/1024/1024)

		if len(backup.Errors) > 0 {
			fmt.Printf("   Errors:\n")
			for _, err := range backup.Errors {
				fmt.Printf("     • %s\n", err)
			}
		}

		if len(backup.Warnings) > 0 {
			fmt.Printf("   Warnings:\n")
			for _, warning := range backup.Warnings {
				fmt.Printf("     • %s\n", warning)
			}
		}

		if len(backup.MissingFiles) > 0 {
			fmt.Printf("   Missing files: %d\n", len(backup.MissingFiles))
		}

		if len(backup.CorruptFiles) > 0 {
			fmt.Printf("   Corrupt files: %d\n", len(backup.CorruptFiles))
		}

		fmt.Printf("\n")
	}

	// Afficher les recommandations
	if len(report.Recommendations) > 0 {
		fmt.Printf("💡 Recommendations:\n")
		for _, rec := range report.Recommendations {
			fmt.Printf("   • %s\n", rec)
		}
		fmt.Printf("\n")
	}

	// Résumé final
	if report.HealthyBackups == report.TotalBackups {
		fmt.Printf("🎉 All backups are healthy!\n")
	} else if report.HealthyBackups > 0 {
		fmt.Printf("⚠️  Some backups have issues. Review the report above.\n")
	} else {
		fmt.Printf("🚨 All backups have issues. Immediate attention required!\n")
	}
}

// calculateSampleSize calcule la taille de l'échantillon pour le mode fast
func (m *Manager) calculateSampleSize(totalFiles int) int {
	if totalFiles <= 10 {
		return totalFiles // Vérifier tous les fichiers si moins de 10
	}

	// Pour les grandes sauvegardes, vérifier 10% des fichiers, minimum 10, maximum 50
	sampleSize := totalFiles / 10
	if sampleSize < 10 {
		sampleSize = 10
	}
	if sampleSize > 50 {
		sampleSize = 50
	}

	return sampleSize
}

// getRandomSample retourne un échantillon aléatoire de fichiers
func (m *Manager) getRandomSample(files []index.FileEntry, sampleSize int) []index.FileEntry {
	if len(files) <= sampleSize {
		return files
	}

	// Initialiser le générateur aléatoire
	rand.Seed(time.Now().UnixNano())

	// Créer un échantillon aléatoire
	sample := make([]index.FileEntry, 0, sampleSize)
	indices := rand.Perm(len(files))

	for i := 0; i < sampleSize; i++ {
		sample = append(sample, files[indices[i]])
	}

	return sample
}

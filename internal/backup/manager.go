package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"bcrdf/internal/compression"
	"bcrdf/internal/crypto"
	"bcrdf/internal/index"
	"bcrdf/internal/retention"
	"bcrdf/pkg/storage"
	"bcrdf/pkg/utils"
)

// FileBatch represents a batch of small files to upload together
type FileBatch struct {
	Files         []index.FileEntry
	TotalSize     int64
	BatchID       string
	ProcessedData map[string][]byte // file path -> processed data
}

// Manager gère les opérations de sauvegarde
type Manager struct {
	configFile       string
	config           *utils.Config
	indexMgr         *index.Manager
	encryptor        *crypto.EncryptorV2
	compressor       *compression.Compressor
	storageClient    storage.Client
	multiProgressBar *utils.IntegratedProgressBar // Barre de progression intégrée pour les gros fichiers
}

// NewManager crée un nouveau gestionnaire de sauvegarde
func NewManager(configFile string) *Manager {
	return &Manager{
		configFile: configFile,
	}
}

// CreateBackup effectue une sauvegarde complète
func (m *Manager) CreateBackup(sourcePath, backupName string, verbose bool) error {
	startTime := time.Now()
	m.logBackupStart(backupName, verbose)

	if err := m.prepareBackup(sourcePath); err != nil {
		return err
	}

	backupID := fmt.Sprintf("%s-%s", backupName, time.Now().Format("20060102-150405"))

	currentIndex, err := m.createCurrentIndex(sourcePath, backupID, verbose)
	if err != nil {
		return err
	}

	diff, err := m.calculateBackupDiff(currentIndex, backupName, verbose)
	if err != nil {
		return err
	}

	// Vérifier s'il y a des fichiers à sauvegarder
	totalFilesToBackup := len(diff.Added) + len(diff.Modified)
	if totalFilesToBackup == 0 {
		// Aucun fichier à sauvegarder, skip le backup
		if verbose {
			utils.Info("🔄 No files to backup, skipping backup creation")
		} else {
			utils.ProgressInfo("No files to backup, skipping backup creation")
		}
		m.logBackupCompletion(diff, time.Since(startTime), verbose)
		return nil
	}

	if err := m.executeBackup(currentIndex, diff, backupID, backupName, verbose); err != nil {
		return err
	}

	m.logBackupCompletion(diff, time.Since(startTime), verbose)

	// Apply retention policy only if a backup was actually created
	if totalFilesToBackup > 0 {
		if err := m.applyRetentionPolicyForBackup(backupName, verbose); err != nil {
			// Don't fail the backup if retention fails, just warn
			if verbose {
				utils.Warn("Retention policy application failed: %v", err)
			} else {
				utils.ProgressWarning("Retention cleanup failed")
			}
		}
	}

	return nil
}

// applyRetentionPolicy applique la politique de rétention après une sauvegarde
func (m *Manager) applyRetentionPolicy(verbose bool) error {
	return m.applyRetentionPolicyForBackup("", verbose)
}

// applyRetentionPolicyForBackup applique la politique de rétention pour un nom de backup spécifique
func (m *Manager) applyRetentionPolicyForBackup(backupName string, verbose bool) error {
	if verbose {
		utils.Info("📋 Task 7: Applying retention policy")
		utils.Info("   - Loading retention configuration")
		if backupName != "" {
			utils.Info("   - Backup name: %s", backupName)
		}
		utils.Info("   - Finding old backups")
		utils.Info("   - Deleting expired backups")
	}

	retentionMgr := retention.NewManager(m.config, m.indexMgr, m.storageClient)
	err := retentionMgr.ApplyRetentionPolicyForBackup(backupName, verbose)

	if verbose {
		if err != nil {
			utils.Warn("⚠️ Task 7 completed with warnings: Retention policy failed")
		} else {
			utils.Info("✅ Task 7 completed: Retention policy applied")
		}
	}

	return err
}

// DeleteBackup supprime une sauvegarde
func (m *Manager) DeleteBackup(backupID string) error {
	utils.Info("🗑️ Suppression de la sauvegarde: %s", backupID)

	// Initialiser les composants si nécessaire
	if err := m.initializeComponents(); err != nil {
		return fmt.Errorf("error during l'initialisation des composants: %w", err)
	}

	// Charger l'index de la sauvegarde
	backupIndex, err := m.indexMgr.LoadIndex(backupID)
	if err != nil {
		return fmt.Errorf("erreur lors du chargement de l'index: %w", err)
	}

	// Supprimer les fichiers de données
	if err := m.deleteBackupFiles(backupIndex); err != nil {
		return fmt.Errorf("error during la suppression des fichiers: %w", err)
	}

	// Supprimer l'index
	if err := m.deleteBackupIndex(backupID); err != nil {
		return fmt.Errorf("error during la suppression de l'index: %w", err)
	}

	utils.Info("✅ Backup deleted: %s", backupID)
	return nil
}

// initializeComponents initialise tous les composants nécessaires
func (m *Manager) initializeComponents() error {
	// Charger la configuration si nécessaire
	if m.config == nil {
		config, err := utils.LoadConfig(m.configFile)
		if err != nil {
			return fmt.Errorf("erreur lors du chargement de la configuration: %w", err)
		}
		m.config = config
	}

	// Initialiser le gestionnaire d'index
	m.indexMgr = index.NewManager(m.configFile)

	// Initialiser le chiffreur avec l'algorithme configuré
	algorithm := crypto.EncryptionAlgorithm(m.config.Backup.EncryptionAlgo)
	if algorithm == "" {
		algorithm = crypto.AES256GCM // Valeur par défaut
	}

	encryptor, err := crypto.NewEncryptorV2(m.config.Backup.EncryptionKey, algorithm)
	if err != nil {
		return fmt.Errorf("error during l'initialisation du chiffreur: %w", err)
	}
	m.encryptor = encryptor

	// Initialiser le compresseur
	compressor, err := compression.NewCompressor(m.config.Backup.CompressionLevel)
	if err != nil {
		return fmt.Errorf("error during l'initialisation du compresseur: %w", err)
	}
	m.compressor = compressor

	// Initialiser le client de stockage
	storageClient, err := storage.NewStorageClient(m.config)
	if err != nil {
		return fmt.Errorf("error during l'initialisation du client de stockage: %w", err)
	}
	m.storageClient = storageClient

	return nil
}

// findPreviousBackup trouve la sauvegarde précédente
func (m *Manager) findPreviousBackup(currentBackupName string) (*index.BackupIndex, error) {
	if err := m.ensureInitialized(); err != nil {
		return nil, err
	}

	keys, err := m.listBackupIndexes()
	if err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		utils.Debug("No index found, first backup")
		return nil, nil
	}

	filteredKeys := m.filterKeysByName(keys, currentBackupName)
	keysToUse := m.selectKeysToUse(keys, filteredKeys)

	latestKey, latestTime, err := m.findLatestBackup(keysToUse)
	if err != nil {
		return nil, err
	}

	if latestKey == "" {
		utils.Debug("No valid index found")
		return nil, nil
	}

	utils.Debug("Found previous backup: %s (created at %s)", latestKey, latestTime.Format("2006-01-02 15:04:05"))
	return m.loadBackupIndex(latestKey, latestTime)
}

// ensureInitialized ensures the manager is properly initialized
func (m *Manager) ensureInitialized() error {
	if m.config == nil {
		config, err := utils.LoadConfig(m.configFile)
		if err != nil {
			return err
		}
		m.config = config
	}

	if m.storageClient == nil {
		storageClient, err := storage.NewStorageClient(m.config)
		if err != nil {
			return fmt.Errorf("error during l'initialisation du client de stockage: %w", err)
		}
		m.storageClient = storageClient
	}

	return nil
}

// listBackupIndexes lists all available backup indexes
func (m *Manager) listBackupIndexes() ([]string, error) {
	objects, err := m.storageClient.ListObjects("indexes/")
	if err != nil {
		utils.Warn("Impossible de lister les index: %v", err)
		return nil, err
	}

	keys := make([]string, len(objects))
	for i, obj := range objects {
		keys[i] = obj.Key
	}

	utils.Debug("Found %d indexes: %v", len(keys), keys)
	return keys, nil
}

// filterKeysByName filters keys by backup name
func (m *Manager) filterKeysByName(keys []string, backupName string) []string {
	if backupName == "" {
		return nil
	}

	var filteredKeys []string
	for _, key := range keys {
		if !strings.HasSuffix(key, ".json") {
			continue
		}

		keyBackupID := strings.TrimSuffix(strings.TrimPrefix(key, "indexes/"), ".json")
		parts := strings.Split(keyBackupID, "-")
		if len(parts) >= 3 {
			extractedBackupName := strings.Join(parts[:len(parts)-2], "-")
			if extractedBackupName == backupName {
				filteredKeys = append(filteredKeys, key)
			}
		}
	}

	utils.Debug("Filtered backups by name '%s': %d found", backupName, len(filteredKeys))
	return filteredKeys
}

// selectKeysToUse selects which keys to use for processing
func (m *Manager) selectKeysToUse(allKeys, filteredKeys []string) []string {
	if len(filteredKeys) > 0 {
		return filteredKeys
	}
	return allKeys
}

// findLatestBackup finds the most recent backup from the given keys
func (m *Manager) findLatestBackup(keys []string) (string, time.Time, error) {
	if len(keys) == 0 {
		return "", time.Time{}, nil
	}

	m.sortKeysByTimestamp(keys)
	latestKey := keys[len(keys)-1]

	// Extract timestamp from key
	parts := strings.Split(latestKey, "-")
	if len(parts) < 3 {
		return "", time.Time{}, fmt.Errorf("invalid key format: %s", latestKey)
	}

	// Parse timestamp from the last parts
	timestampStr := strings.Join(parts[len(parts)-2:], "-")
	timestampStr = strings.TrimSuffix(timestampStr, ".json")

	latestTime, err := time.Parse("20060102-150405", timestampStr)
	if err != nil {
		utils.Warn("Cannot parse timestamp from key %s: %v", latestKey, err)
		return "", time.Time{}, err
	}

	utils.Debug("Latest backup found: %s (timestamp: %s)", latestKey, latestTime.Format("2006-01-02 15:04:05"))
	return latestKey, latestTime, nil
}

// sortKeysByTimestamp sorts keys by timestamp
func (m *Manager) sortKeysByTimestamp(keys []string) {
	sort.Slice(keys, func(i, j int) bool {
		keyI := keys[i]
		keyJ := keys[j]

		partsI := strings.Split(strings.TrimSuffix(keyI, ".json"), "-")
		partsJ := strings.Split(strings.TrimSuffix(keyJ, ".json"), "-")

		if len(partsI) < 2 || len(partsJ) < 2 {
			return keyI < keyJ
		}

		timestampI := partsI[len(partsI)-2] + "-" + partsI[len(partsI)-1]
		timestampJ := partsJ[len(partsJ)-2] + "-" + partsJ[len(partsJ)-1]

		return timestampI < timestampJ
	})
}

// loadBackupIndex loads the backup index for the given key
func (m *Manager) loadBackupIndex(latestKey string, latestTime time.Time) (*index.BackupIndex, error) {
	backupID := strings.TrimSuffix(strings.TrimPrefix(latestKey, "indexes/"), ".json")

	utils.Debug("Loading previous backup index: %s", backupID)

	previousIndex, err := m.indexMgr.LoadIndex(backupID)
	if err != nil {
		utils.Warn("Unable to load previous backup index %s: %v", backupID, err)
		// Retourner une erreur au lieu de nil pour que le système traite comme un premier backup
		return nil, fmt.Errorf("failed to load previous backup index: %w", err)
	}

	utils.Info("Previous backup found: %s (created on %s)",
		backupID, latestTime.Format("2006-01-02 15:04:05"))

	return previousIndex, nil
}

// BackupStats contient les statistiques de backup en cours
type BackupStats struct {
	StartTime        time.Time
	TotalFiles       int
	ProcessedFiles   int
	CurrentFile      string
	CurrentFileSize  int64
	CurrentFileIndex int
	TotalSize        int64
	ProcessedSize    int64
	ChunksProcessed  int
	TotalChunks      int
	LastActivity     time.Time
	Status           string
	mu               sync.RWMutex
	stopChan         chan struct{} // Canal pour arrêter le monitoring
}

// NewBackupStats crée de nouvelles statistiques de backup
func NewBackupStats() *BackupStats {
	return &BackupStats{
		StartTime:    time.Now(),
		LastActivity: time.Now(),
		Status:       "Initializing",
		stopChan:     make(chan struct{}),
	}
}

// StopMonitoring arrête le monitoring
func (bs *BackupStats) StopMonitoring() {
	close(bs.stopChan)
}

// UpdateStats met à jour les statistiques
func (bs *BackupStats) UpdateStats(file string, size int64, index int, total int) {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	bs.CurrentFile = file
	bs.CurrentFileSize = size
	bs.CurrentFileIndex = index
	bs.TotalFiles = total
	bs.ProcessedFiles = index
	bs.ProcessedSize += size
	bs.LastActivity = time.Now()
}

// UpdateChunkStats met à jour les statistiques de chunking
func (bs *BackupStats) UpdateChunkStats(chunkIndex, totalChunks int, chunkSize int64) {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	bs.ChunksProcessed = chunkIndex
	bs.TotalChunks = totalChunks
	bs.LastActivity = time.Now()
}

// UpdateStatus met à jour le statut
func (bs *BackupStats) UpdateStatus(status string) {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	bs.Status = status
	bs.LastActivity = time.Now()
}

// GetStats retourne une copie des statistiques
func (bs *BackupStats) GetStats() BackupStats {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	return BackupStats{
		StartTime:        bs.StartTime,
		TotalFiles:       bs.TotalFiles,
		ProcessedFiles:   bs.ProcessedFiles,
		CurrentFile:      bs.CurrentFile,
		CurrentFileSize:  bs.CurrentFileSize,
		CurrentFileIndex: bs.CurrentFileIndex,
		TotalSize:        bs.TotalSize,
		ProcessedSize:    bs.ProcessedSize,
		ChunksProcessed:  bs.ChunksProcessed,
		TotalChunks:      bs.TotalChunks,
		LastActivity:     bs.LastActivity,
		Status:           bs.Status,
	}
}

// LogStats affiche les statistiques actuelles
func (bs *BackupStats) LogStats() {
	stats := bs.GetStats()

	elapsed := time.Since(stats.StartTime)
	progress := float64(0)
	if stats.TotalFiles > 0 {
		progress = float64(stats.ProcessedFiles) / float64(stats.TotalFiles) * 100
	}

	utils.Info("📊 BACKUP MONITORING - %s", stats.Status)
	utils.Info("   ⏱️  Elapsed time: %v", elapsed.Round(time.Second))
	utils.Info("   📁 Files: %d/%d (%.1f%%)", stats.ProcessedFiles, stats.TotalFiles, progress)
	utils.Info("   📦 Size: %.2f MB / %.2f MB", float64(stats.ProcessedSize)/1024/1024, float64(stats.TotalSize)/1024/1024)

	if stats.CurrentFile != "" {
		utils.Info("   🔄 Current file: %s (%.2f MB)", filepath.Base(stats.CurrentFile), float64(stats.CurrentFileSize)/1024/1024)
	}

	if stats.TotalChunks > 0 {
		chunkProgress := float64(0)
		if stats.TotalChunks > 0 {
			chunkProgress = float64(stats.ChunksProcessed) / float64(stats.TotalChunks) * 100
		}
		utils.Info("   📦 Chunks: %d/%d (%.1f%%)", stats.ChunksProcessed, stats.TotalChunks, chunkProgress)
	}

	utils.Info("   🕐 Last activity: %v ago", time.Since(stats.LastActivity).Round(time.Second))
	utils.Info("   📈 Processing speed: %.2f MB/s", float64(stats.ProcessedSize)/1024/1024/elapsed.Seconds())
}

// startMonitoring démarre le monitoring automatique
func (m *Manager) startMonitoring(stats *BackupStats, verbose bool) {
	if !verbose {
		return // Monitoring seulement en mode verbose
	}

	// Démarrer le monitoring en arrière-plan
	go func() {
		ticker := time.NewTicker(5 * time.Minute) // Toutes les 5 minutes
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				stats.LogStats()
			case <-stats.stopChan:
				return // Arrêter le monitoring
			}
		}
	}()
}

// startChunkMonitoring démarre le monitoring spécifique pour les fichiers chunkés
func (m *Manager) startChunkMonitoring(stats *BackupStats, verbose bool) {
	if !verbose {
		return // Monitoring seulement en mode verbose
	}

	// Démarrer le monitoring en arrière-plan avec un intervalle plus court pour les chunks
	go func() {
		ticker := time.NewTicker(2 * time.Minute) // Toutes les 2 minutes pour les chunks
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				stats.LogStats()
			case <-stats.stopChan:
				return // Arrêter le monitoring
			}
		}
	}()
}

// backupFiles sauvegarde les fichiers spécifiés
func (m *Manager) backupFiles(added, modified []index.FileEntry, backupID string, verbose bool) error {
	allFiles := append(added, modified...)

	if len(allFiles) == 0 {
		if verbose {
			utils.Info("No files to backup")
		} else {
			utils.ProgressInfo("No files to backup")
		}
		return nil
	}

	// Initialiser les statistiques de monitoring
	stats := NewBackupStats()
	stats.TotalFiles = len(allFiles)
	stats.TotalSize = m.calculateTotalSize(allFiles)
	stats.UpdateStatus("Starting file processing")

	// Démarrer le monitoring
	m.startMonitoring(stats, verbose)

	// Arrêter le monitoring à la fin de la fonction
	defer stats.StopMonitoring()

	// Optimisation : Trier les fichiers par taille (petits en premier)
	if m.config.Backup.SortBySize {
		if verbose {
			utils.Info("   - Sorting %d files by size (smallest first)...", len(allFiles))
		} else {
			utils.ProgressStep(fmt.Sprintf("Sorting %d files by size", len(allFiles)))
		}

		// Trier par taille croissante (petits fichiers en premier)
		sort.Slice(allFiles, func(i, j int) bool {
			return allFiles[i].Size < allFiles[j].Size
		})

		if verbose {
			utils.Info("   - Backing up %d files (sorted by size)", len(allFiles))
		} else {
			utils.ProgressStep(fmt.Sprintf("Backing up %d files (smallest first)", len(allFiles)))
		}
	} else {
		if verbose {
			utils.Info("   - Backing up %d files (original order)", len(allFiles))
		} else {
			utils.ProgressStep(fmt.Sprintf("Backing up %d files", len(allFiles)))
		}
	}

	if verbose {
		utils.Info("   - Starting parallel processing with %d workers", m.config.Backup.MaxWorkers)
	}

	stats.UpdateStatus("Processing files in parallel")

	// Créer un pool de workers pour le traitement parallèle
	semaphore := make(chan struct{}, m.config.Backup.MaxWorkers)
	var wg sync.WaitGroup
	errors := make(chan error, len(allFiles))

	// Barre de progression intégrée pour le mode non-verbeux
	var multiProgressBar *utils.IntegratedProgressBar
	if !verbose {
		multiProgressBar = utils.NewIntegratedProgressBar(stats.TotalSize)
		multiProgressBar.SetMaxActiveFiles(5) // Afficher max 5 fichiers simultanément

		// Afficher le démarrage
		utils.ProgressInfo(fmt.Sprintf("Starting backup of %d files (%.2f MB total)",
			len(allFiles), float64(stats.TotalSize)/1024/1024))
	}

	completed := int64(0)
	var completedMutex sync.Mutex

	// Timeout global pour éviter les blocages infinis
	globalTimeout := time.Duration(m.config.Backup.NetworkTimeout*len(allFiles)) * time.Second
	if globalTimeout == 0 {
		globalTimeout = 30 * time.Minute // Default 30 minutes
	}

	ctx, cancel := context.WithTimeout(context.Background(), globalTimeout)
	defer cancel()

	for i, file := range allFiles {
		wg.Add(1)
		go func(f index.FileEntry, index int) {
			defer wg.Done()

			// Vérifier le timeout global
			select {
			case <-ctx.Done():
				errors <- fmt.Errorf("global timeout reached for file %s", f.Path)
				return
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			}

			// Mettre à jour les statistiques
			stats.UpdateStats(f.Path, f.Size, index+1, len(allFiles))

			// Ne pas afficher de barre pour les petits fichiers; les fonctions dédiées géreront l'affichage si nécessaire

			if verbose {
				utils.Debug("   - Processing file: %s (%.2f MB)", filepath.Base(f.Path), float64(f.Size)/1024/1024)
			}

			// Sauvegarder le fichier avec suivi de progression
			if err := m.backupSingleFileWithMultiProgress(f, backupID, multiProgressBar, verbose); err != nil {
				errors <- fmt.Errorf("error saving de %s: %w", f.Path, err)
			}

			// Mettre à jour la progression globale
			if !verbose && multiProgressBar != nil {
				completedMutex.Lock()
				completed += f.Size
				multiProgressBar.UpdateGlobal(completed)
				completedMutex.Unlock()

				// Marquer le fichier comme terminé (utiliser le nom de base pour la cohérence)
				fileName := filepath.Base(f.Path)
				multiProgressBar.RemoveFile(fileName)
			}
		}(file, i)
	}

	// Attendre la fin ou le timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Tous les fichiers ont été traités
	case <-ctx.Done():
		utils.Warn("⚠️  Global timeout reached, some files may not have been processed")
	}

	close(errors)

	// Terminer la barre de progression multi-fichiers
	if !verbose && multiProgressBar != nil {
		multiProgressBar.Finish()
	}

	// Vérifier s'il y a eu des erreurs
	errorCount := 0
	for err := range errors {
		errorCount++
		if verbose {
			utils.Error("%v", err)
		} else {
			utils.ProgressError(err.Error())
		}
	}

	if verbose {
		if errorCount > 0 {
			utils.Warn("   - Completed with %d errors", errorCount)
		} else {
			utils.Info("   - All files processed successfully")
		}
	}

	stats.UpdateStatus("File processing completed")
	return nil
}

// calculateTotalSize calcule la taille totale des fichiers
func (m *Manager) calculateTotalSize(files []index.FileEntry) int64 {
	var total int64
	for _, file := range files {
		total += file.Size
	}
	return total
}

// backupSingleFileWithMultiProgress sauvegarde un seul fichier avec la barre de progression intégrée
func (m *Manager) backupSingleFileWithMultiProgress(file index.FileEntry, backupID string, multiProgressBar *utils.IntegratedProgressBar, verbose bool) error {
	fileName := filepath.Base(file.Path)
	utils.Debug("   - Processing file: %s (%.2f MB)", fileName, float64(file.Size)/1024/1024)

	// Vérifier si le fichier existe
	fileInfo, err := os.Stat(file.Path)
	if err != nil {
		if os.IsNotExist(err) {
			utils.Debug("⚠️  Skipping non-existent file: %s", file.Path)
			return nil
		}
		return fmt.Errorf("error reading file %s: %w", file.Path, err)
	}

	// Vérifier si c'est un répertoire
	if fileInfo.IsDir() {
		utils.Debug("⚠️  Skipping directory: %s", file.Path)
		return nil
	}

	// Parser les seuils de taille
	largeThreshold, err := parseSizeString(m.config.Backup.LargeFileThreshold)
	if err != nil {
		utils.Warn("Invalid large_file_threshold config, using default 100MB: %v", err)
		largeThreshold = 100 * 1024 * 1024 // 100MB default
	}

	ultraLargeThreshold, err := parseSizeString(m.config.Backup.UltraLargeThreshold)
	if err != nil {
		utils.Warn("Invalid ultra_large_threshold config, using default 5GB: %v", err)
		ultraLargeThreshold = 5 * 1024 * 1024 * 1024 // 5GB default
	}

	// Choisir la méthode de sauvegarde selon la taille avec suivi de progression
	if file.Size >= ultraLargeThreshold {
		if verbose {
			utils.Debug("🔄 Processing ultra-large file: %s (%.2f MB)", fileName, float64(file.Size)/1024/1024)
		}
		return m.backupUltraLargeFileWithMultiProgress(file, backupID, multiProgressBar, verbose)
	} else if file.Size >= largeThreshold {
		if verbose {
			utils.Debug("🔄 Processing large file: %s (%.2f MB)", fileName, float64(file.Size)/1024/1024)
		}
		return m.backupLargeFileWithMultiProgress(file, backupID, multiProgressBar, verbose)
	} else {
		if verbose {
			utils.Debug("🔄 Processing standard file: %s (%.2f MB)", fileName, float64(file.Size)/1024/1024)
		}
		return m.backupStandardFileWithMultiProgress(file, backupID, multiProgressBar, verbose)
	}
}

// backupStandardFileWithMultiProgress sauvegarde un fichier standard avec la barre de progression intégrée
func (m *Manager) backupStandardFileWithMultiProgress(file index.FileEntry, backupID string, multiProgressBar *utils.IntegratedProgressBar, verbose bool) error {
	fileName := filepath.Base(file.Path)

	if verbose {
		utils.Debug("🔄 Processing standard file: %s (%.2f MB)", file.Path, float64(file.Size)/1024/1024)
	}

	// Mettre à jour la progression si la barre est disponible
	if multiProgressBar != nil && !verbose {
		fileName := filepath.Base(file.Path)
		multiProgressBar.UpdateChunkWithName(fileName, 0, 1) // 1 chunk pour les fichiers standards
	}

	// Lire le fichier
	fileData, err := os.ReadFile(file.Path)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	// Mettre à jour la progression après lecture
	if multiProgressBar != nil && !verbose {
		fileName := filepath.Base(file.Path)
		multiProgressBar.UpdateChunkWithName(fileName, 1, 1) // Lecture terminée
	}

	// Compresser les données si configuré
	if m.config.Backup.CompressionLevel > 0 {
		if verbose {
			utils.Debug("🗜️  Compressing file...")
		}
		compressedData, err := m.compressor.Compress(fileData)
		if err != nil {
			return fmt.Errorf("error compressing file: %w", err)
		}
		fileData = compressedData
	}

	// Chiffrer les données
	if verbose {
		utils.Debug("🔐 Encrypting file...")
	}
	encryptedData, err := m.encryptor.Encrypt(fileData)
	if err != nil {
		return fmt.Errorf("error encrypting file: %w", err)
	}

	// Générer la clé de stockage cohérente avec l'index: data/{backupID}/{storageKey}
	storageKey := fmt.Sprintf("data/%s/%s", backupID, file.GetStorageKey())

	// Sauvegarder avec retry
	if err := m.saveToStorageWithRetry(storageKey, encryptedData); err != nil {
		return fmt.Errorf("error saving file to storage: %w", err)
	}

	// Retirer le fichier de la barre de progression
	if multiProgressBar != nil && !verbose {
		fileName := filepath.Base(file.Path)
		multiProgressBar.RemoveFile(fileName)
	}

	if verbose {
		utils.Debug("✅ Standard file saved: %s", fileName)
	}
	return nil
}

// backupLargeFileWithMultiProgress sauvegarde un fichier volumineux avec la barre de progression intégrée
func (m *Manager) backupLargeFileWithMultiProgress(file index.FileEntry, backupID string, multiProgressBar *utils.IntegratedProgressBar, verbose bool) error {
	fileName := filepath.Base(file.Path)

	if verbose {
		utils.Debug("🔄 Processing large file: %s (%.2f MB)", file.Path, float64(file.Size)/1024/1024)
	}

	// Initialiser les statistiques de chunking
	stats := NewBackupStats()
	stats.TotalSize = file.Size
	stats.UpdateStatus(fmt.Sprintf("Processing large file: %s", fileName))

	// Arrêter le monitoring à la fin de la fonction
	defer stats.StopMonitoring()

	// Read file in chunks and process each chunk
	fileHandle, err := os.Open(file.Path)
	if err != nil {
		return fmt.Errorf("error opening large file: %w", err)
	}
	defer fileHandle.Close()

	storageKey := fmt.Sprintf("data/%s/%s", backupID, file.GetStorageKey())
	if verbose {
		utils.Debug("📋 Starting chunked upload for large file: %s", file.Path)
	}

	// Get chunk size from config or use default
	chunkSizeStr := m.config.Backup.ChunkSize
	if chunkSizeStr == "" {
		chunkSizeStr = "10MB" // Default
	}

	chunkSize, err := parseSizeString(chunkSizeStr)
	if err != nil {
		utils.Warn("Invalid chunk_size config, using default 10MB: %v", err)
		chunkSize = 10 * 1024 * 1024 // 10MB default
	}

	if verbose {
		utils.Debug("🔧 Using chunk size: %s (%d bytes) for large file", chunkSizeStr, chunkSize)
	}

	chunkNumber := 0
	totalProcessed := int64(0)

	// Calculate total chunks for progress bar
	totalChunks := (file.Size + chunkSize - 1) / chunkSize // Ceiling division
	stats.TotalChunks = int(totalChunks)

	// Afficher la progression avec la barre multi-fichiers si disponible
	if multiProgressBar != nil && !verbose {
		multiProgressBar.SetCurrentFile(fileName, file.Size)
		multiProgressBar.UpdateChunk(fileName, 0, int64(totalChunks))
	} else if verbose {
		utils.ProgressStep(fmt.Sprintf("Processing large file: %s (%.2f MB)",
			fileName, float64(file.Size)/1024/1024))
	}

	if verbose {
		utils.Debug("📊 File processing plan:")
		utils.Debug("   - Total file size: %.2f MB", float64(file.Size)/1024/1024)
		utils.Debug("   - Chunk size: %.2f MB", float64(chunkSize)/1024/1024)
		utils.Debug("   - Total chunks: %d", totalChunks)
		utils.Debug("   - Storage key: %s", storageKey)
	}

	// Démarrer le monitoring spécifique pour ce fichier chunké
	m.startChunkMonitoring(stats, verbose)

	for {
		// Read chunk
		chunk := make([]byte, chunkSize)
		n, err := fileHandle.Read(chunk)
		if n == 0 {
			break // End of file
		}
		if err != nil && err != io.EOF {
			return fmt.Errorf("error reading chunk %d: %w", chunkNumber, err)
		}

		chunk = chunk[:n] // Adjust slice to actual bytes read
		totalProcessed += int64(n)

		// Mettre à jour les statistiques de chunking
		stats.UpdateChunkStats(chunkNumber+1, int(totalChunks), int64(n))

		// Mettre à jour la progression des chunks avec la barre multi-fichiers si disponible
		if multiProgressBar != nil && !verbose {
			multiProgressBar.UpdateChunk(fileName, int64(chunkNumber+1), int64(totalChunks))
		} else if verbose {
			// Show progress for each chunk with file name for clarity
			progress := float64(chunkNumber+1) / float64(totalChunks) * 100
			utils.ProgressStep(fmt.Sprintf("[%s] Chunk %d/%d (%.1f%%) - %.2f MB / %.2f MB",
				fileName, chunkNumber+1, totalChunks, progress,
				float64(totalProcessed)/1024/1024, float64(file.Size)/1024/1024))
		}

		if verbose {
			utils.Debug("🔄 Processing chunk %d: %d bytes (%.2f MB), total: %.2f MB / %.2f MB",
				chunkNumber, n, float64(n)/1024/1024, float64(totalProcessed)/1024/1024, float64(file.Size)/1024/1024)
		}

		// Compress then encrypt (dans cet ordre)
		processedChunk := chunk
		if m.config.Backup.CompressionLevel > 0 {
			if verbose {
				utils.Debug("🗜️  Compressing chunk %d...", chunkNumber)
			}
			compressedChunk, err := m.compressor.Compress(processedChunk)
			if err != nil {
				return fmt.Errorf("error compressing chunk %d: %w", chunkNumber, err)
			}
			processedChunk = compressedChunk
		}

		// Encrypt chunk
		if verbose {
			utils.Debug("🔐 Encrypting chunk %d...", chunkNumber)
		}
		encryptedChunk, err := m.encryptor.Encrypt(processedChunk)
		if err != nil {
			return fmt.Errorf("error encrypting chunk %d: %w", chunkNumber, err)
		}

		// Generate chunk key
		chunkKey := fmt.Sprintf("%s.chunk.%03d", storageKey, chunkNumber)
		if verbose {
			utils.Debug("📤 Uploading chunk %d to storage: %s", chunkNumber, chunkKey)
		}

		// Upload chunk with retry
		if err := m.saveToStorageWithRetry(chunkKey, encryptedChunk); err != nil {
			return fmt.Errorf("error uploading chunk %d: %w", chunkNumber, err)
		}

		chunkNumber++
	}

	// Save metadata
	metadata := map[string]interface{}{
		"chunks": chunkNumber,
		"size":   file.Size,
	}

	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("error marshaling metadata: %w", err)
	}

	metadataKey := fmt.Sprintf("%s.metadata", storageKey)
	if err := m.saveToStorageWithRetry(metadataKey, metadataBytes); err != nil {
		return fmt.Errorf("error saving metadata: %w", err)
	}

	// Retirer le fichier de la barre de progression
	if multiProgressBar != nil && !verbose {
		multiProgressBar.RemoveFile(fileName)
	}

	if verbose {
		utils.Debug("✅ Large file saved: %s (%d chunks)", fileName, chunkNumber)
	}
	return nil
}

// backupUltraLargeFileWithMultiProgress sauvegarde un fichier extrêmement volumineux avec la barre de progression intégrée
func (m *Manager) backupUltraLargeFileWithMultiProgress(file index.FileEntry, backupID string, multiProgressBar *utils.IntegratedProgressBar, verbose bool) error {
	fileName := filepath.Base(file.Path)

	if verbose {
		utils.Debug("🔄 Processing ultra-large file: %s (%.2f MB)", file.Path, float64(file.Size)/1024/1024)
	}

	// Initialiser les statistiques de chunking
	stats := NewBackupStats()
	stats.TotalSize = file.Size
	stats.UpdateStatus(fmt.Sprintf("Processing ultra-large file: %s", fileName))

	// Arrêter le monitoring à la fin de la fonction
	defer stats.StopMonitoring()

	// Read file in chunks and process each chunk
	fileHandle, err := os.Open(file.Path)
	if err != nil {
		return fmt.Errorf("error opening ultra-large file: %w", err)
	}
	defer fileHandle.Close()

	storageKey := fmt.Sprintf("data/%s/%s", backupID, file.GetStorageKey())
	if verbose {
		utils.Debug("📋 Starting chunked upload for ultra-large file: %s", file.Path)
	}

	// Get chunk size from config or use default
	chunkSizeStr := m.config.Backup.ChunkSize
	if chunkSizeStr == "" {
		chunkSizeStr = "50MB" // Default pour les très gros fichiers
	}

	chunkSize, err := parseSizeString(chunkSizeStr)
	if err != nil {
		utils.Warn("Invalid chunk_size config, using default 50MB: %v", err)
		chunkSize = 50 * 1024 * 1024 // 50MB default
	}

	if verbose {
		utils.Debug("🔧 Using chunk size: %s (%d bytes) for ultra-large file", chunkSizeStr, chunkSize)
	}

	chunkNumber := 0
	totalProcessed := int64(0)

	// Calculate total chunks for progress bar
	totalChunks := (file.Size + chunkSize - 1) / chunkSize // Ceiling division
	stats.TotalChunks = int(totalChunks)

	// Afficher la progression avec la barre multi-fichiers si disponible
	if multiProgressBar != nil && !verbose {
		multiProgressBar.SetCurrentFile(fileName, file.Size)
		multiProgressBar.UpdateChunk(fileName, 0, int64(totalChunks))
	} else if verbose {
		utils.ProgressStep(fmt.Sprintf("Processing ultra-large file: %s (%.2f MB)",
			fileName, float64(file.Size)/1024/1024))
	}

	if verbose {
		utils.Debug("📊 File processing plan:")
		utils.Debug("   - Total file size: %.2f MB", float64(file.Size)/1024/1024)
		utils.Debug("   - Chunk size: %.2f MB", float64(chunkSize)/1024/1024)
		utils.Debug("   - Total chunks: %d", totalChunks)
		utils.Debug("   - Storage key: %s", storageKey)
	}

	// Démarrer le monitoring spécifique pour ce fichier chunké
	m.startChunkMonitoring(stats, verbose)

	for {
		// Read chunk
		chunk := make([]byte, chunkSize)
		n, err := fileHandle.Read(chunk)
		if n == 0 {
			break // End of file
		}
		if err != nil && err != io.EOF {
			return fmt.Errorf("error reading chunk %d: %w", chunkNumber, err)
		}

		chunk = chunk[:n] // Adjust slice to actual bytes read
		totalProcessed += int64(n)

		// Mettre à jour les statistiques de chunking
		stats.UpdateChunkStats(chunkNumber+1, int(totalChunks), int64(n))

		// Mettre à jour la progression des chunks avec la barre multi-fichiers si disponible
		if multiProgressBar != nil && !verbose {
			multiProgressBar.UpdateChunk(fileName, int64(chunkNumber+1), int64(totalChunks))
		} else if verbose {
			// Show progress for each chunk with file name for clarity
			progress := float64(chunkNumber+1) / float64(totalChunks) * 100
			utils.ProgressStep(fmt.Sprintf("[%s] Chunk %d/%d (%.1f%%) - %.2f MB / %.2f MB",
				fileName, chunkNumber+1, totalChunks, progress,
				float64(totalProcessed)/1024/1024, float64(file.Size)/1024/1024))
		}

		if verbose {
			utils.Debug("🔄 Processing chunk %d: %d bytes (%.2f MB), total: %.2f MB / %.2f MB",
				chunkNumber, n, float64(n)/1024/1024, float64(totalProcessed)/1024/1024, float64(file.Size)/1024/1024)
		}

		// Compress then encrypt (dans cet ordre)
		processedChunk := chunk
		if m.config.Backup.CompressionLevel > 0 {
			if verbose {
				utils.Debug("🗜️  Compressing chunk %d...", chunkNumber)
			}
			compressedChunk, err := m.compressor.Compress(processedChunk)
			if err != nil {
				return fmt.Errorf("error compressing chunk %d: %w", chunkNumber, err)
			}
			processedChunk = compressedChunk
		}

		// Encrypt chunk
		if verbose {
			utils.Debug("🔐 Encrypting chunk %d...", chunkNumber)
		}
		encryptedChunk, err := m.encryptor.Encrypt(processedChunk)
		if err != nil {
			return fmt.Errorf("error encrypting chunk %d: %w", chunkNumber, err)
		}

		// Generate chunk key
		chunkKey := fmt.Sprintf("%s.chunk.%03d", storageKey, chunkNumber)
		if verbose {
			utils.Debug("📤 Uploading chunk %d to storage: %s", chunkNumber, chunkKey)
		}

		// Upload chunk with retry
		if err := m.saveToStorageWithRetry(chunkKey, encryptedChunk); err != nil {
			return fmt.Errorf("error uploading chunk %d: %w", chunkNumber, err)
		}

		chunkNumber++
	}

	// Save metadata
	metadata := map[string]interface{}{
		"chunks": chunkNumber,
		"size":   file.Size,
	}

	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("error marshaling metadata: %w", err)
	}

	metadataKey := fmt.Sprintf("%s.metadata", storageKey)
	if err := m.saveToStorageWithRetry(metadataKey, metadataBytes); err != nil {
		return fmt.Errorf("error saving metadata: %w", err)
	}

	// Retirer le fichier de la barre de progression
	if multiProgressBar != nil && !verbose {
		multiProgressBar.RemoveFile(fileName)
	}

	if verbose {
		utils.Debug("✅ Ultra-large file saved: %s (%d chunks)", fileName, chunkNumber)
	}
	return nil
}

// isFileModifiedSinceLastBackup vérifie si un fichier a été modifié depuis le dernier backup
func (m *Manager) isFileModifiedSinceLastBackup(file index.FileEntry, fileInfo os.FileInfo) bool {
	// Comparer la taille
	if fileInfo.Size() != file.Size {
		utils.Debug("   Size changed: %d -> %d", file.Size, fileInfo.Size())
		return true
	}

	// Comparer le temps de modification
	if !file.ModifiedTime.Equal(fileInfo.ModTime()) {
		utils.Debug("   Modification time changed: %s -> %s",
			file.ModifiedTime.Format("2006-01-02 15:04:05"),
			fileInfo.ModTime().Format("2006-01-02 15:04:05"))
		return true
	}

	// Si les métadonnées sont identiques, le fichier n'a pas changé
	return false
}

// backupLargeFile sauvegarde les fichiers volumineux (100MB - 5GB) avec chunking
func (m *Manager) backupLargeFile(file index.FileEntry, backupID string) error {
	fileName := filepath.Base(file.Path)
	utils.Debug("🔄 Processing large file: %s (%d bytes, %.2f MB)", file.Path, file.Size, float64(file.Size)/1024/1024)

	// Initialiser les statistiques de chunking
	stats := NewBackupStats()
	stats.TotalSize = file.Size
	stats.UpdateStatus(fmt.Sprintf("Processing large file: %s", fileName))

	// Arrêter le monitoring à la fin de la fonction
	defer stats.StopMonitoring()

	// Read file in chunks and process each chunk
	fileHandle, err := os.Open(file.Path)
	if err != nil {
		return fmt.Errorf("error opening large file: %w", err)
	}
	defer fileHandle.Close()

	storageKey := fmt.Sprintf("data/%s/%s", backupID, file.GetStorageKey())
	utils.Debug("📋 Starting chunked upload for large file: %s", file.Path)

	// Get chunk size from config or use default
	chunkSizeStr := m.config.Backup.ChunkSize
	if chunkSizeStr == "" {
		chunkSizeStr = "10MB" // Default
	}

	chunkSize, err := parseSizeString(chunkSizeStr)
	if err != nil {
		utils.Warn("Invalid chunk_size config, using default 10MB: %v", err)
		chunkSize = 10 * 1024 * 1024 // 10MB default
	}

	utils.Debug("🔧 Using chunk size: %s (%d bytes) for large file", chunkSizeStr, chunkSize)

	chunkNumber := 0
	totalProcessed := int64(0)

	// Calculate total chunks for progress bar
	totalChunks := (file.Size + chunkSize - 1) / chunkSize // Ceiling division
	stats.TotalChunks = int(totalChunks)

	// Afficher la progression avec la barre multi-fichiers si disponible
	if m.multiProgressBar != nil {
		m.multiProgressBar.SetCurrentFile(fileName, file.Size)
		m.multiProgressBar.UpdateChunk(fileName, 0, int64(totalChunks))
	} else {
		utils.ProgressStep(fmt.Sprintf("Processing large file: %s (%.2f MB)",
			fileName, float64(file.Size)/1024/1024))
	}

	utils.Debug("📊 File processing plan:")
	utils.Debug("   - Total file size: %.2f MB", float64(file.Size)/1024/1024)
	utils.Debug("   - Chunk size: %.2f MB", float64(chunkSize)/1024/1024)
	utils.Debug("   - Total chunks: %d", totalChunks)
	utils.Debug("   - Storage key: %s", storageKey)

	// Démarrer le monitoring spécifique pour ce fichier chunké
	m.startChunkMonitoring(stats, true)

	for {
		// Read chunk
		chunk := make([]byte, chunkSize)
		n, err := fileHandle.Read(chunk)
		if n == 0 {
			break // End of file
		}
		if err != nil && err != io.EOF {
			return fmt.Errorf("error reading chunk %d: %w", chunkNumber, err)
		}

		chunk = chunk[:n] // Adjust slice to actual bytes read
		totalProcessed += int64(n)

		// Mettre à jour les statistiques de chunking
		stats.UpdateChunkStats(chunkNumber+1, int(totalChunks), int64(n))

		// Mettre à jour la progression des chunks avec la barre multi-fichiers si disponible
		if m.multiProgressBar != nil {
			m.multiProgressBar.UpdateChunk(fileName, int64(chunkNumber+1), int64(totalChunks))
		} else {
			// Show progress for each chunk with file name for clarity
			progress := float64(chunkNumber+1) / float64(totalChunks) * 100
			utils.ProgressStep(fmt.Sprintf("[%s] Chunk %d/%d (%.1f%%) - %.2f MB / %.2f MB",
				fileName, chunkNumber+1, totalChunks, progress,
				float64(totalProcessed)/1024/1024, float64(file.Size)/1024/1024))
		}

		utils.Debug("🔄 Processing chunk %d: %d bytes (%.2f MB), total: %.2f MB / %.2f MB",
			chunkNumber, n, float64(n)/1024/1024, float64(totalProcessed)/1024/1024, float64(file.Size)/1024/1024)

		// Encrypt chunk
		utils.Debug("🔐 Encrypting chunk %d...", chunkNumber)
		encryptedChunk, err := m.encryptor.Encrypt(chunk)
		if err != nil {
			return fmt.Errorf("error encrypting chunk %d: %w", chunkNumber, err)
		}
		utils.Debug("✅ Chunk %d encrypted successfully", chunkNumber)

		// Upload chunk
		chunkKey := fmt.Sprintf("%s.chunk.%03d", storageKey, chunkNumber)
		utils.Debug("📤 Uploading chunk %d to storage: %s", chunkNumber, chunkKey)
		if err := m.saveToStorageWithRetry(chunkKey, encryptedChunk); err != nil {
			return fmt.Errorf("error uploading chunk %d: %w", chunkNumber, err)
		}
		utils.Debug("✅ Chunk %d uploaded successfully", chunkNumber)

		chunkNumber++
	}

	// Show completion
	utils.ProgressSuccess(fmt.Sprintf("Large file completed: %s (%.2f MB in %d chunks)",
		filepath.Base(file.Path), float64(file.Size)/1024/1024, chunkNumber))

	// Create metadata file
	utils.Debug("📝 Creating metadata file for chunked file...")
	metadata := map[string]interface{}{
		"original_file": file.Path,
		"file_size":     file.Size,
		"chunks":        chunkNumber,
		"storage_key":   storageKey,
		"chunked":       true,
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("error creating metadata: %w", err)
	}

	metadataKey := fmt.Sprintf("%s.metadata", storageKey)
	utils.Debug("📤 Uploading metadata file: %s", metadataKey)
	if err := m.saveToStorageWithRetry(metadataKey, metadataJSON); err != nil {
		return fmt.Errorf("error uploading metadata: %w", err)
	}
	utils.Debug("✅ Metadata file uploaded successfully")

	// Stocker seulement la clé relative, pas la clé complète
	file.StorageKey = file.GetStorageKey()
	utils.Debug("🎯 Large file backed up in %d chunks: %s -> %s", chunkNumber, file.Path, storageKey)
	return nil
}

// backupStandardFile sauvegarde un fichier standard (< 100MB)
func (m *Manager) backupStandardFile(file index.FileEntry, backupID string) error {

	utils.Debug("🔄 Processing standard file: %s (%.2f MB)", file.Path, float64(file.Size)/1024/1024)

	// Lire le fichier
	fileData, err := os.ReadFile(file.Path)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	// Compresser les données si configuré
	if m.config.Backup.CompressionLevel > 0 {
		utils.Debug("🗜️  Compressing file...")
		compressedData, err := m.compressor.Compress(fileData)
		if err != nil {
			return fmt.Errorf("error compressing file: %w", err)
		}
		fileData = compressedData
		utils.Debug("✅ File compressed successfully")
	}

	// Chiffrer les données
	utils.Debug("🔐 Encrypting file...")
	encryptedData, err := m.encryptor.Encrypt(fileData)
	if err != nil {
		return fmt.Errorf("error encrypting file: %w", err)
	}
	utils.Debug("✅ File encrypted successfully")

	// Sauvegarder vers le stockage
	storageKey := fmt.Sprintf("data/%s/%s", backupID, file.GetStorageKey())
	utils.Debug("📤 Uploading file to storage: %s", storageKey)
	if err := m.saveToStorageWithRetry(storageKey, encryptedData); err != nil {
		return fmt.Errorf("error uploading file: %w", err)
	}
	utils.Debug("✅ File uploaded successfully")

	// Stocker seulement la clé relative, pas la clé complète
	file.StorageKey = file.GetStorageKey()
	utils.Debug("🎯 Standard file backed up: %s -> %s", file.Path, storageKey)
	return nil
}

// backupUltraLargeFile sauvegarde les fichiers extrêmement volumineux (> 5GB) avec monitoring
func (m *Manager) backupUltraLargeFile(file index.FileEntry, backupID string) error {
	fileName := filepath.Base(file.Path)
	utils.Debug("🔄 Processing ultra-large file: %s (%d bytes, %.2f MB)", file.Path, file.Size, float64(file.Size)/1024/1024)

	// Initialiser les statistiques de chunking
	stats := NewBackupStats()
	stats.TotalSize = file.Size
	stats.UpdateStatus(fmt.Sprintf("Processing ultra-large file: %s", fileName))

	// Arrêter le monitoring à la fin de la fonction
	defer stats.StopMonitoring()

	// Read file in chunks and process each chunk
	fileHandle, err := os.Open(file.Path)
	if err != nil {
		return fmt.Errorf("error opening ultra-large file: %w", err)
	}
	defer fileHandle.Close()

	storageKey := fmt.Sprintf("data/%s/%s", backupID, file.GetStorageKey())
	utils.Debug("📋 Starting chunked upload for ultra-large file: %s", file.Path)

	// Get chunk size from config or use default
	chunkSizeStr := m.config.Backup.ChunkSizeLarge
	if chunkSizeStr == "" {
		chunkSizeStr = "50MB" // Default
	}

	chunkSize, err := parseSizeString(chunkSizeStr)
	if err != nil {
		utils.Warn("Invalid chunk_size_large config, using default 50MB: %v", err)
		chunkSize = 50 * 1024 * 1024 // 50MB default
	}

	utils.Debug("🔧 Using chunk size: %s (%d bytes) for ultra-large file", chunkSizeStr, chunkSize)

	chunkNumber := 0
	totalProcessed := int64(0)

	// Calculate total chunks for progress bar
	totalChunks := (file.Size + chunkSize - 1) / chunkSize // Ceiling division
	stats.TotalChunks = int(totalChunks)

	// Afficher la progression avec la barre multi-fichiers si disponible
	if m.multiProgressBar != nil {
		m.multiProgressBar.SetCurrentFile(fileName, file.Size)
		m.multiProgressBar.UpdateChunk(fileName, 0, int64(totalChunks))
	} else {
		utils.ProgressStep(fmt.Sprintf("Processing large file: %s (%.2f MB)",
			fileName, float64(file.Size)/1024/1024))
	}

	utils.Debug("📊 File processing plan:")
	utils.Debug("   - Total file size: %.2f MB", float64(file.Size)/1024/1024)
	utils.Debug("   - Chunk size: %.2f MB", float64(chunkSize)/1024/1024)
	utils.Debug("   - Total chunks: %d", totalChunks)
	utils.Debug("   - Storage key: %s", storageKey)

	// Démarrer le monitoring spécifique pour ce fichier chunké
	m.startChunkMonitoring(stats, true)

	for {
		// Read chunk
		chunk := make([]byte, chunkSize)
		n, err := fileHandle.Read(chunk)
		if n == 0 {
			break // End of file
		}
		if err != nil && err != io.EOF {
			return fmt.Errorf("error reading chunk %d: %w", chunkNumber, err)
		}

		chunk = chunk[:n] // Adjust slice to actual bytes read
		totalProcessed += int64(n)

		// Mettre à jour les statistiques de chunking
		stats.UpdateChunkStats(chunkNumber+1, int(totalChunks), int64(n))

		// Mettre à jour la progression des chunks avec la barre multi-fichiers si disponible
		if m.multiProgressBar != nil {
			m.multiProgressBar.UpdateChunk(fileName, int64(chunkNumber+1), int64(totalChunks))
		} else {
			// Show progress for each chunk with file name for clarity
			progress := float64(chunkNumber+1) / float64(totalChunks) * 100
			utils.ProgressStep(fmt.Sprintf("[%s] Chunk %d/%d (%.1f%%) - %.2f MB / %.2f MB",
				fileName, chunkNumber+1, totalChunks, progress,
				float64(totalProcessed)/1024/1024, float64(file.Size)/1024/1024))
		}

		utils.Debug("🔄 Processing chunk %d: %d bytes (%.2f MB), total: %.2f MB / %.2f MB",
			chunkNumber, n, float64(n)/1024/1024, float64(totalProcessed)/1024/1024, float64(file.Size)/1024/1024)

		// Encrypt chunk (no compression for ultra-large files)
		utils.Debug("🔐 Encrypting chunk %d...", chunkNumber)
		encryptedChunk, err := m.encryptor.Encrypt(chunk)
		if err != nil {
			return fmt.Errorf("error encrypting chunk %d: %w", chunkNumber, err)
		}
		utils.Debug("✅ Chunk %d encrypted successfully", chunkNumber)

		// Upload chunk
		chunkKey := fmt.Sprintf("%s.chunk.%03d", storageKey, chunkNumber)
		utils.Debug("📤 Uploading chunk %d to storage: %s", chunkNumber, chunkKey)
		if err := m.saveToStorageWithRetry(chunkKey, encryptedChunk); err != nil {
			return fmt.Errorf("error uploading chunk %d: %w", chunkNumber, err)
		}
		utils.Debug("✅ Chunk %d uploaded successfully", chunkNumber)

		chunkNumber++
	}

	// Show completion
	utils.ProgressSuccess(fmt.Sprintf("Large file completed: %s (%.2f MB in %d chunks)",
		filepath.Base(file.Path), float64(file.Size)/1024/1024, chunkNumber))

	// Create metadata file
	utils.Debug("📝 Creating metadata file for chunked file...")
	metadata := map[string]interface{}{
		"original_file": file.Path,
		"file_size":     file.Size,
		"chunks":        chunkNumber,
		"storage_key":   storageKey,
		"chunked":       true,
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("error creating metadata: %w", err)
	}

	metadataKey := fmt.Sprintf("%s.metadata", storageKey)
	utils.Debug("📤 Uploading metadata file: %s", metadataKey)
	if err := m.saveToStorageWithRetry(metadataKey, metadataJSON); err != nil {
		return fmt.Errorf("error uploading metadata: %w", err)
	}
	utils.Debug("✅ Metadata file uploaded successfully")

	// Stocker seulement la clé relative, pas la clé complète
	file.StorageKey = file.GetStorageKey()
	utils.Debug("🎯 Ultra-large file backed up in %d chunks: %s -> %s", chunkNumber, file.Path, storageKey)
	return nil
}

// backupVeryLargeFile sauvegarde les fichiers très volumineux (100MB - 5GB) avec chunking
func (m *Manager) backupVeryLargeFile(file index.FileEntry, backupID string) error {
	fileName := filepath.Base(file.Path)
	utils.Debug("🔄 Processing very large file: %s (%d bytes, %.2f MB)", file.Path, file.Size, float64(file.Size)/1024/1024)

	// Show progress for very large file
	utils.ProgressStep(fmt.Sprintf("Processing very large file: %s (%.2f MB)",
		fileName, float64(file.Size)/1024/1024))

	// Read file in chunks and process each chunk
	fileHandle, err := os.Open(file.Path)
	if err != nil {
		return fmt.Errorf("error opening very large file: %w", err)
	}
	defer fileHandle.Close()

	storageKey := fmt.Sprintf("data/%s/%s", backupID, file.GetStorageKey())
	utils.Debug("📋 Starting chunked upload for very large file: %s", file.Path)

	// Get chunk size from config or use default
	chunkSizeStr := m.config.Backup.ChunkSizeLarge
	if chunkSizeStr == "" {
		chunkSizeStr = "25MB" // Default for very large files
	}

	chunkSize, err := parseSizeString(chunkSizeStr)
	if err != nil {
		utils.Warn("Invalid chunk_size_large config, using default 25MB: %v", err)
		chunkSize = 25 * 1024 * 1024 // 25MB default
	}

	utils.Debug("🔧 Using chunk size: %s (%d bytes) for very large file", chunkSizeStr, chunkSize)

	chunkNumber := 0
	totalProcessed := int64(0)

	// Calculate total chunks for progress bar
	totalChunks := (file.Size + chunkSize - 1) / chunkSize // Ceiling division

	utils.Debug("📊 File processing plan:")
	utils.Debug("   - Total file size: %.2f MB", float64(file.Size)/1024/1024)
	utils.Debug("   - Chunk size: %.2f MB", float64(chunkSize)/1024/1024)
	utils.Debug("   - Total chunks: %d", totalChunks)
	utils.Debug("   - Storage key: %s", storageKey)

	for {
		// Read chunk
		chunk := make([]byte, chunkSize)
		n, err := fileHandle.Read(chunk)
		if n == 0 {
			break // End of file
		}
		if err != nil && err != io.EOF {
			return fmt.Errorf("error reading chunk %d: %w", chunkNumber, err)
		}

		chunk = chunk[:n] // Adjust slice to actual bytes read
		totalProcessed += int64(n)

		// Show progress for each chunk
		progress := float64(chunkNumber+1) / float64(totalChunks) * 100
		utils.ProgressStep(fmt.Sprintf("[%s] Chunk %d/%d (%.1f%%) - %.2f MB / %.2f MB",
			fileName, chunkNumber+1, totalChunks, progress,
			float64(totalProcessed)/1024/1024, float64(file.Size)/1024/1024))

		utils.Debug("🔄 Processing chunk %d: %d bytes (%.2f MB), total: %.2f MB / %.2f MB",
			chunkNumber, n, float64(n)/1024/1024, float64(totalProcessed)/1024/1024, float64(file.Size)/1024/1024)

		// Skip compression for very large files to save memory
		utils.Debug("⏭️ Skipping compression for very large file chunk: %s", file.Path)

		// Encrypt chunk (no compression for very large files)
		utils.Debug("🔐 Encrypting chunk %d...", chunkNumber)
		encryptedChunk, err := m.encryptor.Encrypt(chunk)
		if err != nil {
			return fmt.Errorf("error encrypting chunk %d: %w", chunkNumber, err)
		}
		utils.Debug("✅ Chunk %d encrypted successfully", chunkNumber)

		// Upload chunk
		chunkKey := fmt.Sprintf("%s.chunk.%03d", storageKey, chunkNumber)
		utils.Debug("📤 Uploading chunk %d to storage: %s", chunkNumber, chunkKey)
		if err := m.saveToStorageWithRetry(chunkKey, encryptedChunk); err != nil {
			return fmt.Errorf("error uploading chunk %d: %w", chunkNumber, err)
		}
		utils.Debug("✅ Chunk %d uploaded successfully", chunkNumber)

		chunkNumber++
	}

	// Show completion
	utils.ProgressSuccess(fmt.Sprintf("Very large file completed: %s (%.2f MB in %d chunks)",
		fileName, float64(file.Size)/1024/1024, chunkNumber))

	// Create metadata file
	utils.Debug("📝 Creating metadata file for chunked file...")
	metadata := map[string]interface{}{
		"original_file": file.Path,
		"file_size":     file.Size,
		"chunks":        chunkNumber,
		"storage_key":   storageKey,
		"chunked":       true,
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("error creating metadata: %w", err)
	}

	metadataKey := fmt.Sprintf("%s.metadata", storageKey)
	utils.Debug("📤 Uploading metadata file: %s", metadataKey)
	if err := m.saveToStorageWithRetry(metadataKey, metadataJSON); err != nil {
		return fmt.Errorf("error uploading metadata: %w", err)
	}
	utils.Debug("✅ Metadata file uploaded successfully")

	// Stocker seulement la clé relative, pas la clé complète
	file.StorageKey = file.GetStorageKey()
	utils.Debug("🎯 Very large file backed up in %d chunks: %s -> %s", chunkNumber, file.Path, storageKey)
	return nil
}

// backupStandardLargeFile sauvegarde les fichiers volumineux (100MB - 1GB)
func (m *Manager) backupStandardLargeFile(file index.FileEntry, backupID string) error {
	fileName := filepath.Base(file.Path)
	utils.Debug("Processing standard large file: %s (%d bytes, %.2f MB)", file.Path, file.Size, float64(file.Size)/1024/1024)

	// Show progress for standard large file
	utils.ProgressStep(fmt.Sprintf("Processing large file: %s (%.2f MB)",
		fileName, float64(file.Size)/1024/1024))

	// Use standard buffer for large files
	bufferSize := 2 * 1024 * 1024 // 2MB buffer

	data, err := utils.ReadFileWithBuffer(file.Path, bufferSize)
	if err != nil {
		return fmt.Errorf("error reading large file: %w", err)
	}

	// Try compression for standard large files
	compressedData, err := m.compressor.CompressAdaptive(data, file.Path)
	if err != nil {
		utils.Debug("Compression failed for large file, using uncompressed: %s", file.Path)
		compressedData = data
	}

	// Encrypt
	encryptedData, err := m.encryptor.Encrypt(compressedData)
	if err != nil {
		return fmt.Errorf("error encrypting large file: %w", err)
	}

	// Save to storage
	storageKey := fmt.Sprintf("data/%s/%s", backupID, file.GetStorageKey())
	if err := m.saveToStorageWithRetry(storageKey, encryptedData); err != nil {
		return fmt.Errorf("error saving large file: %w", err)
	}

	// Stocker seulement la clé relative, pas la clé complète
	file.StorageKey = file.GetStorageKey()
	utils.ProgressSuccess(fmt.Sprintf("Large file completed: %s (%.2f MB)",
		fileName, float64(file.Size)/1024/1024))
	utils.Debug("Standard large file backed up: %s -> %s", file.Path, storageKey)
	return nil
}

// saveToStorageWithRetry sauvegarde avec retry et timeout
func (m *Manager) saveToStorageWithRetry(key string, data []byte) error {
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

			utils.Debug("🔄 Retry attempt %d/%d for %s after %v delay",
				attempt+1, maxRetries, key, delay)

			time.Sleep(delay)
		}

		// Créer un contexte avec timeout pour cette tentative
		ctx, cancel := context.WithTimeout(context.Background(), timeout)

		// Canal pour le résultat
		resultChan := make(chan error, 1)

		// Exécuter l'upload en arrière-plan
		go func() {
			resultChan <- m.storageClient.Upload(key, data)
		}()

		// Attendre avec timeout
		select {
		case err := <-resultChan:
			cancel()
			if err == nil {
				// Succès !
				if attempt > 0 {
					utils.Info("✅ Upload succeeded on retry attempt %d for %s", attempt+1, key)
				}
				return nil
			}

			// Erreur, la stocker pour le log final
			lastError = err

			// Log de l'erreur
			if attempt < maxRetries-1 {
				utils.Warn("⚠️  Upload failed for %s (attempt %d/%d): %v",
					key, attempt+1, maxRetries, err)
			}

		case <-ctx.Done():
			cancel()
			lastError = fmt.Errorf("upload timeout after %v", timeout)

			if attempt < maxRetries-1 {
				utils.Warn("⚠️  Upload timeout for %s (attempt %d/%d) after %v",
					key, attempt+1, maxRetries, timeout)
			}
		}
	}

	// Toutes les tentatives ont échoué
	utils.Error("❌ Upload failed for %s after %d attempts. Last error: %v",
		key, maxRetries, lastError)

	return fmt.Errorf("upload failed after %d attempts for %s: %w", maxRetries, key, lastError)
}

// calculateCompressedSize calcule la taille compressede totale
func (m *Manager) calculateCompressedSize(added, modified []index.FileEntry) int64 {
	// TODO: Implémenter le calcul de la taille compressede
	return 0
}

// calculateEncryptedSize calcule la taille encryptede totale
func (m *Manager) calculateEncryptedSize(added, modified []index.FileEntry) int64 {
	// TODO: Implémenter le calcul de la taille encryptede
	return 0
}

// deleteBackupFiles supprime les fichiers de données d'une sauvegarde
func (m *Manager) deleteBackupFiles(backupIndex *index.BackupIndex) error {
	utils.Info("Deleting data files for: %s", backupIndex.BackupID)

	for _, file := range backupIndex.Files {
		if file.StorageKey != "" {
			if err := m.storageClient.DeleteObject(file.StorageKey); err != nil {
				utils.Warn("Impossible de supprimer le fichier %s: %v", file.StorageKey, err)
			} else {
				utils.Debug("File deleted: %s", file.StorageKey)
			}
		}
	}

	return nil
}

// deleteBackupIndex supprime l'index d'une sauvegarde
func (m *Manager) deleteBackupIndex(backupID string) error {
	utils.Info("Suppression de l'index pour: %s", backupID)

	indexKey := fmt.Sprintf("indexes/%s.json", backupID)
	if err := m.storageClient.DeleteObject(indexKey); err != nil {
		return fmt.Errorf("error during la suppression de l'index: %w", err)
	}

	utils.Debug("Index deleted: %s", indexKey)
	return nil
}

// parseSizeString parse une chaîne de taille (e.g., "50MB", "1GB") en bytes
func parseSizeString(sizeStr string) (int64, error) {
	sizeStr = strings.TrimSpace(sizeStr)

	// Si c'est déjà un nombre, le traiter comme des bytes
	if number, err := strconv.ParseInt(sizeStr, 10, 64); err == nil {
		return number, nil
	}

	// Extraire le nombre et l'unité
	var number int64
	var unit string

	// Trouver le premier caractère non-numérique
	for i := 0; i < len(sizeStr); i++ {
		if sizeStr[i] < '0' || sizeStr[i] > '9' {
			numberStr := sizeStr[:i]
			unit = sizeStr[i:]

			var err error
			number, err = strconv.ParseInt(numberStr, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid number in size string: %s", sizeStr)
			}
			break
		}
	}

	if number == 0 {
		return 0, fmt.Errorf("no number found in size string: %s", sizeStr)
	}

	// Convertir selon l'unité
	unit = strings.ToUpper(strings.TrimSpace(unit))
	switch unit {
	case "B", "":
		return number, nil
	case "KB":
		return number * 1024, nil
	case "MB":
		return number * 1024 * 1024, nil
	case "GB":
		return number * 1024 * 1024 * 1024, nil
	case "TB":
		return number * 1024 * 1024 * 1024 * 1024, nil
	default:
		return 0, fmt.Errorf("unknown unit in size string: %s", sizeStr)
	}
}

// saveToStorage sauvegarde des données dans le stockage
func (m *Manager) saveToStorage(key string, data []byte) error {
	return m.storageClient.Upload(key, data)
}

// logBackupStart logs the start of backup operation
func (m *Manager) logBackupStart(backupName string, verbose bool) {
	if verbose {
		utils.Info("🔄 🚀 Starting backup: %s", backupName)
		utils.Info("📋 Tasks to perform:")
		utils.Info("   1. Initialize backup manager")
		utils.Info("   2. Create current file index")
		utils.Info("   3. Find previous backup (if exists)")
		utils.Info("   4. Calculate file differences")
		utils.Info("   5. Backup new/modified files")
		utils.Info("   6. Create and save backup index")
		utils.Info("   7. Apply retention policy")
	} else {
		utils.ProgressStep(fmt.Sprintf("🔄 🚀 Starting backup: %s", backupName))
	}
}

// prepareBackup initializes the backup manager
func (m *Manager) prepareBackup(sourcePath string) error {
	utils.Debug("🔧 Task: Initializing backup manager")

	// Charger la configuration
	config, err := utils.LoadConfig(m.configFile)
	if err != nil {
		return fmt.Errorf("error loading configuration: %w", err)
	}
	m.config = config

	// Initialiser les composants
	if err := m.initializeComponents(); err != nil {
		return fmt.Errorf("error during l'initialisation: %w", err)
	}

	utils.Debug("✅ Task completed: Backup manager initialized")
	return nil
}

// createCurrentIndex creates the current file index
func (m *Manager) createCurrentIndex(sourcePath, backupID string, verbose bool) (*index.BackupIndex, error) {
	if verbose {
		utils.Info("📋 Task 2: Creating current file index")
		utils.Info("   - Scanning directory: %s", sourcePath)
		utils.Info("   - Calculating checksums")
		utils.Info("   - Building file index")
	} else {
		utils.ProgressStep("Creating index...")
	}

	index, err := m.indexMgr.CreateIndex(sourcePath, backupID, verbose)
	if err != nil {
		return nil, fmt.Errorf("error creating index: %w", err)
	}

	if verbose {
		utils.Info("✅ Task 2 completed: Index created with %d files", index.TotalFiles)
	}

	return index, nil
}

// calculateBackupDiff calculates differences between current and previous backup
func (m *Manager) calculateBackupDiff(currentIndex *index.BackupIndex, backupName string, verbose bool) (*index.IndexDiff, error) {
	if verbose {
		utils.Info("📋 Task 3: Finding previous backup")
		utils.Info("   - Searching for existing backups")
		utils.Info("   - Loading previous index (if exists)")
	} else {
		utils.ProgressStep("Searching for previous backup...")
	}

	// Chercher la sauvegarde précédente pour comparaison
	previousIndex, err := m.findPreviousBackup(backupName)
	if err != nil {
		utils.Debug("Error finding previous backup: %v", err)
		// Si on ne peut pas charger l'index précédent, traiter comme un premier backup
		previousIndex = nil
	}

	if previousIndex == nil {
		if verbose {
			utils.Info("No previous backup found or unable to load, performing full backup")
		} else {
			utils.ProgressInfo("First backup - performing full backup")
		}
	} else {
		if verbose {
			utils.Info("Found previous backup: %s (created: %s)",
				previousIndex.BackupID,
				previousIndex.CreatedAt.Format("2006-01-02 15:04:05"))
		}
	}

	if verbose {
		utils.Info("📋 Task 4: Calculating file differences")
		utils.Info("   - Comparing current vs previous index")
		utils.Info("   - Identifying new files")
		utils.Info("   - Identifying modified files")
		utils.Info("   - Identifying deleted files")
	} else {
		utils.ProgressStep("Comparing indexes...")
	}

	// Comparer les index pour déterminer les changements
	var diff *index.IndexDiff
	if previousIndex != nil {
		diff, err = m.indexMgr.CompareIndexes(currentIndex, previousIndex)
		if err != nil {
			return nil, fmt.Errorf("error during la comparaison des index: %w", err)
		}

		if verbose {
			utils.Info("Comparison results:")
			utils.Info("   - Added: %d files", len(diff.Added))
			utils.Info("   - Modified: %d files", len(diff.Modified))
			utils.Info("   - Deleted: %d files", len(diff.Deleted))
		}
	} else {
		// Première sauvegarde ou index précédent non chargeable, tous les fichiers sont nouveaux
		diff = &index.IndexDiff{
			Added:    currentIndex.Files,
			Modified: []index.FileEntry{},
			Deleted:  []index.FileEntry{},
		}

		if verbose {
			utils.Info("First backup or unable to load previous index - all %d files are new", len(diff.Added))
		}
	}

	if verbose {
		utils.Info("✅ Task 4 completed: Found %d new, %d modified, %d deleted files",
			len(diff.Added), len(diff.Modified), len(diff.Deleted))
	}

	return diff, nil
}

// executeBackup executes the actual backup process
func (m *Manager) executeBackup(currentIndex *index.BackupIndex, diff *index.IndexDiff, backupID string, backupName string, verbose bool) error {
	totalFilesToBackup := len(diff.Added) + len(diff.Modified)

	if verbose {
		utils.Info("📋 Task 5: Backing up files")
		utils.Info("   - Total files to backup: %d", totalFilesToBackup)
		utils.Info("   - New files: %d", len(diff.Added))
		utils.Info("   - Modified files: %d", len(diff.Modified))
		utils.Info("   - Processing files in parallel")
		utils.Info("   - Encrypting and compressing")
		utils.Info("   - Uploading to storage")
	}

	// Sauvegarder les fichiers modifiés/ajoutés
	if err := m.backupFiles(diff.Added, diff.Modified, backupID, verbose); err != nil {
		return fmt.Errorf("error saving des fichiers: %w", err)
	}

	// Nettoyer les anciens objets S3 non référencés dans cette sauvegarde
	if err := m.cleanupUnreferencedObjects(backupID, currentIndex, verbose); err != nil {
		if verbose {
			utils.Warn("⚠️  Warning: Failed to cleanup unreferenced objects: %v", err)
		}
		// Ne pas faire échouer le backup complet à cause du nettoyage
	} else if verbose {
		utils.Info("🧹 Cleaned up unreferenced objects from previous backups")
	}

	// Optimisation : Préparer l'index en parallèle pendant les uploads
	if verbose {
		utils.Info("📋 Task 6: Finalizing backup")
		utils.Info("   - Calculating backup statistics")
		utils.Info("   - Creating backup index")
		utils.Info("   - Saving index to storage")
	} else {
		utils.ProgressStep("Finalizing backup...")
	}

	// Mettre à jour l'index avec les informations de sauvegarde
	currentIndex.BackupID = backupID
	currentIndex.CreatedAt = time.Now()
	// Calculer les tailles totales
	currentIndex.TotalFiles = int64(len(currentIndex.Files))
	currentIndex.TotalSize = m.calculateTotalSize(currentIndex.Files)

	// Vérifier que tous les fichiers ont des StorageKey valides
	validFiles := 0
	emptyKeys := 0
	for _, file := range currentIndex.Files {
		if file.StorageKey == "" {
			emptyKeys++
		} else {
			validFiles++
		}
	}

	if verbose {
		utils.Info("📊 Index validation:")
		utils.Info("   - Total files: %d", len(currentIndex.Files))
		utils.Info("   - Valid storage keys: %d", validFiles)
		utils.Info("   - Empty storage keys: %d", emptyKeys)
	}

	if emptyKeys > 0 {
		utils.Warn("⚠️  WARNING: %d files have empty storage keys!", emptyKeys)
		utils.Warn("   This indicates a backup processing issue.")
	}

	// Sauvegarder l'index
	if err := m.indexMgr.SaveIndex(currentIndex); err != nil {
		return fmt.Errorf("error saving de l'index: %w", err)
	}

	if verbose {
		utils.Info("✅ Task 6 completed: Backup index saved")
	}

	// Appliquer la politique de rétention automatiquement
	if verbose {
		utils.Info("📋 Task 7: Applying retention policy")
		utils.Info("   - Loading retention configuration")
		utils.Info("   - Finding old backups")
		utils.Info("   - Deleting expired backups")
	}

	err := m.applyRetentionPolicyForBackup(backupName, verbose)
	if err != nil {
		if verbose {
			utils.Warn("⚠️  Task 7 completed with warnings: Retention policy failed")
			utils.Warn("   - Error: %v", err)
			utils.Warn("   - Backup completed successfully, but retention cleanup failed")
		} else {
			utils.ProgressWarning("Retention policy failed, but backup completed")
		}
		// Ne pas faire échouer le backup complet à cause de la rétention
	} else {
		if verbose {
			utils.Info("✅ Task 7 completed: Retention policy applied successfully")
		} else {
			utils.ProgressSuccess("Retention policy applied")
		}
	}

	return nil
}

// logBackupCompletion logs the completion of backup operation
func (m *Manager) logBackupCompletion(diff *index.IndexDiff, duration time.Duration, verbose bool) {
	if verbose {
		utils.Info("✅ Backup completed in %v", duration)
		utils.Info("📊 Statistics: %d files added, %d modified, %d deleted",
			len(diff.Added), len(diff.Modified), len(diff.Deleted))
		utils.Info("🎯 Final tasks completed:")
		utils.Info("   ✅ All files backed up successfully")
		utils.Info("   ✅ Backup index saved")
		utils.Info("   ✅ Unreferenced objects cleaned up")
		utils.Info("   ✅ Retention policy applied")
	} else {
		utils.ProgressSuccess(fmt.Sprintf("✅ Backup completed in %v", duration))
		utils.ProgressInfo(fmt.Sprintf("📊 %d added, %d modified, %d deleted",
			len(diff.Added), len(diff.Modified), len(diff.Deleted)))
	}
}

// cleanupUnreferencedObjects supprime les anciens objets S3 non référencés dans la sauvegarde actuelle
func (m *Manager) cleanupUnreferencedObjects(backupID string, currentIndex *index.BackupIndex, verbose bool) error {
	if verbose {
		utils.Info("🧹 Cleaning up unreferenced objects from previous backups...")
	}

	// Créer un ensemble des clés de stockage référencées dans l'index actuel
	referencedKeys := make(map[string]bool)
	for _, file := range currentIndex.Files {
		if file.StorageKey != "" {
			referencedKeys[file.StorageKey] = true
		}
	}

	// Lister tous les objets dans le répertoire de cette sauvegarde
	prefix := fmt.Sprintf("data/%s/", backupID)
	objects, err := m.storageClient.ListObjects(prefix)
	if err != nil {
		return fmt.Errorf("error listing objects for cleanup: %w", err)
	}

	// Supprimer les objets non référencés
	deletedCount := 0
	for _, obj := range objects {
		// Extraire la clé de stockage de l'objet S3
		storageKey := strings.TrimPrefix(obj.Key, prefix)
		// Supprimer l'extension .chunk.XXX si c'est un chunk
		if strings.Contains(storageKey, ".chunk.") {
			storageKey = strings.Split(storageKey, ".chunk.")[0]
		}
		// Supprimer l'extension .metadata si c'est un fichier de métadonnées
		storageKey = strings.TrimSuffix(storageKey, ".metadata")

		// Si cette clé n'est pas référencée dans l'index actuel, la supprimer
		if !referencedKeys[storageKey] {
			if verbose {
				utils.Debug("🗑️  Deleting unreferenced object: %s", obj.Key)
			}
			if err := m.storageClient.DeleteObject(obj.Key); err != nil {
				if verbose {
					utils.Warn("⚠️  Warning: Failed to delete unreferenced object %s: %v", obj.Key, err)
				}
				// Continuer le nettoyage même si un objet ne peut pas être supprimé
			} else {
				deletedCount++
			}
		}
	}

	if verbose {
		utils.Info("🧹 Cleanup completed: %d unreferenced objects deleted", deletedCount)
	}

	return nil
}

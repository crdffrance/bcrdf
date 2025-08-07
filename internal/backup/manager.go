package backup

import (
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
	configFile    string
	config        *utils.Config
	indexMgr      *index.Manager
	encryptor     *crypto.EncryptorV2
	compressor    *compression.Compressor
	storageClient storage.Client
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

	if err := m.executeBackup(currentIndex, diff, backupID, verbose); err != nil {
		return err
	}

	m.logBackupCompletion(diff, time.Since(startTime), verbose)

	// Apply retention policy only if a backup was actually created
	if totalFilesToBackup > 0 {
		if err := m.applyRetentionPolicy(verbose); err != nil {
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
	retentionMgr := retention.NewManager(m.config, m.indexMgr, m.storageClient)
	return retentionMgr.ApplyRetentionPolicy(verbose)
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

// findLatestBackup finds the latest backup from the given keys
func (m *Manager) findLatestBackup(keys []string) (string, time.Time, error) {
	m.sortKeysByTimestamp(keys)

	for i := len(keys) - 1; i >= 0; i-- {
		key := keys[i]
		if !strings.HasSuffix(key, ".json") {
			continue
		}

		backupID := strings.TrimSuffix(strings.TrimPrefix(key, "indexes/"), ".json")
		utils.Debug("Checking backup ID: %s", backupID)

		index, err := m.indexMgr.LoadIndex(backupID)
		if err != nil {
			utils.Warn("Impossible de charger l'index %s: %v", backupID, err)
			continue
		}

		utils.Debug("Selected previous backup: %s (created: %s)", backupID, index.CreatedAt.Format("2006-01-02 15:04:05"))
		return key, index.CreatedAt, nil
	}

	return "", time.Time{}, nil
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

	utils.Info("Previous backup found: %s (created on %s)",
		backupID, latestTime.Format("2006-01-02 15:04:05"))

	previousIndex, err := m.indexMgr.LoadIndex(backupID)
	if err != nil {
		utils.Warn("Unable to load previous backup index: %v", err)
		return nil, nil
	}

	return previousIndex, nil
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

	// Optimisation : Trier les fichiers par taille (petits en premier)
	if m.config.Backup.SortBySize {
		if verbose {
			utils.Info("Sorting %d files by size (smallest first)...", len(allFiles))
		} else {
			utils.ProgressStep(fmt.Sprintf("Sorting %d files by size", len(allFiles)))
		}

		// Trier par taille croissante (petits fichiers en premier)
		sort.Slice(allFiles, func(i, j int) bool {
			return allFiles[i].Size < allFiles[j].Size
		})

		if verbose {
			utils.Info("Backing up %d files (sorted by size)", len(allFiles))
		} else {
			utils.ProgressStep(fmt.Sprintf("Backing up %d files (smallest first)", len(allFiles)))
		}
	} else {
		if verbose {
			utils.Info("Backing up %d files (original order)", len(allFiles))
		} else {
			utils.ProgressStep(fmt.Sprintf("Backing up %d files", len(allFiles)))
		}
	}

	// Créer un pool de workers pour le traitement parallèle
	semaphore := make(chan struct{}, m.config.Backup.MaxWorkers)
	var wg sync.WaitGroup
	errors := make(chan error, len(allFiles))

	// Barre de progression pour le mode non-verbeux
	var progressBar *utils.ProgressBar
	if !verbose {
		progressBar = utils.NewProgressBar(int64(len(allFiles)))
	}

	completed := int64(0)
	var completedMutex sync.Mutex

	for _, file := range allFiles {
		wg.Add(1)
		go func(f index.FileEntry) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquérir un slot
			defer func() { <-semaphore }() // Libérer le slot

			if err := m.backupSingleFile(f, backupID); err != nil {
				errors <- fmt.Errorf("error saving de %s: %w", f.Path, err)
			}

			// Mettre à jour la progression
			if !verbose {
				completedMutex.Lock()
				completed++
				progressBar.Update(completed)
				completedMutex.Unlock()
			}
		}(file)
	}

	wg.Wait()
	close(errors)

	// Terminer la barre de progression
	if !verbose && progressBar != nil {
		progressBar.Finish()
	}

	// Vérifier s'il y a eu des erreurs
	for err := range errors {
		if verbose {
			utils.Error("%v", err)
		} else {
			utils.ProgressError(err.Error())
		}
	}

	return nil
}

// backupSingleFile sauvegarde un seul fichier
func (m *Manager) backupSingleFile(file index.FileEntry, backupID string) error {
	if file.IsDirectory {
		return nil // Ignorer les répertoires
	}

	utils.Debug("Sauvegarde du fichier: %s", file.Path)

	// Check file size to determine processing method
	fileSize := file.Size
	largeFileThreshold := int64(100 * 1024 * 1024) // 100MB

	if fileSize > largeFileThreshold {
		utils.Debug("Large file detected (%d bytes), using streaming method", fileSize)
		return m.backupLargeFile(file, backupID)
	}

	// Lire le fichier source avec buffer optimisé
	bufferSize, err := utils.ParseBufferSize(m.config.Backup.BufferSize)
	if err != nil {
		utils.Debug("Invalid buffer size, using default: %v", err)
		bufferSize = 64 * 1024 * 1024 // 64MB default
	}

	data, err := utils.ReadFileWithBuffer(file.Path, bufferSize)
	if err != nil {
		return fmt.Errorf("error during la lecture: %w", err)
	}

	// Compress les données avec compression adaptative
	compressedData, err := m.compressor.CompressFile(data, file.Path)
	if err != nil {
		return fmt.Errorf("error compressing: %w", err)
	}

	// Chiffrer les données compressedes
	encryptedData, err := m.encryptor.Encrypt(compressedData)
	if err != nil {
		return fmt.Errorf("erreur lors du chiffrement: %w", err)
	}

	// Sauvegarder dans le stockage avec retry
	storageKey := fmt.Sprintf("data/%s/%s", backupID, file.GetStorageKey())
	if err := m.saveToStorageWithRetry(storageKey, encryptedData); err != nil {
		return fmt.Errorf("error saving: %w", err)
	}

	// Mettre à jour la clé de stockage dans l'entrée du fichier
	file.StorageKey = storageKey

	utils.Debug("File backed up: %s -> %s", file.Path, storageKey)
	return nil
}

// backupLargeFile sauvegarde un gros fichier en streaming
func (m *Manager) backupLargeFile(file index.FileEntry, backupID string) error {
	utils.Debug("Processing large file: %s (%d bytes, %.2f MB)", file.Path, file.Size, float64(file.Size)/1024/1024)

	// Get thresholds from config or use defaults
	largeThresholdStr := m.config.Backup.LargeFileThreshold
	if largeThresholdStr == "" {
		largeThresholdStr = "100MB" // Default
	}

	ultraLargeThresholdStr := m.config.Backup.UltraLargeThreshold
	if ultraLargeThresholdStr == "" {
		ultraLargeThresholdStr = "5GB" // Default
	}

	largeThreshold, err := parseSizeString(largeThresholdStr)
	if err != nil {
		utils.Warn("Invalid large_file_threshold config, using default 100MB: %v", err)
		largeThreshold = 100 * 1024 * 1024 // 100MB default
	}

	ultraLargeThreshold, err := parseSizeString(ultraLargeThresholdStr)
	if err != nil {
		utils.Warn("Invalid ultra_large_threshold config, using default 5GB: %v", err)
		ultraLargeThreshold = 5 * 1024 * 1024 * 1024 // 5GB default
	}

	utils.Debug("File size: %d bytes (%.2f MB), thresholds: large=%s, ultra=%s",
		file.Size, float64(file.Size)/1024/1024, largeThresholdStr, ultraLargeThresholdStr)

	// For extremely large files (> ultra threshold), use ultra-conservative approach
	if file.Size > ultraLargeThreshold {
		utils.Debug("Processing extremely large file (> %s) with ultra-conservative settings: %s", ultraLargeThresholdStr, file.Path)
		return m.backupUltraLargeFile(file, backupID)
	}

	// For very large files (large threshold - ultra threshold), use conservative approach
	if file.Size > largeThreshold {
		utils.Debug("Processing very large file (%s - %s) with conservative settings: %s", largeThresholdStr, ultraLargeThresholdStr, file.Path)
		return m.backupVeryLargeFile(file, backupID)
	}

	// For large files (100MB - large threshold), use standard large file approach
	utils.Debug("Processing large file (100MB - %s) with standard settings: %s", largeThresholdStr, file.Path)
	return m.backupStandardLargeFile(file, backupID)
}

// backupUltraLargeFile sauvegarde les fichiers extrêmement volumineux (> 5GB)
func (m *Manager) backupUltraLargeFile(file index.FileEntry, backupID string) error {
	utils.Debug("Processing ultra-large file: %s (%d bytes, %.2f MB)", file.Path, file.Size, float64(file.Size)/1024/1024)

	// Read file in chunks and process each chunk
	fileHandle, err := os.Open(file.Path)
	if err != nil {
		return fmt.Errorf("error opening ultra-large file: %w", err)
	}
	defer fileHandle.Close()

	storageKey := fmt.Sprintf("data/%s/%s", backupID, file.GetStorageKey())
	utils.Debug("Starting chunked upload for ultra-large file: %s", file.Path)

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

	utils.Debug("Using chunk size: %s (%d bytes) for ultra-large file", chunkSizeStr, chunkSize)

	chunkNumber := 0
	totalProcessed := int64(0)

	// Calculate total chunks for progress bar
	totalChunks := (file.Size + chunkSize - 1) / chunkSize // Ceiling division

	// Show progress bar for large file processing
	fileName := filepath.Base(file.Path)
	utils.ProgressStep(fmt.Sprintf("Processing large file: %s (%.2f MB)",
		fileName, float64(file.Size)/1024/1024))

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

		// Show progress for each chunk with file name for clarity
		progress := float64(chunkNumber+1) / float64(totalChunks) * 100
		utils.ProgressStep(fmt.Sprintf("[%s] Chunk %d/%d (%.1f%%) - %.2f MB / %.2f MB",
			fileName, chunkNumber+1, totalChunks, progress,
			float64(totalProcessed)/1024/1024, float64(file.Size)/1024/1024))

		utils.Debug("Processing chunk %d: %d bytes (%.2f MB), total: %.2f MB / %.2f MB",
			chunkNumber, n, float64(n)/1024/1024, float64(totalProcessed)/1024/1024, float64(file.Size)/1024/1024)

		// Encrypt chunk (no compression for ultra-large files)
		encryptedChunk, err := m.encryptor.Encrypt(chunk)
		if err != nil {
			return fmt.Errorf("error encrypting chunk %d: %w", chunkNumber, err)
		}

		// Upload chunk
		chunkKey := fmt.Sprintf("%s.chunk.%03d", storageKey, chunkNumber)
		if err := m.saveToStorageWithRetry(chunkKey, encryptedChunk); err != nil {
			return fmt.Errorf("error uploading chunk %d: %w", chunkNumber, err)
		}

		chunkNumber++
	}

	// Show completion
	utils.ProgressSuccess(fmt.Sprintf("Large file completed: %s (%.2f MB in %d chunks)",
		filepath.Base(file.Path), float64(file.Size)/1024/1024, chunkNumber))

	// Create metadata file
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
	if err := m.saveToStorageWithRetry(metadataKey, metadataJSON); err != nil {
		return fmt.Errorf("error uploading metadata: %w", err)
	}

	file.StorageKey = storageKey
	utils.Debug("Ultra-large file backed up in %d chunks: %s -> %s", chunkNumber, file.Path, storageKey)
	return nil
}

// backupVeryLargeFile sauvegarde les fichiers très volumineux (100MB - 5GB) avec chunking
func (m *Manager) backupVeryLargeFile(file index.FileEntry, backupID string) error {
	fileName := filepath.Base(file.Path)
	utils.Debug("Processing very large file: %s (%d bytes, %.2f MB)", file.Path, file.Size, float64(file.Size)/1024/1024)

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
	utils.Debug("Starting chunked upload for very large file: %s", file.Path)

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

	utils.Debug("Using chunk size: %s (%d bytes) for very large file", chunkSizeStr, chunkSize)

	chunkNumber := 0
	totalProcessed := int64(0)

	// Calculate total chunks for progress bar
	totalChunks := (file.Size + chunkSize - 1) / chunkSize // Ceiling division

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

		utils.Debug("Processing chunk %d: %d bytes (%.2f MB), total: %.2f MB / %.2f MB",
			chunkNumber, n, float64(n)/1024/1024, float64(totalProcessed)/1024/1024, float64(file.Size)/1024/1024)

		// Skip compression for very large files to save memory
		utils.Debug("Skipping compression for very large file chunk: %s", file.Path)

		// Encrypt chunk (no compression for very large files)
		encryptedChunk, err := m.encryptor.Encrypt(chunk)
		if err != nil {
			return fmt.Errorf("error encrypting chunk %d: %w", chunkNumber, err)
		}

		// Upload chunk
		chunkKey := fmt.Sprintf("%s.chunk.%03d", storageKey, chunkNumber)
		if err := m.saveToStorageWithRetry(chunkKey, encryptedChunk); err != nil {
			return fmt.Errorf("error uploading chunk %d: %w", chunkNumber, err)
		}

		chunkNumber++
	}

	// Show completion
	utils.ProgressSuccess(fmt.Sprintf("Very large file completed: %s (%.2f MB in %d chunks)",
		fileName, float64(file.Size)/1024/1024, chunkNumber))

	// Create metadata file
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
	if err := m.saveToStorageWithRetry(metadataKey, metadataJSON); err != nil {
		return fmt.Errorf("error uploading metadata: %w", err)
	}

	file.StorageKey = storageKey
	utils.Debug("Very large file backed up in %d chunks: %s -> %s", chunkNumber, file.Path, storageKey)
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

	file.StorageKey = storageKey
	utils.ProgressSuccess(fmt.Sprintf("Large file completed: %s (%.2f MB)",
		fileName, float64(file.Size)/1024/1024))
	utils.Debug("Standard large file backed up: %s -> %s", file.Path, storageKey)
	return nil
}

// saveToStorageWithRetry sauvegarde avec retry en cas d'échec
func (m *Manager) saveToStorageWithRetry(key string, data []byte) error {
	maxRetries := m.config.Backup.RetryAttempts
	retryDelay := time.Duration(m.config.Backup.RetryDelay) * time.Second
	dataSize := len(data)

	// Log file size for debugging
	utils.Debug("Uploading file: %s (%d bytes, %.2f MB)", key, dataSize, float64(dataSize)/1024/1024)

	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := m.saveToStorage(key, data)
		if err == nil {
			utils.Debug("Upload successful: %s (%d bytes)", key, dataSize)
			return nil // Succès
		}

		if attempt < maxRetries {
			utils.Debug("Upload attempt %d failed for %s (%d bytes): %v, retrying in %v...",
				attempt+1, key, dataSize, err, retryDelay)
			time.Sleep(retryDelay)
		} else {
			return fmt.Errorf("upload failed after %d attempts for %s (%d bytes): %w", maxRetries+1, key, dataSize, err)
		}
	}

	return nil
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

	// Extraire le nombre et l'unité
	var number int64
	var unit string

	// Trouver le dernier caractère non-numérique
	for i := len(sizeStr) - 1; i >= 0; i-- {
		if sizeStr[i] < '0' || sizeStr[i] > '9' {
			numberStr := sizeStr[:i+1]
			unit = sizeStr[i+1:]

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
	unit = strings.ToUpper(unit)
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
		utils.Info("🚀 Starting backup: %s", backupName)
	} else {
		utils.ProgressStep(fmt.Sprintf("🚀 Starting backup: %s", backupName))
	}
}

// prepareBackup prepares the backup by loading config and initializing components
func (m *Manager) prepareBackup(sourcePath string) error {
	// Charger la configuration
	config, err := utils.LoadConfig(m.configFile)
	if err != nil {
		return fmt.Errorf("erreur lors du chargement de la configuration: %w", err)
	}
	m.config = config

	// Initialiser les composants
	if err := m.initializeComponents(); err != nil {
		return fmt.Errorf("error during l'initialisation: %w", err)
	}

	// Vérifier que le chemin source existe
	if !utils.FileExists(sourcePath) {
		return fmt.Errorf("le chemin source n'existe pas: %s", sourcePath)
	}

	return nil
}

// createCurrentIndex creates the current backup index
func (m *Manager) createCurrentIndex(sourcePath, backupID string, verbose bool) (*index.BackupIndex, error) {
	// Créer l'index de la sauvegarde actuelle avec le mode de checksum configuré
	checksumMode := m.config.Backup.ChecksumMode
	if checksumMode == "" {
		checksumMode = "fast" // Mode par défaut
	}

	if !verbose {
		utils.ProgressStep("Creating index...")
	}
	currentIndex, err := m.indexMgr.CreateIndexWithMode(sourcePath, backupID, checksumMode, verbose)
	if err != nil {
		return nil, fmt.Errorf("error creating index: %w", err)
	}

	return currentIndex, nil
}

// calculateBackupDiff calculates the difference between current and previous backup
func (m *Manager) calculateBackupDiff(currentIndex *index.BackupIndex, backupName string, verbose bool) (*index.IndexDiff, error) {
	// Chercher la sauvegarde précédente pour comparaison
	if !verbose {
		utils.ProgressStep("Searching for previous backup...")
	}
	previousIndex, err := m.findPreviousBackup(backupName)
	if err != nil {
		if verbose {
			utils.Warn("No previous backup found, performing full backup")
		} else {
			utils.ProgressInfo("First backup - performing full backup")
		}
	}

	// Comparer les index pour déterminer les changements
	var diff *index.IndexDiff
	if previousIndex != nil {
		if !verbose {
			utils.ProgressStep("Comparing indexes...")
		}
		diff, err = m.indexMgr.CompareIndexes(currentIndex, previousIndex)
		if err != nil {
			return nil, fmt.Errorf("error during la comparaison des index: %w", err)
		}
	} else {
		// Première sauvegarde, tous les fichiers sont nouveaux
		diff = &index.IndexDiff{
			Added:    currentIndex.Files,
			Modified: []index.FileEntry{},
			Deleted:  []index.FileEntry{},
		}
	}

	return diff, nil
}

// executeBackup executes the actual backup process
func (m *Manager) executeBackup(currentIndex *index.BackupIndex, diff *index.IndexDiff, backupID string, verbose bool) error {
	// Sauvegarder les fichiers modifiés/ajoutés
	if err := m.backupFiles(diff.Added, diff.Modified, backupID, verbose); err != nil {
		return fmt.Errorf("error saving des fichiers: %w", err)
	}

	// Optimisation : Préparer l'index en parallèle pendant les uploads
	if !verbose {
		utils.ProgressStep("Finalizing backup...")
	}

	// Mettre à jour les métadonnées de l'index de manière optimisée
	currentIndex.CompressedSize = m.calculateCompressedSize(diff.Added, diff.Modified)
	currentIndex.EncryptedSize = m.calculateEncryptedSize(diff.Added, diff.Modified)

	// Optimisation : Mettre à jour les métadonnées de performance
	currentIndex.TotalFiles = int64(len(diff.Added) + len(diff.Modified))
	currentIndex.CreatedAt = time.Now()

	// Sauvegarder l'index avec optimisation
	if err := m.indexMgr.SaveIndex(currentIndex); err != nil {
		return fmt.Errorf("error saving de l'index: %w", err)
	}

	return nil
}

// logBackupCompletion logs the completion of backup operation
func (m *Manager) logBackupCompletion(diff *index.IndexDiff, duration time.Duration, verbose bool) {
	if verbose {
		utils.Info("✅ Backup completed in %v", duration)
		utils.Info("📊 Statistics: %d files added, %d modified, %d deleted",
			len(diff.Added), len(diff.Modified), len(diff.Deleted))
	} else {
		utils.ProgressSuccess(fmt.Sprintf("✅ Backup completed in %v", duration))
		utils.ProgressInfo(fmt.Sprintf("📊 %d added, %d modified, %d deleted",
			len(diff.Added), len(diff.Modified), len(diff.Deleted)))
	}
}

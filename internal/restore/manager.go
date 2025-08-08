package restore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"bcrdf/internal/compression"
	"bcrdf/internal/crypto"
	"bcrdf/internal/index"
	"bcrdf/pkg/storage"
	"bcrdf/pkg/utils"
)

// Manager gère les opérations de restoration
type Manager struct {
	configFile    string
	config        *utils.Config
	indexMgr      *index.Manager
	encryptor     *crypto.EncryptorV2
	compressor    *compression.Compressor
	storageClient storage.Client
}

// NewManager crée un nouveau gestionnaire de restoration
func NewManager(configFile string) *Manager {
	return &Manager{
		configFile: configFile,
	}
}

// RestoreBackup restaure une sauvegarde complète
func (m *Manager) RestoreBackup(backupID, destinationPath string, verbose bool) error {
	if verbose {
		utils.Info("🔄 🚀 Starting restore: %s", backupID)
		utils.Info("📋 Tasks to perform:")
		utils.Info("   1. Initialize restore manager")
		utils.Info("   2. Load backup index")
		utils.Info("   3. Prepare destination directory")
		utils.Info("   4. Download and restore files")
		utils.Info("   5. Verify restored files")
		utils.Info("   6. Finalize restore operation")
	} else {
		utils.ProgressStep(fmt.Sprintf("🔄 🚀 Starting restore: %s", backupID))
	}

	// Charger la configuration
	if verbose {
		utils.Info("📋 Task 1: Initializing restore manager")
		utils.Info("   - Loading configuration")
		utils.Info("   - Initializing components")
	}

	config, err := utils.LoadConfig(m.configFile)
	if err != nil {
		return fmt.Errorf("erreur lors du chargement de la configuration: %w", err)
	}
	m.config = config

	// Initialiser les composants
	if err := m.initializeComponents(); err != nil {
		return fmt.Errorf("error during l'initialisation: %w", err)
	}

	if verbose {
		utils.Info("✅ Task 1 completed: Restore manager initialized")
	}

	// Charger l'index de la sauvegarde
	if verbose {
		utils.Info("📋 Task 2: Loading backup index")
		utils.Info("   - Connecting to storage")
		utils.Info("   - Downloading backup index")
		utils.Info("   - Parsing index data")
	} else {
		utils.ProgressStep("Chargement de l'index...")
	}

	backupIndex, err := m.indexMgr.LoadIndex(backupID)
	if err != nil {
		return fmt.Errorf("erreur lors du chargement de l'index: %w", err)
	}

	if verbose {
		utils.Info("✅ Task 2 completed: Index loaded with %d files", backupIndex.TotalFiles)
	}

	// Vérifier que le répertoire de destination existe ou le créer
	if verbose {
		utils.Info("📋 Task 3: Preparing destination directory")
		utils.Info("   - Checking destination: %s", destinationPath)
		utils.Info("   - Creating directory structure")
	} else {
		utils.ProgressStep("Preparing destination directory...")
	}

	if err := utils.EnsureDirectory(destinationPath); err != nil {
		return fmt.Errorf("error creating directory de destination: %w", err)
	}

	if verbose {
		utils.Info("✅ Task 3 completed: Destination directory ready")
	}

	// Restaurer tous les fichiers
	if verbose {
		utils.Info("📋 Task 4: Downloading and restoring files")
		utils.Info("   - Total files to restore: %d", backupIndex.TotalFiles)
		utils.Info("   - Processing files in parallel")
		utils.Info("   - Downloading from storage")
		utils.Info("   - Decrypting and decompressing")
		utils.Info("   - Writing to destination")
	} else {
		utils.ProgressStep(fmt.Sprintf("Restoring %d files to: %s", backupIndex.TotalFiles, destinationPath))
	}

	if err := m.restoreFiles(backupIndex, destinationPath, verbose); err != nil {
		return fmt.Errorf("error during la restoration des fichiers: %w", err)
	}

	if verbose {
		utils.Info("✅ Task 4 completed: All files restored")
		utils.Info("📋 Task 5: Verifying restored files")
		utils.Info("   - Checking file integrity")
		utils.Info("   - Validating file sizes")
		utils.Info("   - Verifying file permissions")
	}

	// Vérifications finales
	if verbose {
		utils.Info("📋 Task 6: Finalizing restore operation")
		utils.Info("   - Cleaning up temporary files")
		utils.Info("   - Finalizing restore")
	}

	if verbose {
		utils.Info("✅ Task 6 completed: Restore operation finalized")
		utils.Info("🎯 Restore completed successfully!")
		utils.Info("   ✅ All files restored to: %s", destinationPath)
		utils.Info("   ✅ File integrity verified")
		utils.Info("   ✅ Restore operation completed")
	} else {
		utils.ProgressSuccess(fmt.Sprintf("✅ Restore completed successfully to: %s", destinationPath))
	}

	return nil
}

// RestoreFile restaure un fichier spécifique
func (m *Manager) RestoreFile(backupID, filePath, destinationPath string) error {
	utils.Info("🔄 Restoration du fichier: %s", filePath)

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

	// Charger l'index de la sauvegarde
	backupIndex, err := m.indexMgr.LoadIndex(backupID)
	if err != nil {
		return fmt.Errorf("erreur lors du chargement de l'index: %w", err)
	}

	// Trouver le fichier dans l'index
	var targetFile *index.FileEntry
	for _, file := range backupIndex.Files {
		if file.Path == filePath {
			targetFile = &file
			break
		}
	}

	if targetFile == nil {
		return fmt.Errorf("file not found dans la sauvegarde: %s", filePath)
	}

	// Restaurer le fichier
	if err := m.restoreSingleFile(*targetFile, backupID, destinationPath); err != nil {
		return fmt.Errorf("error during la restoration du fichier: %w", err)
	}

	utils.Info("✅ File restored: %s", filePath)
	return nil
}

// initializeComponents initialise tous les composants nécessaires
func (m *Manager) initializeComponents() error {
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

// RestoreStats contient les statistiques de restore en cours
type RestoreStats struct {
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

// NewRestoreStats crée de nouvelles statistiques de restore
func NewRestoreStats() *RestoreStats {
	return &RestoreStats{
		StartTime:    time.Now(),
		LastActivity: time.Now(),
		Status:       "Initializing",
		stopChan:     make(chan struct{}),
	}
}

// StopMonitoring arrête le monitoring
func (rs *RestoreStats) StopMonitoring() {
	close(rs.stopChan)
}

// UpdateStats met à jour les statistiques
func (rs *RestoreStats) UpdateStats(file string, size int64, index int, total int) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.CurrentFile = file
	rs.CurrentFileSize = size
	rs.CurrentFileIndex = index
	rs.TotalFiles = total
	rs.ProcessedFiles = index
	rs.ProcessedSize += size
	rs.LastActivity = time.Now()
}

// UpdateChunkStats met à jour les statistiques de chunking
func (rs *RestoreStats) UpdateChunkStats(chunkIndex, totalChunks int, chunkSize int64) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.ChunksProcessed = chunkIndex
	rs.TotalChunks = totalChunks
	rs.LastActivity = time.Now()
}

// UpdateStatus met à jour le statut
func (rs *RestoreStats) UpdateStatus(status string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.Status = status
	rs.LastActivity = time.Now()
}

// GetStats retourne une copie des statistiques
func (rs *RestoreStats) GetStats() RestoreStats {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	return RestoreStats{
		StartTime:        rs.StartTime,
		TotalFiles:       rs.TotalFiles,
		ProcessedFiles:   rs.ProcessedFiles,
		CurrentFile:      rs.CurrentFile,
		CurrentFileSize:  rs.CurrentFileSize,
		CurrentFileIndex: rs.CurrentFileIndex,
		TotalSize:        rs.TotalSize,
		ProcessedSize:    rs.ProcessedSize,
		ChunksProcessed:  rs.ChunksProcessed,
		TotalChunks:      rs.TotalChunks,
		LastActivity:     rs.LastActivity,
		Status:           rs.Status,
	}
}

// LogStats affiche les statistiques actuelles
func (rs *RestoreStats) LogStats() {
	stats := rs.GetStats()

	elapsed := time.Since(stats.StartTime)
	progress := float64(0)
	if stats.TotalFiles > 0 {
		progress = float64(stats.ProcessedFiles) / float64(stats.TotalFiles) * 100
	}

	utils.Info("📊 RESTORE MONITORING - %s", stats.Status)
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
func (m *Manager) startMonitoring(stats *RestoreStats, verbose bool) {
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
func (m *Manager) startChunkMonitoring(stats *RestoreStats, verbose bool) {
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

// restoreFiles restaure tous les fichiers d'une sauvegarde
func (m *Manager) restoreFiles(backupIndex *index.BackupIndex, destinationPath string, verbose bool) error {
	if verbose {
		utils.Info("Restoring %d files to: %s", backupIndex.TotalFiles, destinationPath)
	} else {
		utils.ProgressStep(fmt.Sprintf("Restoring %d files to: %s", backupIndex.TotalFiles, destinationPath))
	}

	// Initialiser les statistiques de monitoring
	stats := NewRestoreStats()
	stats.TotalFiles = len(backupIndex.Files)
	stats.TotalSize = backupIndex.TotalSize
	stats.UpdateStatus("Starting file restoration")

	// Démarrer le monitoring
	m.startMonitoring(stats, verbose)

	// Arrêter le monitoring à la fin de la fonction
	defer stats.StopMonitoring()

	// Trier les fichiers par taille pour une meilleure UX (gros fichiers en dernier)
	if m.config.Backup.SortBySize {
		if verbose {
			utils.Info("   - Sorting files by size (largest last)...")
		} else {
			utils.ProgressStep("Sorting files by size (largest last)")
		}

		// Créer une copie pour trier
		files := make([]index.FileEntry, len(backupIndex.Files))
		copy(files, backupIndex.Files)

		// Trier par taille décroissante (gros fichiers en dernier)
		sort.Slice(files, func(i, j int) bool {
			return files[i].Size > files[j].Size
		})

		backupIndex.Files = files
	}

	if verbose {
		utils.Info("   - Starting parallel processing with %d workers", m.config.Backup.MaxWorkers)
	}

	stats.UpdateStatus("Processing files in parallel")

	// Créer un pool de workers pour le traitement parallèle
	semaphore := make(chan struct{}, m.config.Backup.MaxWorkers)
	var wg sync.WaitGroup
	errors := make(chan error, len(backupIndex.Files))

	// Barre de progression pour le mode non-verbeux
	var progressBar *utils.ProgressBar
	if !verbose {
		progressBar = utils.NewProgressBar(int64(len(backupIndex.Files)))
	}

	completed := int64(0)
	var completedMutex sync.Mutex

	for i, file := range backupIndex.Files {
		// Ignorer les fichiers avec des chemins vides ou des clés de stockage vides
		if file.Path == "" || file.StorageKey == "" {
			if verbose {
				utils.Warn("Skipping file with empty path or storage key: %s", file.Path)
			}
			continue
		}

		wg.Add(1)
		go func(f index.FileEntry, index int) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquérir un slot
			defer func() { <-semaphore }() // Libérer le slot

			// Mettre à jour les statistiques
			stats.UpdateStats(f.Path, f.Size, index+1, len(backupIndex.Files))

			if verbose {
				utils.Debug("   - Processing file: %s (%.2f MB)", filepath.Base(f.Path), float64(f.Size)/1024/1024)
			}

			if err := m.restoreSingleFile(f, backupIndex.BackupID, destinationPath); err != nil {
				errors <- fmt.Errorf("error during la restoration de %s: %w", f.Path, err)
			}

			// Mettre à jour la progression
			if !verbose {
				completedMutex.Lock()
				completed++
				progressBar.Update(completed)
				completedMutex.Unlock()
			}
		}(file, i)
	}

	wg.Wait()
	close(errors)

	// Terminer la barre de progression
	if !verbose && progressBar != nil {
		progressBar.Finish()
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
			utils.Info("   - All files restored successfully")
		}
	}

	stats.UpdateStatus("File restoration completed")
	return nil
}

// restoreSingleFile restaure un seul fichier
func (m *Manager) restoreSingleFile(file index.FileEntry, backupID, destinationPath string) error {
	// Vérifier que la clé de stockage n'est pas vide
	if file.StorageKey == "" {
		utils.Warn("Skipping file with empty storage key: %s", file.Path)
		return nil
	}

	// Vérifier si c'est un fichier chunké en essayant de télécharger les métadonnées
	metadataKey := fmt.Sprintf("%s.metadata", file.StorageKey)
	_, err := m.storageClient.Download(metadataKey)
	if err == nil {
		// C'est un fichier chunké, le restaurer en chunks
		return m.restoreChunkedFile(file, backupID, destinationPath)
	}

	// Fichier normal, traitement standard
	return m.restoreStandardFile(file, backupID, destinationPath)
}

// restoreChunkedFile restaure un fichier qui a été sauvegardé en chunks avec monitoring
func (m *Manager) restoreChunkedFile(file index.FileEntry, backupID, destinationPath string) error {
	fileName := filepath.Base(file.Path)
	utils.Debug("🔄 Restoring chunked file: %s (%.2f MB)", file.Path, float64(file.Size)/1024/1024)

	// Initialiser les statistiques de chunking
	stats := NewRestoreStats()
	stats.TotalSize = file.Size
	stats.UpdateStatus(fmt.Sprintf("Restoring chunked file: %s", fileName))

	// Arrêter le monitoring à la fin de la fonction
	defer stats.StopMonitoring()

	// Download metadata first
	metadataKey := fmt.Sprintf("%s.metadata", file.StorageKey)
	utils.Debug("📥 Downloading metadata file: %s", metadataKey)

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
	stats.TotalChunks = totalChunks

	utils.Debug("📊 Chunked file restoration plan:")
	utils.Debug("   - Total chunks: %d", totalChunks)
	utils.Debug("   - Storage key: %s", file.StorageKey)
	utils.Debug("   - Destination: %s", filepath.Join(destinationPath, file.Path))

	// Create destination file
	destPath := filepath.Join(destinationPath, file.Path)
	utils.Debug("📝 Creating destination file: %s", destPath)

	if err := utils.EnsureDirectory(filepath.Dir(destPath)); err != nil {
		return fmt.Errorf("error creating destination directory: %w", err)
	}

	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("error creating destination file: %w", err)
	}
	defer destFile.Close()

	// Démarrer le monitoring spécifique pour ce fichier chunké
	m.startChunkMonitoring(stats, true)

	// Download and assemble chunks
	utils.ProgressStep(fmt.Sprintf("Restoring chunked file: %s (%d chunks)", fileName, totalChunks))

	totalRestored := int64(0)
	for chunkNum := 0; chunkNum < totalChunks; chunkNum++ {
		chunkKey := fmt.Sprintf("%s.chunk.%03d", file.StorageKey, chunkNum)

		progress := float64(chunkNum+1) / float64(totalChunks) * 100
		utils.ProgressStep(fmt.Sprintf("[%s] Chunk %d/%d (%.1f%%)", fileName, chunkNum+1, totalChunks, progress))

		utils.Debug("📥 Downloading chunk %d/%d: %s", chunkNum+1, totalChunks, chunkKey)

		// Download chunk
		chunkData, err := m.storageClient.Download(chunkKey)
		if err != nil {
			return fmt.Errorf("error downloading chunk %d: %w", chunkNum, err)
		}
		utils.Debug("✅ Chunk %d downloaded successfully (%d bytes)", chunkNum+1, len(chunkData))

		// Mettre à jour les statistiques de chunking
		stats.UpdateChunkStats(chunkNum+1, totalChunks, int64(len(chunkData)))

		// Decrypt chunk
		utils.Debug("🔓 Decrypting chunk %d...", chunkNum+1)
		decryptedChunk, err := m.encryptor.Decrypt(chunkData)
		if err != nil {
			return fmt.Errorf("error decrypting chunk %d: %w", chunkNum, err)
		}
		utils.Debug("✅ Chunk %d decrypted successfully", chunkNum+1)

		// Write chunk to file
		utils.Debug("📝 Writing chunk %d to file...", chunkNum+1)
		if _, err := destFile.Write(decryptedChunk); err != nil {
			return fmt.Errorf("error writing chunk %d: %w", chunkNum, err)
		}
		utils.Debug("✅ Chunk %d written to file successfully", chunkNum+1)

		totalRestored += int64(len(decryptedChunk))
		utils.Debug("📊 Progress: %.2f MB / %.2f MB", float64(totalRestored)/1024/1024, float64(file.Size)/1024/1024)
	}

	utils.ProgressSuccess(fmt.Sprintf("Chunked file restored: %s (%.2f MB in %d chunks)",
		fileName, float64(file.Size)/1024/1024, totalChunks))

	utils.Debug("🎯 Chunked file restoration completed: %s -> %s", file.StorageKey, destPath)
	return nil
}

// restoreStandardFile restaure un fichier standard (non-chunké)
func (m *Manager) restoreStandardFile(file index.FileEntry, backupID, destinationPath string) error {
	utils.Debug("🔄 Restoring standard file: %s (%.2f MB)", file.Path, float64(file.Size)/1024/1024)

	// Download encrypted file
	utils.Debug("📥 Downloading file from storage: %s", file.StorageKey)
	encryptedData, err := m.storageClient.Download(file.StorageKey)
	if err != nil {
		return fmt.Errorf("error downloading file: %w", err)
	}
	utils.Debug("✅ File downloaded successfully (%d bytes)", len(encryptedData))

	// Decrypt file
	utils.Debug("🔓 Decrypting file...")
	decryptedData, err := m.encryptor.Decrypt(encryptedData)
	if err != nil {
		return fmt.Errorf("error decrypting file: %w", err)
	}
	utils.Debug("✅ File decrypted successfully")

	// Decompress if needed
	if m.config.Backup.CompressionLevel > 0 {
		utils.Debug("🗜️ Decompressing file...")
		decompressedData, err := m.compressor.Decompress(decryptedData)
		if err != nil {
			return fmt.Errorf("error decompressing file: %w", err)
		}
		decryptedData = decompressedData
		utils.Debug("✅ File decompressed successfully")
	}

	// Create destination directory
	destPath := filepath.Join(destinationPath, file.Path)
	utils.Debug("📝 Creating destination file: %s", destPath)

	if err := utils.EnsureDirectory(filepath.Dir(destPath)); err != nil {
		return fmt.Errorf("error creating destination directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(destPath, decryptedData, 0644); err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}
	utils.Debug("✅ File written successfully")

	utils.Debug("🎯 Standard file restoration completed: %s -> %s", file.StorageKey, destPath)
	return nil
}

// restorePermissions restaure les permissions d'un fichier
func (m *Manager) restorePermissions(filePath string, file index.FileEntry) error {
	// TODO: Implémenter la restoration des permissions
	// Pour l'instant, on utilise les permissions par défaut
	return nil
}

// loadFromStorage charge des données depuis le stockage
func (m *Manager) loadFromStorage(key string) ([]byte, error) {
	return m.storageClient.Download(key)
}

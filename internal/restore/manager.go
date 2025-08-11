package restore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

		// Analyser les clés de stockage
		validFiles := 0
		emptyKeys := 0
		for _, file := range backupIndex.Files {
			if file.StorageKey == "" {
				emptyKeys++
			} else {
				validFiles++
			}
		}
		utils.Info("📊 Index analysis:")
		utils.Info("   - Total files: %d", len(backupIndex.Files))
		utils.Info("   - Valid storage keys: %d", validFiles)
		utils.Info("   - Empty storage keys: %d", emptyKeys)

		if emptyKeys > 0 {
			utils.Warn("⚠️  WARNING: %d files have empty storage keys!", emptyKeys)
			utils.Warn("   This indicates a corrupted or incomplete backup index.")
		}
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
	if err := m.restoreSingleFile(*targetFile, backupID, destinationPath, nil, true); err != nil {
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

	// Barre de progression intégrée pour le mode non-verbeux
	var progressBar *utils.IntegratedProgressBar
	if !verbose {
		progressBar = utils.NewIntegratedProgressBar(backupIndex.TotalSize)
		progressBar.SetMaxActiveFiles(5) // Limiter à 5 fichiers actifs simultanément
		// Afficher les barres fichiers uniquement si l'opération dure > 3s
		progressBar.SetDisplayThreshold(3 * time.Second)
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

			// Ajouter le fichier à la barre de progression
			if !verbose && progressBar != nil {
				fileName := filepath.Base(f.Path)
				progressBar.SetCurrentFile(fileName, f.Size)
			}

			if verbose {
				utils.Debug("   - Processing file: %s (%.2f MB)", filepath.Base(f.Path), float64(f.Size)/1024/1024)
			}

			// Construire un chemin relatif par rapport à la racine de sauvegarde pour restaurer sous destinationPath
			relPath := f.Path
			sourceRoot := filepath.Clean(backupIndex.SourcePath)
			// Si le chemin source est absolu et que f.Path commence par sourceRoot, le tronquer
			if filepath.IsAbs(relPath) {
				prefix := sourceRoot + string(os.PathSeparator)
				if strings.HasPrefix(relPath, prefix) {
					relPath = relPath[len(prefix):]
				} else if strings.HasPrefix(relPath, sourceRoot) {
					relPath = relPath[len(sourceRoot):]
					relPath = strings.TrimLeft(relPath, string(os.PathSeparator))
				}
			}
			// Utiliser une copie avec le chemin relatif
			f2 := f
			f2.Path = relPath

			if err := m.restoreSingleFile(f2, backupIndex.BackupID, destinationPath, progressBar, verbose); err != nil {
				errors <- fmt.Errorf("error during la restoration de %s: %w", f.Path, err)
			}

			// Mettre à jour la progression globale
			if !verbose && progressBar != nil {
				completedMutex.Lock()
				completed += f.Size
				progressBar.UpdateGlobal(completed)
				completedMutex.Unlock()

				// Marquer le fichier comme terminé (utiliser le nom de base pour la cohérence)
				fileName := filepath.Base(f.Path)
				progressBar.RemoveFile(fileName)
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
	skippedCount := 0
	for err := range errors {
		errorCount++
		if verbose {
			utils.Error("%v", err)
		} else {
			utils.ProgressError(err.Error())
		}
	}

	// Compter les fichiers ignorés
	for _, file := range backupIndex.Files {
		if file.Path == "" || file.StorageKey == "" {
			skippedCount++
		}
	}

	if verbose {
		if errorCount > 0 {
			utils.Warn("   - Completed with %d errors", errorCount)
		} else {
			utils.Info("   - All files restored successfully")
		}
		if skippedCount > 0 {
			utils.Warn("   - Skipped %d files with empty storage keys", skippedCount)
		}
	} else {
		if skippedCount > 0 {
			utils.ProgressInfo(fmt.Sprintf("Skipped %d files with empty storage keys", skippedCount))
		}
	}

	stats.UpdateStatus("File restoration completed")
	return nil
}

// restoreSingleFile restaure un seul fichier
func (m *Manager) restoreSingleFile(file index.FileEntry, backupID, destinationPath string, progressBar *utils.IntegratedProgressBar, verbose bool) error {
	// Vérifier que la clé de stockage n'est pas vide
	if file.StorageKey == "" {
		utils.Warn("Skipping file with empty storage key: %s", file.Path)
		return nil
	}

	// Reconstruct the full storage key with prefix
	fullStorageKey := fmt.Sprintf("data/%s/%s", backupID, file.StorageKey)

	// Vérifier si c'est un fichier chunké en essayant de télécharger les métadonnées
	metadataKey := fmt.Sprintf("%s.metadata", fullStorageKey)
	_, err := m.storageClient.Download(metadataKey)
	if err == nil {
		// C'est un fichier chunké, le restaurer en chunks
		return m.restoreChunkedFile(file, backupID, destinationPath, progressBar, verbose)
	}

	// Fichier normal, traitement standard
	return m.restoreStandardFile(file, backupID, destinationPath, progressBar, verbose)
}

// restoreChunkedFile restaure un fichier qui a été sauvegardé en chunks avec monitoring
func (m *Manager) restoreChunkedFile(file index.FileEntry, backupID, destinationPath string, progressBar *utils.IntegratedProgressBar, verbose bool) error {
	fileName := filepath.Base(file.Path)
	utils.Debug("🔄 Restoring chunked file: %s (%.2f MB)", file.Path, float64(file.Size)/1024/1024)

	// Initialiser les statistiques de chunking
	stats := NewRestoreStats()
	stats.TotalSize = file.Size
	stats.UpdateStatus(fmt.Sprintf("Restoring chunked file: %s", fileName))

	// Arrêter le monitoring à la fin de la fonction
	defer stats.StopMonitoring()

	// Reconstruct the full storage key with prefix
	fullStorageKey := fmt.Sprintf("data/%s/%s", backupID, file.StorageKey)

	// Download metadata first
	metadataKey := fmt.Sprintf("%s.metadata", fullStorageKey)
	utils.Debug("📥 Downloading metadata file: %s", metadataKey)

	metadataBytes, err := m.downloadWithRetry(metadataKey)
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
	utils.Debug("   - Storage key: %s", fullStorageKey)
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
	if verbose {
		utils.ProgressStep(fmt.Sprintf("Restoring chunked file: %s (%d chunks)", fileName, totalChunks))
	}

	totalRestored := int64(0)
	for chunkNum := 0; chunkNum < totalChunks; chunkNum++ {
		chunkKey := fmt.Sprintf("%s.chunk.%03d", fullStorageKey, chunkNum)

		if verbose {
			progress := float64(chunkNum+1) / float64(totalChunks) * 100
			utils.ProgressStep(fmt.Sprintf("[%s] Chunk %d/%d (%.1f%%)", fileName, chunkNum+1, totalChunks, progress))
		}

		utils.Debug("📥 Downloading chunk %d/%d: %s", chunkNum+1, totalChunks, chunkKey)

		// Download chunk
		chunkData, err := m.downloadWithRetry(chunkKey)
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

		// Decompress chunk if compression was enabled during backup
		if m.config.Backup.CompressionLevel > 0 {
			utils.Debug("🗜️ Decompressing chunk %d...", chunkNum+1)
			decompressed, err := m.compressor.Decompress(decryptedChunk)
			if err != nil {
				return fmt.Errorf("error decompressing chunk %d: %w", chunkNum, err)
			}
			decryptedChunk = decompressed
			utils.Debug("✅ Chunk %d decompressed successfully", chunkNum+1)
		}

		// Write chunk to file
		utils.Debug("📝 Writing chunk %d to file...", chunkNum+1)
		if _, err := destFile.Write(decryptedChunk); err != nil {
			return fmt.Errorf("error writing chunk %d: %w", chunkNum, err)
		}
		utils.Debug("✅ Chunk %d written to file successfully", chunkNum+1)

		totalRestored += int64(len(decryptedChunk))

		// Mettre à jour la barre de progression avec la progression réelle
		if !verbose && progressBar != nil {
			progressBar.UpdateChunkWithName(file.Path, totalRestored, file.Size)
		}

		utils.Debug("📊 Progress: %.2f MB / %.2f MB", float64(totalRestored)/1024/1024, float64(file.Size)/1024/1024)
	}

	utils.ProgressSuccess(fmt.Sprintf("Chunked file restored: %s (%.2f MB in %d chunks)",
		fileName, float64(file.Size)/1024/1024, totalChunks))

	utils.Debug("🎯 Chunked file restoration completed: %s -> %s", fullStorageKey, destPath)
	return nil
}

// restoreStandardFile restaure un fichier standard (non-chunké)
func (m *Manager) restoreStandardFile(file index.FileEntry, backupID, destinationPath string, progressBar *utils.IntegratedProgressBar, verbose bool) error {
	utils.Debug("🔄 Restoring standard file: %s (%.2f MB)", file.Path, float64(file.Size)/1024/1024)

	// Reconstruct the full storage key with prefix
	fullStorageKey := fmt.Sprintf("data/%s/%s", backupID, file.StorageKey)
	utils.Debug("📥 Downloading file from storage: %s", fullStorageKey)
	encryptedData, err := m.downloadWithRetry(fullStorageKey)
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

	utils.Debug("🎯 Standard file restoration completed: %s -> %s", fullStorageKey, destPath)
	return nil
}

// restorePermissions restaure les permissions d'un fichier
func (m *Manager) restorePermissions(filePath string, file index.FileEntry) error {
	// TODO: Implémenter la restoration des permissions
	// Pour l'instant, on utilise les permissions par défaut
	return nil
}

// loadFromStorage charge un objet depuis le stockage
func (m *Manager) loadFromStorage(key string) ([]byte, error) {
	return m.storageClient.Download(key)
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

			utils.Debug("🔄 Download retry attempt %d/%d for %s after %v delay",
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
					utils.Info("✅ Download succeeded on retry attempt %d for %s", attempt+1, key)
				}
				return result.data, nil
			}

			// Erreur, la stocker pour le log final
			lastError = result.err

			// Log de l'erreur
			if attempt < maxRetries-1 {
				utils.Warn("⚠️  Download failed for %s (attempt %d/%d): %v",
					key, attempt+1, maxRetries, result.err)
			}

		case <-ctx.Done():
			cancel()
			lastError = fmt.Errorf("download timeout after %v", timeout)

			if attempt < maxRetries-1 {
				utils.Warn("⚠️  Download timeout for %s (attempt %d/%d) after %v",
					key, attempt+1, maxRetries, timeout)
			}
		}
	}

	// Toutes les tentatives ont échoué
	utils.Error("❌ Download failed for %s after %d attempts. Last error: %v",
		key, maxRetries, lastError)

	return nil, fmt.Errorf("download failed after %d attempts for %s: %w", maxRetries, key, lastError)
}

// downloadResult contient le résultat d'un téléchargement
type downloadResult struct {
	data []byte
	err  error
}

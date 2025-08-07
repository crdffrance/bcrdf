package restore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

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
		utils.Info("🔄 Starting restore: %s", backupID)
	} else {
		utils.ProgressStep(fmt.Sprintf("🔄 Starting restore: %s", backupID))
	}

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
	if !verbose {
		utils.ProgressStep("Chargement de l'index...")
	}
	backupIndex, err := m.indexMgr.LoadIndex(backupID)
	if err != nil {
		return fmt.Errorf("erreur lors du chargement de l'index: %w", err)
	}

	// Vérifier que le répertoire de destination existe ou le créer
	if !verbose {
		utils.ProgressStep("Preparing destination directory...")
	}
	if err := utils.EnsureDirectory(destinationPath); err != nil {
		return fmt.Errorf("error creating directory de destination: %w", err)
	}

	// Restaurer tous les fichiers
	if err := m.restoreFiles(backupIndex, destinationPath, verbose); err != nil {
		return fmt.Errorf("error during la restoration des fichiers: %w", err)
	}

	if verbose {
		utils.Info("✅ Restore completed: %s", backupID)
		utils.Info("📊 Statistics: %d files restored, total size: %d bytes",
			backupIndex.TotalFiles, backupIndex.TotalSize)
	} else {
		utils.ProgressSuccess(fmt.Sprintf("✅ Restore completed: %s", backupID))
		utils.ProgressInfo(fmt.Sprintf("📊 %d files restored, total size: %d bytes",
			backupIndex.TotalFiles, backupIndex.TotalSize))
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

// restoreFiles restaure tous les fichiers d'une sauvegarde
func (m *Manager) restoreFiles(backupIndex *index.BackupIndex, destinationPath string, verbose bool) error {
	if verbose {
		utils.Info("Restoring %d files to: %s", backupIndex.TotalFiles, destinationPath)
	} else {
		utils.ProgressStep(fmt.Sprintf("Restoring %d files to: %s", backupIndex.TotalFiles, destinationPath))
	}

	// Trier les fichiers par taille pour une meilleure UX (gros fichiers en dernier)
	if m.config.Backup.SortBySize {
		if verbose {
			utils.Info("Sorting files by size (largest last)...")
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

	for _, file := range backupIndex.Files {
		wg.Add(1)
		go func(f index.FileEntry) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquérir un slot
			defer func() { <-semaphore }() // Libérer le slot

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

// restoreSingleFile restaure un seul fichier
func (m *Manager) restoreSingleFile(file index.FileEntry, backupID, destinationPath string) error {
	if file.IsDirectory {
		// Créer le répertoire
		dirPath := filepath.Join(destinationPath, file.Path)
		if err := utils.EnsureDirectory(dirPath); err != nil {
			return fmt.Errorf("error creating directory: %w", err)
		}
		return nil
	}

	utils.Debug("Restoration du fichier: %s", file.Path)

	// Vérifier si c'est un fichier chunké
	storageKey := fmt.Sprintf("data/%s/%s", backupID, file.GetStorageKey())
	metadataKey := fmt.Sprintf("%s.metadata", storageKey)

	// Essayer de charger les métadonnées pour vérifier si c'est chunké
	metadataData, err := m.loadFromStorage(metadataKey)
	if err == nil {
		// C'est un fichier chunké, le restaurer en chunks
		return m.restoreChunkedFile(file, backupID, destinationPath, metadataData)
	}

	// Fichier normal, traitement standard
	return m.restoreStandardFile(file, backupID, destinationPath, storageKey)
}

// restoreChunkedFile restaure un fichier qui a été sauvegardé en chunks
func (m *Manager) restoreChunkedFile(file index.FileEntry, backupID, destinationPath string, metadataData []byte) error {
	utils.Debug("Restoring chunked file: %s", file.Path)

	// Parser les métadonnées
	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataData, &metadata); err != nil {
		return fmt.Errorf("error parsing metadata: %w", err)
	}

	chunks, ok := metadata["chunks"].(float64)
	if !ok {
		return fmt.Errorf("invalid metadata: chunks field not found")
	}

	storageKey, ok := metadata["storage_key"].(string)
	if !ok {
		return fmt.Errorf("invalid metadata: storage_key field not found")
	}

	// Créer le chemin de destination
	destPath := filepath.Join(destinationPath, file.Path)
	destDir := filepath.Dir(destPath)

	// Créer le répertoire parent si nécessaire
	if err := utils.EnsureDirectory(destDir); err != nil {
		return fmt.Errorf("error creating directory: %w", err)
	}

	// Ouvrir le fichier de destination
	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("error creating destination file: %w", err)
	}
	defer destFile.Close()

	// Restaurer chaque chunk
	totalChunks := int(chunks)
	var totalRestored int64

	for chunkNum := 0; chunkNum < totalChunks; chunkNum++ {
		chunkKey := fmt.Sprintf("%s.chunk.%03d", storageKey, chunkNum)

		// Charger le chunk
		encryptedChunk, err := m.loadFromStorage(chunkKey)
		if err != nil {
			return fmt.Errorf("error loading chunk %d: %w", chunkNum, err)
		}

		// Déchiffrer le chunk
		chunkData, err := m.encryptor.Decrypt(encryptedChunk)
		if err != nil {
			return fmt.Errorf("error decrypting chunk %d: %w", chunkNum, err)
		}

		// Écrire le chunk dans le fichier
		if _, err := destFile.Write(chunkData); err != nil {
			return fmt.Errorf("error writing chunk %d: %w", chunkNum, err)
		}

		totalRestored += int64(len(chunkData))

		// Afficher le progrès pour les gros fichiers
		if file.Size > 100*1024*1024 { // > 100MB
			progress := float64(chunkNum+1) / float64(totalChunks) * 100
			utils.ProgressStep(fmt.Sprintf("Chunk %d/%d (%.1f%%) - %.2f MB / %.2f MB",
				chunkNum+1, totalChunks, progress,
				float64(totalRestored)/1024/1024, float64(file.Size)/1024/1024))
		}
	}

	utils.Debug("Chunked file restored: %s (%d chunks, %.2f MB)",
		filepath.Base(file.Path), totalChunks, float64(totalRestored)/1024/1024)

	return nil
}

// restoreStandardFile restaure un fichier standard (non-chunké)
func (m *Manager) restoreStandardFile(file index.FileEntry, backupID, destinationPath, storageKey string) error {
	// Charger les données depuis le stockage
	encryptedData, err := m.loadFromStorage(storageKey)
	if err != nil {
		return fmt.Errorf("erreur lors du chargement depuis le stockage: %w", err)
	}

	// Déchiffrer les données
	compressedData, err := m.encryptor.Decrypt(encryptedData)
	if err != nil {
		return fmt.Errorf("error decrypting: %w", err)
	}

	// Decompress les données
	originalData, err := m.compressor.Decompress(compressedData)
	if err != nil {
		return fmt.Errorf("error decompressing: %w", err)
	}

	// Créer le chemin de destination (utiliser le chemin complet)
	destPath := filepath.Join(destinationPath, file.Path)
	destDir := filepath.Dir(destPath)

	// Créer le répertoire parent si nécessaire
	if err := utils.EnsureDirectory(destDir); err != nil {
		return fmt.Errorf("error creating directory parent: %w", err)
	}

	// Écrire le fichier restauré
	if err := utils.WriteFile(destPath, originalData); err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}

	// Restaurer les permissions (si possible)
	if err := m.restorePermissions(destPath, file); err != nil {
		utils.Warn("Impossible de restaurer les permissions pour %s: %v", file.Path, err)
	}

	utils.Debug("File restored: %s -> %s", file.Path, destPath)
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

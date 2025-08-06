package backup

import (
	"fmt"
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

	diff, err := m.calculateBackupDiff(currentIndex, verbose)
	if err != nil {
		return err
	}

	if err := m.executeBackup(currentIndex, diff, backupID, verbose); err != nil {
		return err
	}

	m.logBackupCompletion(diff, time.Since(startTime), verbose)

	// Apply retention policy after successful backup
	if err := m.applyRetentionPolicy(verbose); err != nil {
		// Don't fail the backup if retention fails, just warn
		if verbose {
			utils.Warn("Retention policy application failed: %v", err)
		} else {
			utils.ProgressWarning("Retention cleanup failed")
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
func (m *Manager) findPreviousBackup() (*index.BackupIndex, error) {
	// Charger la configuration si nécessaire
	if m.config == nil {
		config, err := utils.LoadConfig(m.configFile)
		if err != nil {
			return nil, err
		}
		m.config = config
	}

	// Initialiser le client S3 si nécessaire
	if m.storageClient == nil {
		storageClient, err := storage.NewStorageClient(m.config)
		if err != nil {
			return nil, fmt.Errorf("error during l'initialisation du client de stockage: %w", err)
		}
		m.storageClient = storageClient
	}

	// Lister les index disponibles
	objects, err := m.storageClient.ListObjects("indexes/")
	if err != nil {
		utils.Warn("Impossible de lister les index: %v", err)
		return nil, nil
	}

	// Extraire les clés des objets
	keys := make([]string, len(objects))
	for i, obj := range objects {
		keys[i] = obj.Key
	}

	if len(keys) == 0 {
		utils.Debug("No index found, first backup")
		return nil, nil
	}

	// Trouver l'index le plus récent
	var latestKey string
	var latestTime time.Time

	for _, key := range keys {
		if strings.HasSuffix(key, ".json") {
			// Extraire l'ID de sauvegarde du nom de fichier
			backupID := strings.TrimSuffix(strings.TrimPrefix(key, "indexes/"), ".json")

			// Charger l'index pour obtenir la date de création
			index, err := m.indexMgr.LoadIndex(backupID)
			if err != nil {
				utils.Warn("Impossible de charger l'index %s: %v", backupID, err)
				continue
			}

			// Vérifier si cet index est plus récent
			if index.CreatedAt.After(latestTime) {
				latestTime = index.CreatedAt
				latestKey = key
			}
		}
	}

	if latestKey == "" {
		utils.Debug("No valid index found")
		return nil, nil
	}

	// Extraire l'ID de la sauvegarde la plus récente
	backupID := strings.TrimSuffix(strings.TrimPrefix(latestKey, "indexes/"), ".json")

	utils.Info("Previous backup found: %s (created on %s)",
		backupID, latestTime.Format("2006-01-02 15:04:05"))

	// Charger l'index de la sauvegarde précédente
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

	if verbose {
		utils.Info("Sauvegarde de %d fichiers", len(allFiles))
	} else {
		utils.ProgressStep(fmt.Sprintf("Backing up %d files", len(allFiles)))
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

	// Sauvegarder dans le stockage
	storageKey := fmt.Sprintf("data/%s/%s", backupID, file.GetStorageKey())
	if err := m.saveToStorage(storageKey, encryptedData); err != nil {
		return fmt.Errorf("error saving: %w", err)
	}

	// Mettre à jour la clé de stockage dans l'entrée du fichier
	file.StorageKey = storageKey

	utils.Debug("File backed up: %s -> %s", file.Path, storageKey)
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
func (m *Manager) calculateBackupDiff(currentIndex *index.BackupIndex, verbose bool) (*index.IndexDiff, error) {
	// Chercher la sauvegarde précédente pour comparaison
	if !verbose {
		utils.ProgressStep("Searching for previous backup...")
	}
	previousIndex, err := m.findPreviousBackup()
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

	// Mettre à jour les métadonnées de l'index
	currentIndex.CompressedSize = m.calculateCompressedSize(diff.Added, diff.Modified)
	currentIndex.EncryptedSize = m.calculateEncryptedSize(diff.Added, diff.Modified)

	// Sauvegarder l'index
	if !verbose {
		utils.ProgressStep("Saving index...")
	}
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

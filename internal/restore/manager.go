package restore

import (
	"fmt"
	"path/filepath"
	"sync"

	"bcrdf/internal/compression"
	"bcrdf/internal/crypto"
	"bcrdf/internal/index"
	"bcrdf/pkg/storage"
	"bcrdf/pkg/utils"
)

// Manager gère les opérations de restauration
type Manager struct {
	configFile    string
	config        *utils.Config
	indexMgr      *index.Manager
	encryptor     *crypto.EncryptorV2
	compressor    *compression.Compressor
	storageClient storage.Client
}

// NewManager crée un nouveau gestionnaire de restauration
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
		return fmt.Errorf("erreur lors de l'initialisation: %w", err)
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
		return fmt.Errorf("erreur lors de la création du répertoire de destination: %w", err)
	}

	// Restaurer tous les fichiers
	if err := m.restoreFiles(backupIndex, destinationPath, verbose); err != nil {
		return fmt.Errorf("erreur lors de la restauration des fichiers: %w", err)
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
	utils.Info("🔄 Restauration du fichier: %s", filePath)

	// Charger la configuration
	config, err := utils.LoadConfig(m.configFile)
	if err != nil {
		return fmt.Errorf("erreur lors du chargement de la configuration: %w", err)
	}
	m.config = config

	// Initialiser les composants
	if err := m.initializeComponents(); err != nil {
		return fmt.Errorf("erreur lors de l'initialisation: %w", err)
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
		return fmt.Errorf("fichier non trouvé dans la sauvegarde: %s", filePath)
	}

	// Restaurer le fichier
	if err := m.restoreSingleFile(*targetFile, backupID, destinationPath); err != nil {
		return fmt.Errorf("erreur lors de la restauration du fichier: %w", err)
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
		return fmt.Errorf("erreur lors de l'initialisation du chiffreur: %w", err)
	}
	m.encryptor = encryptor

	// Initialiser le compresseur
	compressor, err := compression.NewCompressor(m.config.Backup.CompressionLevel)
	if err != nil {
		return fmt.Errorf("erreur lors de l'initialisation du compresseur: %w", err)
	}
	m.compressor = compressor

	// Initialiser le client de stockage
	storageClient, err := storage.NewStorageClient(m.config)
	if err != nil {
		return fmt.Errorf("erreur lors de l'initialisation du client de stockage: %w", err)
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
				errors <- fmt.Errorf("erreur lors de la restauration de %s: %w", f.Path, err)
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
			return fmt.Errorf("erreur lors de la création du répertoire: %w", err)
		}
		return nil
	}

	utils.Debug("Restauration du fichier: %s", file.Path)

	// Charger les données depuis le stockage
	storageKey := fmt.Sprintf("data/%s/%s", backupID, file.GetStorageKey())
	encryptedData, err := m.loadFromStorage(storageKey)
	if err != nil {
		return fmt.Errorf("erreur lors du chargement depuis le stockage: %w", err)
	}

	// Déchiffrer les données
	compressedData, err := m.encryptor.Decrypt(encryptedData)
	if err != nil {
		return fmt.Errorf("erreur lors du déchiffrement: %w", err)
	}

	// Décompresser les données
	originalData, err := m.compressor.Decompress(compressedData)
	if err != nil {
		return fmt.Errorf("erreur lors de la décompression: %w", err)
	}

	// Créer le chemin de destination
	destPath := filepath.Join(destinationPath, file.Path)
	destDir := filepath.Dir(destPath)

	// Créer le répertoire parent si nécessaire
	if err := utils.EnsureDirectory(destDir); err != nil {
		return fmt.Errorf("erreur lors de la création du répertoire parent: %w", err)
	}

	// Écrire le fichier restauré
	if err := utils.WriteFile(destPath, originalData); err != nil {
		return fmt.Errorf("erreur lors de l'écriture du fichier: %w", err)
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
	// TODO: Implémenter la restauration des permissions
	// Pour l'instant, on utilise les permissions par défaut
	return nil
}

// loadFromStorage charge des données depuis le stockage
func (m *Manager) loadFromStorage(key string) ([]byte, error) {
	return m.storageClient.Download(key)
}

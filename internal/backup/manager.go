package backup

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"bcrdf/internal/compression"
	"bcrdf/internal/crypto"
	"bcrdf/internal/index"
	"bcrdf/pkg/s3"
	"bcrdf/pkg/utils"
)

// Manager gère les opérations de sauvegarde
type Manager struct {
	configFile string
	config     *utils.Config
	indexMgr   *index.Manager
	encryptor  *crypto.EncryptorV2
	compressor *compression.Compressor
	s3Client   *s3.Client
}

// NewManager crée un nouveau gestionnaire de sauvegarde
func NewManager(configFile string) *Manager {
	return &Manager{
		configFile: configFile,
	}
}

// CreateBackup effectue une sauvegarde complète
func (m *Manager) CreateBackup(sourcePath, backupName string) error {
	utils.Info("🚀 Début de la sauvegarde: %s", backupName)
	startTime := time.Now()

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

	// Vérifier que le chemin source existe
	if !utils.FileExists(sourcePath) {
		return fmt.Errorf("le chemin source n'existe pas: %s", sourcePath)
	}

	// Créer l'ID de sauvegarde
	backupID := fmt.Sprintf("%s-%s", backupName, time.Now().Format("20060102-150405"))

	// Créer l'index de la sauvegarde actuelle
	currentIndex, err := m.indexMgr.CreateIndex(sourcePath, backupID)
	if err != nil {
		return fmt.Errorf("erreur lors de la création de l'index: %w", err)
	}

	// Chercher la sauvegarde précédente pour comparaison
	previousIndex, err := m.findPreviousBackup()
	if err != nil {
		utils.Warn("Aucune sauvegarde précédente trouvée, sauvegarde complète")
	}

	// Comparer les index pour déterminer les changements
	var diff *index.IndexDiff
	if previousIndex != nil {
		diff, err = m.indexMgr.CompareIndexes(currentIndex, previousIndex)
		if err != nil {
			return fmt.Errorf("erreur lors de la comparaison des index: %w", err)
		}
	} else {
		// Première sauvegarde, tous les fichiers sont nouveaux
		diff = &index.IndexDiff{
			Added:    currentIndex.Files,
			Modified: []index.FileEntry{},
			Deleted:  []index.FileEntry{},
		}
	}

	// Sauvegarder les fichiers modifiés/ajoutés
	if err := m.backupFiles(diff.Added, diff.Modified, backupID); err != nil {
		return fmt.Errorf("erreur lors de la sauvegarde des fichiers: %w", err)
	}

	// Mettre à jour les métadonnées de l'index
	currentIndex.CompressedSize = m.calculateCompressedSize(diff.Added, diff.Modified)
	currentIndex.EncryptedSize = m.calculateEncryptedSize(diff.Added, diff.Modified)

	// Sauvegarder l'index
	if err := m.indexMgr.SaveIndex(currentIndex); err != nil {
		return fmt.Errorf("erreur lors de la sauvegarde de l'index: %w", err)
	}

	duration := time.Since(startTime)
	utils.Info("✅ Sauvegarde terminée en %v", duration)
	utils.Info("📊 Statistiques: %d fichiers ajoutés, %d modifiés, %d supprimés",
		len(diff.Added), len(diff.Modified), len(diff.Deleted))

	return nil
}

// DeleteBackup supprime une sauvegarde
func (m *Manager) DeleteBackup(backupID string) error {
	utils.Info("🗑️ Suppression de la sauvegarde: %s", backupID)

	// Initialiser les composants si nécessaire
	if err := m.initializeComponents(); err != nil {
		return fmt.Errorf("erreur lors de l'initialisation des composants: %w", err)
	}

	// Charger l'index de la sauvegarde
	backupIndex, err := m.indexMgr.LoadIndex(backupID)
	if err != nil {
		return fmt.Errorf("erreur lors du chargement de l'index: %w", err)
	}

	// Supprimer les fichiers de données
	if err := m.deleteBackupFiles(backupIndex); err != nil {
		return fmt.Errorf("erreur lors de la suppression des fichiers: %w", err)
	}

	// Supprimer l'index
	if err := m.deleteBackupIndex(backupID); err != nil {
		return fmt.Errorf("erreur lors de la suppression de l'index: %w", err)
	}

	utils.Info("✅ Sauvegarde supprimée: %s", backupID)
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
		return fmt.Errorf("erreur lors de l'initialisation du chiffreur: %w", err)
	}
	m.encryptor = encryptor

	// Initialiser le compresseur
	compressor, err := compression.NewCompressor(m.config.Backup.CompressionLevel)
	if err != nil {
		return fmt.Errorf("erreur lors de l'initialisation du compresseur: %w", err)
	}
	m.compressor = compressor

	// Initialiser le client S3
	s3Client, err := s3.NewClient(
		m.config.Storage.AccessKey,
		m.config.Storage.SecretKey,
		m.config.Storage.Region,
		m.config.Storage.Endpoint,
		m.config.Storage.Bucket,
	)
	if err != nil {
		return fmt.Errorf("erreur lors de l'initialisation du client S3: %w", err)
	}
	m.s3Client = s3Client

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
	if m.s3Client == nil {
		s3Client, err := s3.NewClient(
			m.config.Storage.AccessKey,
			m.config.Storage.SecretKey,
			m.config.Storage.Region,
			m.config.Storage.Endpoint,
			m.config.Storage.Bucket,
		)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de l'initialisation du client S3: %w", err)
		}
		m.s3Client = s3Client
	}

	// Lister les index disponibles
	keys, err := m.s3Client.ListObjects("indexes/")
	if err != nil {
		utils.Warn("Impossible de lister les index: %v", err)
		return nil, nil
	}

	if len(keys) == 0 {
		utils.Debug("Aucun index trouvé, première sauvegarde")
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
		utils.Debug("Aucun index valide trouvé")
		return nil, nil
	}

	// Extraire l'ID de la sauvegarde la plus récente
	backupID := strings.TrimSuffix(strings.TrimPrefix(latestKey, "indexes/"), ".json")

	utils.Info("Sauvegarde précédente trouvée: %s (créée le %s)",
		backupID, latestTime.Format("2006-01-02 15:04:05"))

	// Charger l'index de la sauvegarde précédente
	previousIndex, err := m.indexMgr.LoadIndex(backupID)
	if err != nil {
		utils.Warn("Impossible de charger l'index de la sauvegarde précédente: %v", err)
		return nil, nil
	}

	return previousIndex, nil
}

// backupFiles sauvegarde les fichiers spécifiés
func (m *Manager) backupFiles(added, modified []index.FileEntry, backupID string) error {
	allFiles := append(added, modified...)

	if len(allFiles) == 0 {
		utils.Info("Aucun fichier à sauvegarder")
		return nil
	}

	utils.Info("Sauvegarde de %d fichiers", len(allFiles))

	// Créer un pool de workers pour le traitement parallèle
	semaphore := make(chan struct{}, m.config.Backup.MaxWorkers)
	var wg sync.WaitGroup
	errors := make(chan error, len(allFiles))

	for _, file := range allFiles {
		wg.Add(1)
		go func(f index.FileEntry) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquérir un slot
			defer func() { <-semaphore }() // Libérer le slot

			if err := m.backupSingleFile(f, backupID); err != nil {
				errors <- fmt.Errorf("erreur lors de la sauvegarde de %s: %w", f.Path, err)
			}
		}(file)
	}

	wg.Wait()
	close(errors)

	// Vérifier s'il y a eu des erreurs
	for err := range errors {
		utils.Error("%v", err)
	}

	return nil
}

// backupSingleFile sauvegarde un seul fichier
func (m *Manager) backupSingleFile(file index.FileEntry, backupID string) error {
	if file.IsDirectory {
		return nil // Ignorer les répertoires
	}

	utils.Debug("Sauvegarde du fichier: %s", file.Path)

	// Lire le fichier source
	data, err := utils.ReadFile(file.Path)
	if err != nil {
		return fmt.Errorf("erreur lors de la lecture: %w", err)
	}

	// Compresser les données
	compressedData, err := m.compressor.Compress(data)
	if err != nil {
		return fmt.Errorf("erreur lors de la compression: %w", err)
	}

	// Chiffrer les données compressées
	encryptedData, err := m.encryptor.Encrypt(compressedData)
	if err != nil {
		return fmt.Errorf("erreur lors du chiffrement: %w", err)
	}

	// Sauvegarder dans le stockage
	storageKey := fmt.Sprintf("data/%s/%s", backupID, file.GetStorageKey())
	if err := m.saveToStorage(storageKey, encryptedData); err != nil {
		return fmt.Errorf("erreur lors de la sauvegarde: %w", err)
	}

	// Mettre à jour la clé de stockage dans l'entrée du fichier
	file.StorageKey = storageKey

	utils.Debug("Fichier sauvegardé: %s -> %s", file.Path, storageKey)
	return nil
}

// calculateCompressedSize calcule la taille compressée totale
func (m *Manager) calculateCompressedSize(added, modified []index.FileEntry) int64 {
	// TODO: Implémenter le calcul de la taille compressée
	return 0
}

// calculateEncryptedSize calcule la taille chiffrée totale
func (m *Manager) calculateEncryptedSize(added, modified []index.FileEntry) int64 {
	// TODO: Implémenter le calcul de la taille chiffrée
	return 0
}

// deleteBackupFiles supprime les fichiers de données d'une sauvegarde
func (m *Manager) deleteBackupFiles(backupIndex *index.BackupIndex) error {
	utils.Info("Suppression des fichiers de données pour: %s", backupIndex.BackupID)

	for _, file := range backupIndex.Files {
		if file.StorageKey != "" {
			if err := m.s3Client.DeleteObject(file.StorageKey); err != nil {
				utils.Warn("Impossible de supprimer le fichier %s: %v", file.StorageKey, err)
			} else {
				utils.Debug("Fichier supprimé: %s", file.StorageKey)
			}
		}
	}

	return nil
}

// deleteBackupIndex supprime l'index d'une sauvegarde
func (m *Manager) deleteBackupIndex(backupID string) error {
	utils.Info("Suppression de l'index pour: %s", backupID)

	indexKey := fmt.Sprintf("indexes/%s.json", backupID)
	if err := m.s3Client.DeleteObject(indexKey); err != nil {
		return fmt.Errorf("erreur lors de la suppression de l'index: %w", err)
	}

	utils.Debug("Index supprimé: %s", indexKey)
	return nil
}

// saveToStorage sauvegarde des données dans le stockage
func (m *Manager) saveToStorage(key string, data []byte) error {
	return m.s3Client.Upload(key, data)
}

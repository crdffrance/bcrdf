package index

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"bcrdf/pkg/s3"
	"bcrdf/pkg/utils"
)

// Manager gère les opérations sur les index
type Manager struct {
	configFile string
	config     *utils.Config
	s3Client   *s3.Client
}

// NewManager crée un nouveau gestionnaire d'index
func NewManager(configFile string) *Manager {
	return &Manager{
		configFile: configFile,
	}
}

// CreateIndex crée un nouvel index pour un répertoire
func (m *Manager) CreateIndex(sourcePath, backupID string) (*BackupIndex, error) {
	utils.Info("Création de l'index pour: %s", sourcePath)

	index := &BackupIndex{
		BackupID:   backupID,
		CreatedAt:  time.Now(),
		SourcePath: sourcePath,
		Files:      []FileEntry{},
	}

	// Parcourir récursivement le répertoire source
	err := filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			utils.Warn("Erreur lors de l'accès à %s: %v", path, err)
			return nil // Continuer malgré l'erreur
		}

		// Ignorer les fichiers système et temporaires
		if shouldSkipFile(path, info) {
			return nil
		}

		// Créer une entrée pour ce fichier
		entry, err := NewFileEntry(path, info)
		if err != nil {
			utils.Warn("Erreur lors de la création de l'entrée pour %s: %v", path, err)
			return nil
		}

		index.Files = append(index.Files, *entry)
		index.TotalFiles++
		index.TotalSize += entry.Size

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("erreur lors du parcours du répertoire: %w", err)
	}

	utils.Info("Index créé avec %d fichiers, taille totale: %d bytes",
		index.TotalFiles, index.TotalSize)

	return index, nil
}

// LoadIndex charge un index depuis le stockage
func (m *Manager) LoadIndex(backupID string) (*BackupIndex, error) {
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

	// Construire le chemin de l'index
	indexKey := fmt.Sprintf("indexes/%s.json", backupID)

	// Charger depuis S3
	data, err := m.s3Client.Download(indexKey)
	if err != nil {
		return nil, fmt.Errorf("erreur lors du chargement de l'index: %w", err)
	}

	var index BackupIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("erreur lors du décodage de l'index: %w", err)
	}

	return &index, nil
}

// SaveIndex sauvegarde un index
func (m *Manager) SaveIndex(index *BackupIndex) error {
	// Charger la configuration si nécessaire
	if m.config == nil {
		config, err := utils.LoadConfig(m.configFile)
		if err != nil {
			return err
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
			return fmt.Errorf("erreur lors de l'initialisation du client S3: %w", err)
		}
		m.s3Client = s3Client
	}

	// Sérialiser l'index
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("erreur lors de la sérialisation de l'index: %w", err)
	}

	// Sauvegarder dans S3
	indexKey := fmt.Sprintf("indexes/%s.json", index.BackupID)
	if err := m.s3Client.Upload(indexKey, data); err != nil {
		return fmt.Errorf("erreur lors de la sauvegarde de l'index: %w", err)
	}

	utils.Info("Index sauvegardé: %s", indexKey)
	return nil
}

// CompareIndexes compare deux index et retourne les différences
func (m *Manager) CompareIndexes(current, previous *BackupIndex) (*IndexDiff, error) {
	utils.Info("Comparaison des index: %s vs %s", current.BackupID, previous.BackupID)

	diff := &IndexDiff{
		Added:    []FileEntry{},
		Modified: []FileEntry{},
		Deleted:  []FileEntry{},
	}

	// Créer des maps pour une recherche rapide
	currentMap := make(map[string]FileEntry)
	previousMap := make(map[string]FileEntry)

	for _, file := range current.Files {
		currentMap[file.Path] = file
	}

	for _, file := range previous.Files {
		previousMap[file.Path] = file
	}

	// Trouver les fichiers ajoutés et modifiés
	for path, currentFile := range currentMap {
		if previousFile, exists := previousMap[path]; !exists {
			// Nouveau fichier
			diff.Added = append(diff.Added, currentFile)
		} else if currentFile.IsModified(&previousFile) {
			// Fichier modifié
			diff.Modified = append(diff.Modified, currentFile)
		}
	}

	// Trouver les fichiers supprimés
	for path, previousFile := range previousMap {
		if _, exists := currentMap[path]; !exists {
			// Fichier supprimé
			diff.Deleted = append(diff.Deleted, previousFile)
		}
	}

	utils.Info("Différences trouvées: %d ajoutés, %d modifiés, %d supprimés",
		len(diff.Added), len(diff.Modified), len(diff.Deleted))

	return diff, nil
}

// ListBackups liste toutes les sauvegardes disponibles
func (m *Manager) ListBackups(backupID string) error {
	// Si un backupID spécifique est fourni, afficher ses détails
	if backupID != "" {
		return m.showBackupDetails(backupID)
	}

	// Lister les index depuis S3
	indexes, err := m.listIndexes()
	if err != nil {
		return fmt.Errorf("erreur lors de la récupération des sauvegardes: %w", err)
	}

	if len(indexes) == 0 {
		utils.Info("Aucune sauvegarde trouvée")
		return nil
	}

	// Trier par date de création (plus récent en premier)
	sort.Slice(indexes, func(i, j int) bool {
		return indexes[i].CreatedAt.After(indexes[j].CreatedAt)
	})

	fmt.Printf("\n📋 Sauvegardes disponibles:\n")
	fmt.Printf("%-20s %-25s %-15s %-12s %-12s\n",
		"ID", "Date", "Fichiers", "Taille", "Comprimé")
	fmt.Printf("%s\n", strings.Repeat("-", 90))

	for _, backup := range indexes {
		sizeMB := float64(backup.TotalSize) / 1024 / 1024
		compressedMB := float64(backup.CompressedSize) / 1024 / 1024

		fmt.Printf("%-20s %-20s %-15d %-12.1f MB %-12.1f MB\n",
			backup.BackupID,
			backup.CreatedAt.Format("2006-01-02 15:04:05"),
			backup.TotalFiles,
			sizeMB,
			compressedMB)
	}

	fmt.Printf("\nTotal: %d sauvegardes\n", len(indexes))
	return nil
}

// showBackupDetails affiche les détails d'une sauvegarde spécifique
func (m *Manager) showBackupDetails(backupID string) error {
	// Charger l'index de la sauvegarde
	index, err := m.LoadIndex(backupID)
	if err != nil {
		return fmt.Errorf("erreur lors du chargement de l'index %s: %w", backupID, err)
	}

	fmt.Printf("\n📋 Détails de la sauvegarde: %s\n", backupID)
	fmt.Printf("%s\n", strings.Repeat("-", 60))
	fmt.Printf("Date de création: %s\n", index.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Chemin source: %s\n", index.SourcePath)
	fmt.Printf("Nombre de fichiers: %d\n", index.TotalFiles)
	fmt.Printf("Taille totale: %.1f MB\n", float64(index.TotalSize)/(1024*1024))
	fmt.Printf("Taille compressée: %.1f MB\n", float64(index.CompressedSize)/(1024*1024))
	fmt.Printf("Taille chiffrée: %.1f MB\n", float64(index.EncryptedSize)/(1024*1024))

	fmt.Printf("\n📁 Fichiers:\n")
	for i, file := range index.Files {
		sizeKB := float64(file.Size) / 1024
		fmt.Printf("  [%d] %s (%.1f KB) -> %s\n",
			i+1, file.Path, sizeKB, file.GetStorageKey())
	}

	return nil
}

// IndexDiff représente les différences entre deux index
type IndexDiff struct {
	Added    []FileEntry `json:"added"`
	Modified []FileEntry `json:"modified"`
	Deleted  []FileEntry `json:"deleted"`
}

// shouldSkipFile détermine si un fichier doit être ignoré
func shouldSkipFile(path string, info os.FileInfo) bool {
	// Ignorer les fichiers cachés
	if strings.HasPrefix(filepath.Base(path), ".") {
		return true
	}

	// Ignorer les fichiers temporaires
	tempPatterns := []string{
		".tmp", ".temp", ".swp", ".bak", ".backup",
		"~", "#", ".DS_Store", "Thumbs.db",
	}

	for _, pattern := range tempPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	// Ignorer les répertoires système
	systemDirs := []string{
		"/proc", "/sys", "/dev", "/tmp", "/var/tmp",
	}

	for _, dir := range systemDirs {
		if strings.HasPrefix(path, dir) {
			return true
		}
	}

	return false
}

// listIndexes liste les index depuis S3
func (m *Manager) listIndexes() ([]BackupMetadata, error) {
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

	// Lister les objets dans le préfixe indexes/
	keys, err := m.s3Client.ListObjects("indexes/")
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la liste des index: %w", err)
	}

	var backups []BackupMetadata
	for _, key := range keys {
		// Extraire l'ID de sauvegarde du nom de fichier
		if strings.HasSuffix(key, ".json") {
			backupID := strings.TrimSuffix(strings.TrimPrefix(key, "indexes/"), ".json")

			// Charger l'index pour obtenir les métadonnées
			index, err := m.LoadIndex(backupID)
			if err != nil {
				utils.Warn("Impossible de charger l'index %s: %v", backupID, err)
				continue
			}

			backup := BackupMetadata{
				BackupID:       index.BackupID,
				CreatedAt:      index.CreatedAt,
				SourcePath:     index.SourcePath,
				TotalFiles:     index.TotalFiles,
				TotalSize:      index.TotalSize,
				CompressedSize: index.CompressedSize,
				EncryptedSize:  index.EncryptedSize,
				Status:         "completed",
			}
			backups = append(backups, backup)
		}
	}

	return backups, nil
}

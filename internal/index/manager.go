package index

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"bcrdf/internal/crypto"
	"bcrdf/pkg/storage"
	"bcrdf/pkg/utils"
)

// Manager gère les opérations sur les index
type Manager struct {
	configFile    string
	config        *utils.Config
	storageClient storage.Client
	checksumCache *ChecksumCache
	encryptor     *crypto.EncryptorV2
}

// NewManager crée un nouveau gestionnaire d'index
func NewManager(configFile string) *Manager {
	return &Manager{
		configFile:    configFile,
		checksumCache: NewChecksumCache(),
	}
}

// initializeEncryptor initialise le chiffreur si nécessaire
func (m *Manager) initializeEncryptor() error {
	if m.encryptor != nil {
		return nil
	}

	// Charger la configuration si nécessaire
	if m.config == nil {
		config, err := utils.LoadConfig(m.configFile)
		if err != nil {
			return err
		}
		m.config = config
	}

	// Initialiser le chiffreur avec l'algorithme configuré
	algorithm := crypto.EncryptionAlgorithm(m.config.Backup.EncryptionAlgo)
	if algorithm == "" {
		algorithm = crypto.AES256GCM // Valeur par défaut
	}

	encryptor, err := crypto.NewEncryptorV2(m.config.Backup.EncryptionKey, algorithm)
	if err != nil {
		return fmt.Errorf("error during l'initialisation du chiffreur pour les index: %w", err)
	}
	m.encryptor = encryptor

	return nil
}

// CreateIndex crée un nouvel index pour un répertoire
func (m *Manager) CreateIndex(sourcePath, backupID string, verbose bool) (*BackupIndex, error) {
	return m.CreateIndexWithMode(sourcePath, backupID, "fast", verbose)
}

// CreateIndexWithMode crée un nouvel index avec un mode de checksum spécifique
func (m *Manager) CreateIndexWithMode(sourcePath, backupID, checksumMode string, verbose bool) (*BackupIndex, error) {
	if verbose {
		utils.Info("Creating index for: %s (mode: %s)", sourcePath, checksumMode)
	}

	index := m.initializeIndex(backupID, sourcePath)

	fileCount, err := m.countFiles(sourcePath, checksumMode, verbose)
	if err != nil {
		return nil, err
	}

	progressBar := m.setupProgressBar(verbose, fileCount)

	err = m.processFiles(sourcePath, checksumMode, verbose, index, progressBar)
	if err != nil {
		return nil, err
	}

	// Terminer la barre de progression
	if !verbose && progressBar != nil {
		progressBar.Finish()
	}

	if verbose {
		utils.Info("Index created with %d files, total size: %d bytes",
			index.TotalFiles, index.TotalSize)

		// Show cache statistics
		stats := m.checksumCache.GetStats()
		if stats.Hits > 0 || stats.Misses > 0 {
			hitRate := float64(stats.Hits) / float64(stats.Hits+stats.Misses) * 100
			utils.Info("Cache performance: %d hits, %d misses (%.1f%% hit rate)",
				stats.Hits, stats.Misses, hitRate)
		}
	} else {
		utils.ProgressDone(fmt.Sprintf("Index created with %d files", index.TotalFiles))
	}

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
	if m.storageClient == nil {
		storageClient, err := storage.NewStorageClient(m.config)
		if err != nil {
			return nil, fmt.Errorf("error initializing storage client: %w", err)
		}
		m.storageClient = storageClient
	}

	// Initialiser le chiffreur si nécessaire
	if err := m.initializeEncryptor(); err != nil {
		return nil, fmt.Errorf("error initializing encryptor for index loading: %w", err)
	}

	// Construire le chemin de l'index
	indexKey := fmt.Sprintf("indexes/%s.json", backupID)

	// Charger depuis S3
	data, err := m.storageClient.Download(indexKey)
	if err != nil {
		return nil, fmt.Errorf("error loading index: %w", err)
	}

	// Déchiffrer les données
	decryptedData, err := m.encryptor.Decrypt(data)
	if err != nil {
		return nil, fmt.Errorf("error decrypting index: %w", err)
	}

	var index BackupIndex
	if err := json.Unmarshal(decryptedData, &index); err != nil {
		return nil, fmt.Errorf("error decoding index: %w", err)
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

	// Initialiser le client de stockage si nécessaire
	if m.storageClient == nil {
		storageClient, err := storage.NewStorageClient(m.config)
		if err != nil {
			return fmt.Errorf("error initializing storage client: %w", err)
		}
		m.storageClient = storageClient
	}

	// Initialiser le chiffreur si nécessaire
	if err := m.initializeEncryptor(); err != nil {
		return fmt.Errorf("error initializing encryptor for index saving: %w", err)
	}

	// Sérialiser l'index
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("error serializing index: %w", err)
	}

	// Chiffrer les données
	encryptedData, err := m.encryptor.Encrypt(data)
	if err != nil {
		return fmt.Errorf("error encrypting index: %w", err)
	}

	// Sauvegarder dans S3
	indexKey := fmt.Sprintf("indexes/%s.json", index.BackupID)
	if err := m.storageClient.Upload(indexKey, encryptedData); err != nil {
		return fmt.Errorf("error saving index: %w", err)
	}

	utils.Info("Index saved: %s", indexKey)
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
			utils.Debug("📁 Added: %s", path)
		} else {
			// Vérifier si le fichier a été modifié
			if m.isFileModified(&currentFile, &previousFile) {
				diff.Modified = append(diff.Modified, currentFile)
				utils.Debug("🔄 Modified: %s", path)
			} else {
				utils.Debug("✅ Unchanged: %s", path)
			}
		}
	}

	// Trouver les fichiers deleted
	for path, previousFile := range previousMap {
		if _, exists := currentMap[path]; !exists {
			// File deleted
			diff.Deleted = append(diff.Deleted, previousFile)
			utils.Debug("🗑️  Deleted: %s", path)
		}
	}

	utils.Info("Differences found: %d added, %d modified, %d deleted",
		len(diff.Added), len(diff.Modified), len(diff.Deleted))

	return diff, nil
}

// isFileModified détermine si un fichier a été modifié avec une logique améliorée
func (m *Manager) isFileModified(current, previous *FileEntry) bool {
	// Vérifier d'abord la taille (le plus rapide)
	if current.Size != previous.Size {
		utils.Debug("   Size changed: %d -> %d", previous.Size, current.Size)
		return true
	}

	// Vérifier le temps de modification
	if !current.ModifiedTime.Equal(previous.ModifiedTime) {
		utils.Debug("   Modification time changed: %s -> %s",
			previous.ModifiedTime.Format("2006-01-02 15:04:05"),
			current.ModifiedTime.Format("2006-01-02 15:04:05"))
		return true
	}

	// Vérifier les permissions
	if current.Permissions != previous.Permissions {
		utils.Debug("   Permissions changed: %s -> %s", previous.Permissions, current.Permissions)
		return true
	}

	// Vérifier le checksum seulement si nécessaire
	if current.Checksum != previous.Checksum {
		utils.Debug("   Checksum changed: %s -> %s", previous.Checksum[:8], current.Checksum[:8])
		return true
	}

	return false
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
		return fmt.Errorf("error retrieving backups: %w", err)
	}

	if len(indexes) == 0 {
		utils.Info("No backup found")
		return nil
	}

	// Trier par date de création (plus récent en premier)
	sort.Slice(indexes, func(i, j int) bool {
		return indexes[i].CreatedAt.After(indexes[j].CreatedAt)
	})

	fmt.Printf("\n📋 Available backups:\n")
	fmt.Printf("%-20s %-25s %-15s %-12s %-12s\n",
		"ID", "Date", "Files", "Size", "Compressed")
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

	fmt.Printf("\nTotal: %d backups\n", len(indexes))
	return nil
}

// showBackupDetails affiche les détails d'une sauvegarde spécifique
func (m *Manager) showBackupDetails(backupID string) error {
	// Charger l'index de la sauvegarde
	index, err := m.LoadIndex(backupID)
	if err != nil {
		return fmt.Errorf("error loading index %s: %w", backupID, err)
	}

	fmt.Printf("\n📋 Backup details: %s\n", backupID)
	fmt.Printf("%s\n", strings.Repeat("-", 60))
	fmt.Printf("Created: %s\n", index.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Source path: %s\n", index.SourcePath)
	fmt.Printf("Files: %d\n", index.TotalFiles)
	fmt.Printf("Total size: %.1f MB\n", float64(index.TotalSize)/(1024*1024))
	fmt.Printf("Compressed size: %.1f MB\n", float64(index.CompressedSize)/(1024*1024))
	fmt.Printf("Encrypted size: %.1f MB\n", float64(index.EncryptedSize)/(1024*1024))

	fmt.Printf("\n📁 Files:\n")
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

	// Ignorer les répertoires système (sauf /tmp pour les tests)
	systemDirs := []string{
		"/proc", "/sys", "/dev", "/var/tmp",
	}

	for _, dir := range systemDirs {
		if strings.HasPrefix(path, dir) {
			return true
		}
	}

	return false
}

// shouldSkipFileWithConfig determines if a file should be skipped based on config patterns
func (m *Manager) shouldSkipFileWithConfig(path string, info os.FileInfo) bool {
	// First check basic skip rules
	if shouldSkipFile(path, info) {
		return true
	}

	// Skip directories by default (only backup files, not directories)
	if info.IsDir() {
		return true
	}

	// Check configured skip patterns
	if m.config != nil && len(m.config.Backup.SkipPatterns) > 0 {
		relativePath := filepath.Base(path)
		fullPath := path

		for _, pattern := range m.config.Backup.SkipPatterns {
			// Handle directory patterns (ending with /)
			if strings.HasSuffix(pattern, "/") {
				dirPattern := strings.TrimSuffix(pattern, "/")
				if info.IsDir() && (strings.Contains(fullPath, dirPattern) || relativePath == dirPattern) {
					return true
				}
				// Skip files inside the directory
				if strings.Contains(fullPath, "/"+dirPattern+"/") {
					return true
				}
			} else {
				// Handle file patterns with wildcards
				if strings.Contains(pattern, "*") {
					if matched, _ := filepath.Match(pattern, relativePath); matched {
						return true
					}
					// Also check the full path for patterns like "*.log"
					if matched, _ := filepath.Match(pattern, filepath.Base(fullPath)); matched {
						return true
					}
				} else {
					// Exact match
					if relativePath == pattern || strings.Contains(fullPath, pattern) {
						return true
					}
				}
			}
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
	if m.storageClient == nil {
		storageClient, err := storage.NewStorageClient(m.config)
		if err != nil {
			return nil, fmt.Errorf("error initializing storage client: %w", err)
		}
		m.storageClient = storageClient
	}

	// Lister les objets dans le préfixe indexes/
	objects, err := m.storageClient.ListObjects("indexes/")
	if err != nil {
		return nil, fmt.Errorf("error listing indexes: %w", err)
	}

	// Extraire les clés des objets
	keys := make([]string, len(objects))
	for i, obj := range objects {
		keys[i] = obj.Key
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

// initializeIndex initializes a new backup index
func (m *Manager) initializeIndex(backupID, sourcePath string) *BackupIndex {
	return &BackupIndex{
		BackupID:   backupID,
		CreatedAt:  time.Now(),
		SourcePath: sourcePath,
		Files:      []FileEntry{},
	}
}

// countFiles counts the total number of files to process
func (m *Manager) countFiles(sourcePath, checksumMode string, verbose bool) (int64, error) {
	var fileCount int64

	// Load configuration for skip patterns
	if m.config == nil {
		config, err := utils.LoadConfig(m.configFile)
		if err != nil {
			return 0, fmt.Errorf("error loading config: %w", err)
		}
		m.config = config
	}

	modeDesc := map[string]string{
		"full":     "🔄 Analyzing directory (full integrity)...",
		"fast":     "🔄 Analyzing directory (fast mode)...",
		"metadata": "🔄 Analyzing directory (metadata only)...",
	}
	desc := modeDesc[checksumMode]
	if desc == "" {
		desc = "🔄 Analyzing directory..."
	}
	utils.ProgressStep(desc)

	err := filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil || m.shouldSkipFileWithConfig(path, info) {
			return nil
		}

		// Only count files, not directories
		if !info.IsDir() {
			fileCount++
		}

		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("error counting files: %w", err)
	}

	return fileCount, nil
}

// setupProgressBar creates and initializes a progress bar if needed
func (m *Manager) setupProgressBar(verbose bool, fileCount int64) *utils.ProgressBar {
	if verbose || fileCount <= 0 {
		return nil
	}
	return utils.NewProgressBar(fileCount)
}

// processFiles walks through the source directory and processes each file
func (m *Manager) processFiles(sourcePath, checksumMode string, verbose bool, index *BackupIndex, progressBar *utils.ProgressBar) error {
	processed := int64(0)

	// Load configuration for skip patterns
	if m.config == nil {
		config, err := utils.LoadConfig(m.configFile)
		if err != nil {
			return fmt.Errorf("error loading config: %w", err)
		}
		m.config = config
	}

	return filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if verbose {
				utils.Warn("Error accessing %s: %v", path, err)
			}
			return nil // Continue despite error
		}

		if m.shouldSkipFileWithConfig(path, info) {
			if verbose {
				utils.Debug("Skipping file: %s", path)
			}
			return nil
		}

		if verbose {
			utils.Debug("Processing file: %s", path)
		}

		entry, err := NewFileEntryWithModeAndCache(path, info, checksumMode, m.checksumCache)
		if err != nil {
			if verbose {
				utils.Warn("Error creating entry for %s: %v", path, err)
			}
			return nil
		}

		index.Files = append(index.Files, *entry)
		index.TotalFiles++
		index.TotalSize += entry.Size

		if !verbose && progressBar != nil {
			processed++
			progressBar.Update(processed)
		}

		return nil
	})
}

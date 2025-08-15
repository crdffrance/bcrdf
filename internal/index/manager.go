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

	// Construire le chemin de l'index
	indexKey := fmt.Sprintf("indexes/%s.json", backupID)

	// Charger depuis S3
	data, err := m.storageClient.Download(indexKey)
	if err != nil {
		return nil, fmt.Errorf("error loading index: %w", err)
	}

	// Initialiser le chiffreur si nécessaire
	if err := m.initializeEncryptor(); err != nil {
		return nil, fmt.Errorf("error initializing encryptor for index loading: %w", err)
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

	// Sérialiser l'index
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("error serializing index: %w", err)
	}

	// Initialiser le chiffreur si nécessaire
	if err := m.initializeEncryptor(); err != nil {
		return fmt.Errorf("error initializing encryptor for index saving: %w", err)
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

		// Ignorer les fichiers vides (ils ne seront pas sauvegardés)
		if info.Size() == 0 {
			if verbose {
				utils.Debug("Skipping empty file: %s", path)
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

		// Générer la StorageKey immédiatement
		entry.StorageKey = entry.GetStorageKey()

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

// CleanOrphanedFiles nettoie les fichiers orphelins sur le stockage
// qui ne correspondent pas à l'index de sauvegarde
// Si backupID est vide, nettoie toutes les sauvegardes
func (m *Manager) CleanOrphanedFiles(backupID string, dryRun, verbose bool) error {
	if verbose {
		utils.ProgressStep("Loading backup index...")
	}

	// Charger l'index de sauvegarde
	index, err := m.LoadIndex(backupID)
	if err != nil {
		return fmt.Errorf("failed to load index for backup %s: %w", backupID, err)
	}

	if verbose {
		utils.ProgressDone("Index loaded successfully")
		utils.ProgressStep("Initializing storage client...")
	}

	// Initialiser le client de stockage
	if m.storageClient == nil {
		if m.config == nil {
			config, err := utils.LoadConfig(m.configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			m.config = config
		}

		storageClient, err := storage.NewStorageClient(m.config)
		if err != nil {
			return fmt.Errorf("failed to initialize storage client: %w", err)
		}
		m.storageClient = storageClient
	}

	if verbose {
		utils.ProgressDone("Storage client initialized")
		utils.ProgressStep("Scanning storage for orphaned files...")
	}

	// Créer un ensemble des clés de stockage valides depuis l'index
	validKeys := make(map[string]bool)
	for _, file := range index.Files {
		if !file.IsDirectory {
			// Construire la clé de stockage complète avec le préfixe data/backupID/
			fullStorageKey := fmt.Sprintf("data/%s/%s", backupID, file.StorageKey)
			validKeys[fullStorageKey] = true

			// Si c'est un gros fichier, ajouter aussi tous ses chunks comme valides
			if file.Size > 100*1024*1024 { // 100MB - seuil pour les fichiers chunkés
				// Ajouter le fichier de métadonnées
				metadataKey := fmt.Sprintf("%s.metadata", fullStorageKey)
				validKeys[metadataKey] = true

				// Ajouter tous les chunks possibles (on ne connaît pas le nombre exact)
				// On va ajouter jusqu'à 1000 chunks pour couvrir la plupart des cas
				for chunkNum := 0; chunkNum < 1000; chunkNum++ {
					chunkKey := fmt.Sprintf("%s.chunk.%03d", fullStorageKey, chunkNum)
					validKeys[chunkKey] = true
				}
			}
		}
	}

	// Lister tous les objets sur le stockage avec le préfixe du backup
	// Utiliser le préfixe data/ au lieu de backups/ pour correspondre à la structure réelle
	backupPrefix := fmt.Sprintf("data/%s/", backupID)
	objects, err := m.storageClient.ListObjects(backupPrefix)
	if err != nil {
		return fmt.Errorf("failed to list objects on storage: %w", err)
	}

	// Identifier les fichiers orphelins
	var orphanedFiles []storage.ObjectInfo
	for _, obj := range objects {
		// Ignorer les fichiers d'index et de métadonnées
		if strings.HasSuffix(obj.Key, ".index") || strings.HasSuffix(obj.Key, ".metadata") {
			continue
		}

		// Vérifier si la clé existe dans l'index
		if !validKeys[obj.Key] {
			orphanedFiles = append(orphanedFiles, obj)
		}
	}

	if verbose {
		utils.ProgressDone(fmt.Sprintf("Found %d orphaned files", len(orphanedFiles)))
	}

	// Afficher les fichiers orphelins
	if len(orphanedFiles) == 0 {
		utils.ProgressSuccess("No orphaned files found. Storage is clean!")
		return nil
	}

	// Afficher le résumé
	totalOrphanedSize := int64(0)
	for _, obj := range orphanedFiles {
		totalOrphanedSize += obj.Size
	}

	fmt.Printf("\n🔍 Orphaned files found:\n")
	fmt.Printf("%s\n", strings.Repeat("-", 80))
	fmt.Printf("Total files: %d\n", len(orphanedFiles))
	fmt.Printf("Total size: %s\n", formatBytes(totalOrphanedSize))
	fmt.Printf("Backup ID: %s\n", backupID)
	fmt.Printf("Mode: %s\n", map[bool]string{true: "DRY RUN (no files will be deleted)", false: "LIVE (files will be deleted)"}[dryRun])
	fmt.Printf("%s\n", strings.Repeat("-", 80))

	// Afficher la liste des fichiers orphelins
	if verbose {
		fmt.Printf("\n📋 Orphaned files list:\n")
		for i, obj := range orphanedFiles {
			fmt.Printf("%3d. %s (%s)\n", i+1, obj.Key, formatBytes(obj.Size))
		}
		fmt.Println()
	}

	// Demander confirmation si pas en mode dry-run
	if !dryRun {
		fmt.Printf("⚠️  WARNING: This will permanently delete %d orphaned files (%s)\n", len(orphanedFiles), formatBytes(totalOrphanedSize))
		fmt.Printf("Are you sure you want to continue? (yes/no): ")

		var response string
		fmt.Scanln(&response)
		if strings.ToLower(strings.TrimSpace(response)) != "yes" {
			utils.ProgressWarning("Operation cancelled by user")
			return nil
		}
	}

	// Supprimer les fichiers orphelins
	if verbose {
		utils.ProgressStep("Deleting orphaned files...")
	}

	deletedCount := 0
	deletedSize := int64(0)
	errors := make([]string, 0)

	progressBar := utils.NewProgressBar(int64(len(orphanedFiles)))

	for i, obj := range orphanedFiles {
		if verbose {
			utils.ProgressStep(fmt.Sprintf("Deleting %s (%s)...", obj.Key, formatBytes(obj.Size)))
		}

		if !dryRun {
			err := m.storageClient.DeleteObject(obj.Key)
			if err != nil {
				errorMsg := fmt.Sprintf("failed to delete %s: %v", obj.Key, err)
				errors = append(errors, errorMsg)
				if verbose {
					utils.ProgressError(errorMsg)
				}
			} else {
				deletedCount++
				deletedSize += obj.Size
				if verbose {
					utils.ProgressDone(fmt.Sprintf("Deleted %s", obj.Key))
				}
			}
		} else {
			deletedCount++
			deletedSize += obj.Size
		}

		if !verbose {
			progressBar.Update(int64(i + 1))
		}
	}

	if !verbose {
		progressBar.Finish()
	}

	// Afficher le résumé final
	fmt.Printf("\n%s\n", strings.Repeat("=", 80))
	if dryRun {
		fmt.Printf("🧹 CLEAN OPERATION COMPLETED (DRY RUN)\n")
	} else {
		fmt.Printf("🧹 CLEAN OPERATION COMPLETED\n")
	}
	fmt.Printf("%s\n", strings.Repeat("=", 80))
	fmt.Printf("Backup ID: %s\n", backupID)
	fmt.Printf("Files processed: %d\n", len(orphanedFiles))
	fmt.Printf("Files deleted: %d\n", deletedCount)
	fmt.Printf("Size freed: %s\n", formatBytes(deletedSize))

	if len(errors) > 0 {
		fmt.Printf("Errors encountered: %d\n", len(errors))
		for _, err := range errors {
			fmt.Printf("  • %s\n", err)
		}
	}

	if dryRun {
		utils.ProgressInfo("This was a dry run. No files were actually deleted.")
	} else {
		utils.ProgressSuccess("Orphaned files cleanup completed successfully!")
	}

	return nil
}

// CleanAllBackups nettoie toutes les sauvegardes et supprime celles sans index
func (m *Manager) CleanAllBackups(dryRun, verbose, removeOrphaned bool) error {
	if verbose {
		utils.ProgressStep("Scanning all backups...")
	}

	// Lister toutes les sauvegardes disponibles
	backups, err := m.listIndexes()
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}

	if verbose {
		utils.ProgressDone(fmt.Sprintf("Found %d backups with indexes", len(backups)))
	}

	// Initialiser le client de stockage si nécessaire
	if m.storageClient == nil {
		if m.config == nil {
			config, err := utils.LoadConfig(m.configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			m.config = config
		}

		storageClient, err := storage.NewStorageClient(m.config)
		if err != nil {
			return fmt.Errorf("failed to initialize storage client: %w", err)
		}
		m.storageClient = storageClient
	}

	// Lister tous les objets sur le stockage
	if verbose {
		utils.ProgressStep("Scanning storage for all objects...")
	}

	// Analyser la structure du stockage pour trouver les fichiers de sauvegarde
	var allObjects []storage.ObjectInfo
	var storagePrefix string

	// Approche ultra-robuste : utiliser le scan global comme ScanAllObjects
	globalObjects, err := m.storageClient.ListObjects("")
	if err == nil && len(globalObjects) > 0 {
		allObjects = globalObjects
		storagePrefix = "global"
		if verbose {
			utils.ProgressDone(fmt.Sprintf("Found %d objects in global scan", len(globalObjects)))
		}
	} else {
		// Fallback : essayer des préfixes spécifiques
		if verbose {
			utils.ProgressStep("Global scan failed, trying specific prefixes...")
		}

		// Essayer différents préfixes pour voir ce qui existe
		prefixes := []string{"data/", "indexes/", "backups/", "files/", "storage/"}

		for _, prefix := range prefixes {
			objects, err := m.storageClient.ListObjects(prefix)
			if err == nil && len(objects) > 0 {
				allObjects = append(allObjects, objects...)
				if verbose {
					utils.ProgressDone(fmt.Sprintf("Found %d objects in %s prefix", len(objects), prefix))
				}
			}
		}

		// Si on n'a trouvé aucun objet, essayer de comprendre pourquoi
		if len(allObjects) == 0 {
			if verbose {
				utils.ProgressWarning("No objects found in any storage prefix")
				utils.ProgressStep("This might indicate:")
				utils.ProgressStep("  - Files are stored with a different prefix")
				utils.ProgressStep("  - Storage bucket is empty")
				utils.ProgressStep("  - Access permissions issue")
			}

			// Essayer de lister avec différents préfixes pour diagnostiquer
			testPrefixes := []string{"data/", "files/", "storage/", "backup/", "snapshots/"}
			for _, prefix := range testPrefixes {
				testObjects, err := m.storageClient.ListObjects(prefix)
				if err == nil && len(testObjects) > 0 {
					if verbose {
						utils.ProgressDone(fmt.Sprintf("Found %d objects in %s prefix", len(testObjects), prefix))
					}
					allObjects = testObjects
					storagePrefix = prefix
					break
				}
			}
		}
	}

	// Approche 3: Recherche spécifique pour les répertoires test* dans data/
	if verbose {
		utils.ProgressStep("Searching specifically for test* directories...")
	}

	// Essayer des préfixes de plus en plus spécifiques pour test-20250810-214443
	testPrefixes := []string{
		"data/test",
		"data/test-",
		"data/test-2025",
		"data/test-20250810",
		"data/test-20250810-",
		"data/test-20250810-21",
		"data/test-20250810-214",
		"data/test-20250810-2144",
		"data/test-20250810-21444",
		"data/test-20250810-214443",
	}

	for _, testPrefix := range testPrefixes {
		testObjects, err := m.storageClient.ListObjects(testPrefix)
		if err == nil && len(testObjects) > 0 {
			allObjects = append(allObjects, testObjects...)
			if verbose {
				utils.ProgressDone(fmt.Sprintf("Found %d objects in %s prefix", len(testObjects), testPrefix))
			}
		}
	}

	if verbose {
		utils.ProgressDone(fmt.Sprintf("Using storage prefix: %s", storagePrefix))

		// Afficher quelques exemples d'objets pour le débogage
		if len(allObjects) > 0 {
			utils.ProgressStep("Sample objects found:")
			maxSamples := 5
			if len(allObjects) < maxSamples {
				maxSamples = len(allObjects)
			}
			for i := 0; i < maxSamples; i++ {
				utils.ProgressDone(fmt.Sprintf("  %s (%s)", allObjects[i].Key, formatBytes(allObjects[i].Size)))
			}
			if len(allObjects) > maxSamples {
				utils.ProgressDone(fmt.Sprintf("  ... and %d more objects", len(allObjects)-maxSamples))
			}
		}
	}

	// Créer un ensemble des clés valides depuis tous les index
	validKeys := make(map[string]bool)
	validBackupIDs := make(map[string]bool)
	storageKeyPatterns := make(map[string]int)

	if verbose {
		utils.ProgressStep("Analyzing backup indexes for storage patterns...")
	}

	for _, backup := range backups {
		validBackupIDs[backup.BackupID] = true

		// Charger l'index pour obtenir les clés de stockage
		index, err := m.LoadIndex(backup.BackupID)
		if err != nil {
			if verbose {
				utils.ProgressWarning(fmt.Sprintf("Failed to load index for %s: %v", backup.BackupID, err))
			}
			continue
		}

		for _, file := range index.Files {
			if !file.IsDirectory {
				// Construire la clé de stockage complète avec le préfixe data/backupID/
				fullStorageKey := fmt.Sprintf("data/%s/%s", backup.BackupID, file.StorageKey)
				validKeys[fullStorageKey] = true

				// Si c'est un gros fichier, ajouter aussi tous ses chunks comme valides
				if file.Size > 100*1024*1024 { // 100MB - seuil pour les fichiers chunkés
					// Ajouter le fichier de métadonnées
					metadataKey := fmt.Sprintf("%s.metadata", fullStorageKey)
					validKeys[metadataKey] = true

					// Ajouter tous les chunks possibles (on ne connaît pas le nombre exact)
					// On va ajouter jusqu'à 1000 chunks pour couvrir la plupart des cas
					for chunkNum := 0; chunkNum < 1000; chunkNum++ {
						chunkKey := fmt.Sprintf("%s.chunk.%03d", fullStorageKey, chunkNum)
						validKeys[chunkKey] = true
					}
				}

				// Analyser le pattern de la clé de stockage
				parts := strings.Split(file.StorageKey, "/")
				if len(parts) > 0 {
					pattern := parts[0]
					storageKeyPatterns[pattern]++
				}
			}
		}
	}

	if verbose {
		utils.ProgressDone(fmt.Sprintf("Found %d valid storage keys", len(validKeys)))
		if len(storageKeyPatterns) > 0 {
			utils.ProgressStep("Storage key patterns found:")
			for pattern, count := range storageKeyPatterns {
				utils.ProgressDone(fmt.Sprintf("  %s: %d files", pattern, count))
			}
		}
	}

	// Identifier les fichiers orphelins et les sauvegardes sans index
	var orphanedFiles []storage.ObjectInfo
	var orphanedBackups []string
	orphanedBackupObjects := make(map[string][]storage.ObjectInfo)

	for _, obj := range allObjects {
		// Ignorer les fichiers d'index et de métadonnées
		if strings.HasSuffix(obj.Key, ".index") || strings.HasSuffix(obj.Key, ".metadata") ||
			strings.HasSuffix(obj.Key, ".json") || strings.Contains(obj.Key, "indexes/") {
			continue
		}

		// Identifier l'ID de sauvegarde selon la structure
		var backupID string
		parts := strings.Split(obj.Key, "/")

		if storagePrefix == "data/" && len(parts) >= 2 {
			// Structure: data/backup-id/file
			backupID = parts[1]
		} else if storagePrefix == "backups/" && len(parts) >= 2 {
			// Structure: backups/backup-id/file
			backupID = parts[1]
		} else if len(parts) >= 1 {
			// Structure: backup-id/file ou file
			if strings.Contains(parts[0], "-") {
				// Essayer d'extraire l'ID depuis le nom de fichier
				backupID = parts[0]
			} else if len(parts) >= 2 {
				backupID = parts[1]
			}
		}

		// Si on n'a pas pu identifier l'ID, ignorer
		if backupID == "" {
			if verbose {
				utils.ProgressWarning(fmt.Sprintf("Could not identify backup ID for: %s", obj.Key))
			}
			continue
		}

		// Vérifier si la clé existe dans un index valide
		if !validKeys[obj.Key] {
			orphanedFiles = append(orphanedFiles, obj)
		}

		// Vérifier si la sauvegarde a un index
		if !validBackupIDs[backupID] {
			if orphanedBackupObjects[backupID] == nil {
				orphanedBackupObjects[backupID] = []storage.ObjectInfo{}
				orphanedBackups = append(orphanedBackups, backupID)
			}
			orphanedBackupObjects[backupID] = append(orphanedBackupObjects[backupID], obj)
		}
	}

	// Afficher le résumé
	totalOrphanedSize := int64(0)
	totalOrphanedBackupSize := int64(0)

	for _, obj := range orphanedFiles {
		totalOrphanedSize += obj.Size
	}

	for _, backupID := range orphanedBackups {
		for _, obj := range orphanedBackupObjects[backupID] {
			totalOrphanedBackupSize += obj.Size
		}
	}

	if verbose {
		utils.ProgressDone(fmt.Sprintf("Found %d orphaned files (%s) and %d orphaned backups (%s)",
			len(orphanedFiles), formatBytes(totalOrphanedSize),
			len(orphanedBackups), formatBytes(totalOrphanedBackupSize)))
	}

	// Afficher le rapport détaillé
	fmt.Printf("\n🔍 CLEANUP ANALYSIS REPORT\n")
	fmt.Printf("%s\n", strings.Repeat("=", 80))
	fmt.Printf("Backups with indexes: %d\n", len(backups))
	fmt.Printf("Orphaned files: %d (%s)\n", len(orphanedFiles), formatBytes(totalOrphanedSize))
	fmt.Printf("Orphaned backups: %d (%s)\n", len(orphanedBackups), formatBytes(totalOrphanedBackupSize))
	fmt.Printf("Total space to reclaim: %s\n", formatBytes(totalOrphanedSize+totalOrphanedBackupSize))
	fmt.Printf("Mode: %s\n", map[bool]string{true: "DRY RUN (no files will be deleted)", false: "LIVE (files will be deleted)"}[dryRun])
	fmt.Printf("%s\n", strings.Repeat("=", 80))

	// Afficher les sauvegardes orphelines
	if len(orphanedBackups) > 0 {
		fmt.Printf("\n📋 Orphaned backups (no index found):\n")
		for i, backupID := range orphanedBackups {
			backupSize := int64(0)
			for _, obj := range orphanedBackupObjects[backupID] {
				backupSize += obj.Size
			}
			fmt.Printf("%3d. %s (%s, %d files)\n", i+1, backupID, formatBytes(backupSize), len(orphanedBackupObjects[backupID]))
		}
	}

	// Afficher les fichiers orphelins
	if len(orphanedFiles) > 0 {
		fmt.Printf("\n📋 Orphaned files (not in any index):\n")
		for i, obj := range orphanedFiles {
			fmt.Printf("%3d. %s (%s)\n", i+1, obj.Key, formatBytes(obj.Size))
		}
	}

	// Si aucun fichier à nettoyer
	if len(orphanedFiles) == 0 && len(orphanedBackups) == 0 {
		utils.ProgressSuccess("No cleanup needed. All storage is consistent!")
		return nil
	}

	// Demander confirmation si pas en mode dry-run
	if !dryRun {
		fmt.Printf("\n⚠️  WARNING: This will permanently delete:\n")
		if len(orphanedFiles) > 0 {
			fmt.Printf("  • %d orphaned files (%s)\n", len(orphanedFiles), formatBytes(totalOrphanedSize))
		}
		if len(orphanedBackups) > 0 && removeOrphaned {
			fmt.Printf("  • %d orphaned backups (%s)\n", len(orphanedBackups), formatBytes(totalOrphanedBackupSize))
		}
		fmt.Printf("Total space to reclaim: %s\n", formatBytes(totalOrphanedSize+totalOrphanedBackupSize))
		fmt.Printf("Are you sure you want to continue? (yes/no): ")

		var response string
		fmt.Scanln(&response)
		if strings.ToLower(strings.TrimSpace(response)) != "yes" {
			utils.ProgressWarning("Operation cancelled by user")
			return nil
		}
	}

	// Procéder au nettoyage
	if verbose {
		utils.ProgressStep("Starting cleanup process...")
	}

	deletedFiles := 0
	deletedSize := int64(0)
	deletedBackups := 0
	deletedBackupSize := int64(0)
	errors := make([]string, 0)

	// Supprimer les fichiers orphelins
	if len(orphanedFiles) > 0 {
		if verbose {
			utils.ProgressStep("Deleting orphaned files...")
		}

		progressBar := utils.NewProgressBar(int64(len(orphanedFiles)))

		for i, obj := range orphanedFiles {
			if verbose {
				utils.ProgressStep(fmt.Sprintf("Deleting %s (%s)...", obj.Key, formatBytes(obj.Size)))
			}

			if !dryRun {
				err := m.storageClient.DeleteObject(obj.Key)
				if err != nil {
					errorMsg := fmt.Sprintf("failed to delete %s: %v", obj.Key, err)
					errors = append(errors, errorMsg)
					if verbose {
						utils.ProgressError(errorMsg)
					}
				} else {
					deletedFiles++
					deletedSize += obj.Size
					if verbose {
						utils.ProgressDone(fmt.Sprintf("Deleted %s", obj.Key))
					}
				}
			} else {
				deletedFiles++
				deletedSize += obj.Size
			}

			if !verbose {
				progressBar.Update(int64(i + 1))
			}
		}

		if !verbose {
			progressBar.Finish()
		}
	}

	// Supprimer les sauvegardes orphelines si demandé
	if len(orphanedBackups) > 0 && removeOrphaned {
		if verbose {
			utils.ProgressStep("Deleting orphaned backups...")
		}

		progressBar := utils.NewProgressBar(int64(len(orphanedBackups)))

		for i, backupID := range orphanedBackups {
			backupObjects := orphanedBackupObjects[backupID]
			if verbose {
				utils.ProgressStep(fmt.Sprintf("Deleting orphaned backup %s (%d files, %s)...",
					backupID, len(backupObjects), formatBytes(totalOrphanedBackupSize)))
			}

			// Supprimer tous les objets de la sauvegarde orpheline
			backupDeleted := 0
			backupDeletedSize := int64(0)

			for _, obj := range backupObjects {
				if !dryRun {
					err := m.storageClient.DeleteObject(obj.Key)
					if err != nil {
						errorMsg := fmt.Sprintf("failed to delete %s: %v", obj.Key, err)
						errors = append(errors, errorMsg)
						if verbose {
							utils.ProgressError(errorMsg)
						}
					} else {
						backupDeleted++
						backupDeletedSize += obj.Size
					}
				} else {
					backupDeleted++
					backupDeletedSize += obj.Size
				}
			}

			if backupDeleted > 0 {
				deletedBackups++
				deletedBackupSize += backupDeletedSize
				if verbose {
					utils.ProgressDone(fmt.Sprintf("Deleted orphaned backup %s (%d files, %s)",
						backupID, backupDeleted, formatBytes(backupDeletedSize)))
				}
			}

			if !verbose {
				progressBar.Update(int64(i + 1))
			}
		}

		if !verbose {
			progressBar.Finish()
		}
	}

	// Afficher le résumé final
	fmt.Printf("\n%s\n", strings.Repeat("=", 80))
	if dryRun {
		fmt.Printf("🧹 COMPLETE CLEANUP ANALYSIS COMPLETED (DRY RUN)\n")
	} else {
		fmt.Printf("🧹 COMPLETE CLEANUP OPERATION COMPLETED\n")
	}
	fmt.Printf("%s\n", strings.Repeat("=", 80))
	fmt.Printf("Files deleted: %d (%s)\n", deletedFiles, formatBytes(deletedSize))
	fmt.Printf("Orphaned backups deleted: %d (%s)\n", deletedBackups, formatBytes(deletedBackupSize))
	fmt.Printf("Total space reclaimed: %s\n", formatBytes(deletedSize+deletedBackupSize))

	if len(errors) > 0 {
		fmt.Printf("Errors encountered: %d\n", len(errors))
		for _, err := range errors {
			fmt.Printf("  • %s\n", err)
		}
	}

	if dryRun {
		utils.ProgressInfo("This was a dry run. No files were actually deleted.")
	} else {
		utils.ProgressSuccess("Complete cleanup operation completed successfully!")
	}

	return nil
}

// ScanAllObjects liste tous les objets dans le stockage pour diagnostic
func (m *Manager) ScanAllObjects(verbose bool) error {
	if verbose {
		utils.ProgressStep("Scanning all objects in storage...")
	}

	// Initialiser le client de stockage si nécessaire
	if m.storageClient == nil {
		if m.config == nil {
			config, err := utils.LoadConfig(m.configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			m.config = config
		}

		storageClient, err := storage.NewStorageClient(m.config)
		if err != nil {
			return fmt.Errorf("failed to initialize storage client: %w", err)
		}
		m.storageClient = storageClient
	}

	// Lister tous les objets dans le stockage
	if verbose {
		utils.ProgressStep("Listing all objects...")
	}

	// Approche 1: Scan global sans préfixe (le plus exhaustif)
	var allObjects []storage.ObjectInfo

	// Essayer d'abord un scan global
	globalObjects, err := m.storageClient.ListObjects("")
	if err == nil && len(globalObjects) > 0 {
		allObjects = append(allObjects, globalObjects...)
		if verbose {
			utils.ProgressDone(fmt.Sprintf("Found %d objects in global scan", len(globalObjects)))
		}
	}

	// Approche 2: Si le scan global ne fonctionne pas, essayer des préfixes spécifiques
	if len(allObjects) == 0 {
		if verbose {
			utils.ProgressStep("Global scan failed, trying specific prefixes...")
		}

		// Essayer différents préfixes pour voir ce qui existe
		prefixes := []string{"", "data/", "indexes/", "backups/", "files/", "storage/"}

		for _, prefix := range prefixes {
			objects, err := m.storageClient.ListObjects(prefix)
			if err == nil && len(objects) > 0 {
				allObjects = append(allObjects, objects...)
				if verbose {
					utils.ProgressDone(fmt.Sprintf("Found %d objects in %s prefix", len(objects), prefix))
				}
			}
		}
	}

	// Approche 3: Recherche spécifique pour les répertoires test* dans data/
	if verbose {
		utils.ProgressStep("Searching specifically for test* directories...")
	}

	// Essayer des préfixes de plus en plus spécifiques pour test-20250810-214443
	testPrefixes := []string{
		"data/test",
		"data/test-",
		"data/test-2025",
		"data/test-20250810",
		"data/test-20250810-",
		"data/test-20250810-21",
		"data/test-20250810-214",
		"data/test-20250810-2144",
		"data/test-20250810-21444",
		"data/test-20250810-214443",
	}

	for _, testPrefix := range testPrefixes {
		testObjects, err := m.storageClient.ListObjects(testPrefix)
		if err == nil && len(testObjects) > 0 {
			allObjects = append(allObjects, testObjects...)
			if verbose {
				utils.ProgressDone(fmt.Sprintf("Found %d objects in %s prefix", len(testObjects), testPrefix))
			}
		}
	}

	// Approche 4: Essayer de lister tous les objets avec des préfixes de caractères
	if verbose {
		utils.ProgressStep("Trying character-based prefixes...")
	}

	// Essayer de lister avec des préfixes de caractères pour couvrir tous les cas
	charPrefixes := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z"}
	for _, char := range charPrefixes {
		charObjects, err := m.storageClient.ListObjects(char)
		if err == nil && len(charObjects) > 0 {
			allObjects = append(allObjects, charObjects...)
			if verbose {
				utils.ProgressDone(fmt.Sprintf("Found %d objects with prefix %s", len(charObjects), char))
			}
		}
	}

	if len(allObjects) == 0 {
		utils.Info("No objects found in storage")
		return nil
	}

	// Dédupliquer les objets (au cas où plusieurs préfixes retourneraient les mêmes objets)
	objectMap := make(map[string]storage.ObjectInfo)
	for _, obj := range allObjects {
		objectMap[obj.Key] = obj
	}

	// Convertir la map en slice
	uniqueObjects := make([]storage.ObjectInfo, 0, len(objectMap))
	for _, obj := range objectMap {
		uniqueObjects = append(uniqueObjects, obj)
	}
	allObjects = uniqueObjects

	// Afficher le résumé
	fmt.Printf("\n📊 Storage Scan Results:\n")
	fmt.Printf("Found %d total unique objects\n", len(allObjects))
	fmt.Printf("Scan methods used: Global scan + specific prefixes + test* search + character prefixes\n\n")

	// Grouper les objets par préfixe pour une meilleure lisibilité
	objectsByPrefix := make(map[string][]storage.ObjectInfo)
	for _, obj := range allObjects {
		// Extraire le préfixe principal (premier niveau)
		parts := strings.Split(obj.Key, "/")
		if len(parts) > 0 {
			prefix := parts[0] + "/"
			objectsByPrefix[prefix] = append(objectsByPrefix[prefix], obj)
		} else {
			objectsByPrefix["root"] = append(objectsByPrefix["root"], obj)
		}
	}

	// Afficher les objets par préfixe
	for prefix, objects := range objectsByPrefix {
		if len(objects) == 0 {
			continue
		}

		fmt.Printf("📁 %s (%d objects):\n", prefix, len(objects))

		// Trier par taille (plus gros en premier)
		sort.Slice(objects, func(i, j int) bool {
			return objects[i].Size > objects[j].Size
		})

		// Afficher les premiers objets (max 15 par préfixe pour plus de visibilité)
		maxDisplay := 15
		if len(objects) < maxDisplay {
			maxDisplay = len(objects)
		}

		for i := 0; i < maxDisplay; i++ {
			obj := objects[i]
			sizeStr := formatBytes(obj.Size)
			fmt.Printf("  %s (%s)\n", obj.Key, sizeStr)
		}

		if len(objects) > maxDisplay {
			fmt.Printf("  ... and %d more objects\n", len(objects)-maxDisplay)
		}
		fmt.Println()
	}

	// Calculer la taille totale
	var totalSize int64
	for _, obj := range allObjects {
		totalSize += obj.Size
	}

	fmt.Printf("💾 Total storage used: %s\n", formatBytes(totalSize))

	// Recherche spécifique pour test-20250810-214443
	fmt.Printf("\n🔍 Specific search for test-20250810-214443:\n")
	found := false
	for _, obj := range allObjects {
		if strings.Contains(obj.Key, "test-20250810-214443") {
			fmt.Printf("✅ FOUND: %s (%s)\n", obj.Key, formatBytes(obj.Size))
			found = true
		}
	}

	if !found {
		fmt.Printf("❌ NOT FOUND: No objects containing 'test-20250810-214443' were found\n")
		fmt.Printf("   This suggests the directory may have been removed or is in a different location\n")
	}

	return nil
}

// formatBytes formate les bytes en unités lisibles
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

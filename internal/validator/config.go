package validator

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"bcrdf/internal/crypto"
	"bcrdf/pkg/storage"
	"bcrdf/pkg/utils"
)

// ConfigValidator valide une configuration BCRDF
type ConfigValidator struct {
	config *utils.Config
}

// NewConfigValidator crée un nouveau validateur
func NewConfigValidator(config *utils.Config) *ConfigValidator {
	return &ConfigValidator{config: config}
}

// ValidateAll valide tous les aspects de la configuration
func (v *ConfigValidator) ValidateAll(verbose bool) error {
	if !verbose {
		utils.ProgressStep("Validating configuration...")
	} else {
		utils.Info("🔍 Starting configuration validation")
	}

	// Validation du stockage
	if err := v.validateStorage(verbose); err != nil {
		return fmt.Errorf("erreur de validation du stockage: %w", err)
	}

	// Validation de la sauvegarde
	if err := v.validateBackup(verbose); err != nil {
		return fmt.Errorf("erreur de validation de la sauvegarde: %w", err)
	}

	// Testing storage connectivity
	if err := v.testStorageConnectivity(verbose); err != nil {
		return fmt.Errorf("storage connectivity error: %w", err)
	}

	if verbose {
		utils.Info("✅ Validation completed successfully")
	} else {
		utils.ProgressSuccess("Configuration validated successfully")
	}

	return nil
}

// validateStorage valide les paramètres de stockage
func (v *ConfigValidator) validateStorage(verbose bool) error {
	if verbose {
		utils.Info("Validating storage parameters...")
	}

	storageConfig := v.config.Storage

	// Vérifier le type de stockage
	switch storageConfig.Type {
	case "s3":
		return v.validateS3Storage(storageConfig, verbose)
	case "webdav":
		return v.validateWebDAVStorage(storageConfig, verbose)
	default:
		return fmt.Errorf("unsupported storage type: %s", storageConfig.Type)
	}
}

// validateS3Storage valide les paramètres S3
func (v *ConfigValidator) validateS3Storage(storageConfig struct {
	Type      string `mapstructure:"type"`
	Bucket    string `mapstructure:"bucket"`
	Region    string `mapstructure:"region"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	Endpoint  string `mapstructure:"endpoint"`
	Username  string `mapstructure:"username"`
	Password  string `mapstructure:"password"`
}, verbose bool) error {
	// Vérifier le bucket
	if storageConfig.Bucket == "" {
		return fmt.Errorf("nom du bucket requis pour S3")
	}

	// Vérifier la région
	if storageConfig.Region == "" {
		return fmt.Errorf("region required for S3")
	}

	// Vérifier l'endpoint
	if storageConfig.Endpoint != "" {
		_, err := url.Parse(storageConfig.Endpoint)
		if err != nil {
			return fmt.Errorf("endpoint S3 invalide: %w", err)
		}
	}

	// Vérifier les clés d'accès
	if storageConfig.AccessKey == "" {
		return fmt.Errorf("access key required for S3")
	}

	if storageConfig.SecretKey == "" {
		return fmt.Errorf("secret key required for S3")
	}

	if verbose {
		utils.Info("✅ Parameters valid")
	}

	return nil
}

// validateWebDAVStorage valide les paramètres WebDAV
func (v *ConfigValidator) validateWebDAVStorage(storageConfig struct {
	Type      string `mapstructure:"type"`
	Bucket    string `mapstructure:"bucket"`
	Region    string `mapstructure:"region"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	Endpoint  string `mapstructure:"endpoint"`
	Username  string `mapstructure:"username"`
	Password  string `mapstructure:"password"`
}, verbose bool) error {
	// Vérifier l'endpoint
	if storageConfig.Endpoint == "" {
		return fmt.Errorf("URL du serveur WebDAV requise")
	}

	_, err := url.Parse(storageConfig.Endpoint)
	if err != nil {
		return fmt.Errorf("URL WebDAV invalide: %w", err)
	}

	// Vérifier les identifiants
	if storageConfig.Username == "" {
		return fmt.Errorf("nom d'utilisateur requis pour WebDAV")
	}

	if storageConfig.Password == "" {
		return fmt.Errorf("mot de passe requis pour WebDAV")
	}

	if verbose {
		utils.Info("✅ Parameters valid")
	}

	return nil
}

// validateBackup valide les paramètres de sauvegarde
func (v *ConfigValidator) validateBackup(verbose bool) error {
	if verbose {
		utils.Info("Validating backup parameters...")
	}

	backup := v.config.Backup

	// Vérifier la clé de chiffrement
	if backup.EncryptionKey == "" {
		return fmt.Errorf("encryption key required")
	}

	// Valider la clé selon l'algorithme
	algorithm := crypto.EncryptionAlgorithm(backup.EncryptionAlgo)
	if algorithm == "" {
		algorithm = crypto.AES256GCM // Par défaut
	}

	if err := crypto.ValidateKeyV2(backup.EncryptionKey, algorithm); err != nil {
		return fmt.Errorf("invalid encryption key: %w", err)
	}

	// Vérifier l'algorithme de chiffrement
	switch algorithm {
	case crypto.AES256GCM, crypto.XChaCha20Poly1305:
		// OK
	default:
		return fmt.Errorf("unsupported encryption algorithm: %s", backup.EncryptionAlgo)
	}

	// Vérifier le niveau de compression
	if backup.CompressionLevel < 1 || backup.CompressionLevel > 9 {
		return fmt.Errorf("invalid compression level (1-9): %d", backup.CompressionLevel)
	}

	// Vérifier le nombre de workers
	if backup.MaxWorkers < 1 || backup.MaxWorkers > 100 {
		return fmt.Errorf("nombre de workers invalide (1-100): %d", backup.MaxWorkers)
	}

	if verbose {
		utils.Info("✅ Parameters valid")
	}

	return nil
}

// testStorageConnectivity teste la connectivité du stockage
func (v *ConfigValidator) testStorageConnectivity(verbose bool) error {
	if verbose {
		utils.Info("Testing storage connectivity...")
	}

	// Créer le client de stockage
	storageClient, err := storage.NewStorageClient(v.config)
	if err != nil {
		return fmt.Errorf("error creating storage client: %w", err)
	}

	// Tester la connectivité
	if err := storageClient.TestConnectivity(); err != nil {
		return fmt.Errorf("impossible de se connecter au stockage: %w", err)
	}

	// Tester la liste d'objets
	objects, err := storageClient.ListObjects("test/")
	if err != nil {
		return fmt.Errorf("impossible de lister les objets: %w", err)
	}

	if verbose {
		utils.Info("✅ Connectivity successful (%d objects found with prefix 'test/')", len(objects))
	}

	// Tester les permissions en créant un fichier test
	testKey := "bcrdf-test-connectivity"
	testData := []byte("test de connectivité BCRDF")

	if err := storageClient.Upload(testKey, testData); err != nil {
		return fmt.Errorf("unable to write to storage: %w", err)
	}

	// Vérifier qu'on peut le lire
	if _, err := storageClient.Download(testKey); err != nil {
		return fmt.Errorf("impossible de lire depuis le stockage: %w", err)
	}

	// Nettoyer le fichier test
	if err := storageClient.DeleteObject(testKey); err != nil {
		if verbose {
			utils.Warn("Impossible de supprimer le fichier test: %v", err)
		}
	}

	if verbose {
		utils.Info("✅ Storage permissions validated")
	}

	return nil
}

// GenerateConfig génère une configuration par défaut
func GenerateConfig(outputPath string) error {
	return GenerateConfigWithType(outputPath, "s3")
}

// GenerateConfigWithType génère une configuration pour un type de stockage spécifique
func GenerateConfigWithType(outputPath, storageType string) error {
	// Générer une clé de chiffrement sécurisée
	encryptionKey, err := crypto.GenerateKeyV2(crypto.AES256GCM)
	if err != nil {
		return fmt.Errorf("error generating key: %w", err)
	}

	// Configuration par défaut
	config := &utils.Config{}

	config.Storage.Type = storageType

	switch storageType {
	case "s3":
		config.Storage.Bucket = "my-backup-bucket"
		config.Storage.Region = "eu-west-3"
		config.Storage.Endpoint = "https://s3.eu-west-3.amazonaws.com"
		config.Storage.AccessKey = "YOUR_ACCESS_KEY"
		config.Storage.SecretKey = "YOUR_SECRET_KEY"
	case "webdav":
		config.Storage.Endpoint = "https://your-server.com/remote.php/dav/files/username/"
		config.Storage.Username = "YOUR_USERNAME"
		config.Storage.Password = "YOUR_PASSWORD"
	default:
		return fmt.Errorf("unsupported storage type: %s", storageType)
	}

	config.Backup.EncryptionKey = hex.EncodeToString([]byte(encryptionKey))
	config.Backup.EncryptionAlgo = "aes-256-gcm"
	config.Backup.CompressionLevel = 3
	config.Backup.MaxWorkers = 32 // Optimized for performance
	config.Backup.ChecksumMode = "fast"
	config.Backup.BufferSize = "64MB"
	config.Backup.BatchSize = 50
	config.Backup.BatchSizeLimit = "10MB"
	config.Backup.SkipPatterns = []string{
		"*.tmp",
		"*.cache",
		"*.log",
		".DS_Store",
		"Thumbs.db",
		"*.swp",
		"*.swo",
		"node_modules/",
		".git/",
		"__pycache__/",
	}

	config.Retention.Days = 30
	config.Retention.MaxBackups = 10

	// Créer le répertoire parent si nécessaire
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("error creating directory: %w", err)
	}

	// Sauvegarder la configuration
	if err := utils.WriteConfig(config, outputPath); err != nil {
		return fmt.Errorf("error saving: %w", err)
	}

	return nil
}

// GenerateInteractiveConfig génère une configuration via un mode interactif
func GenerateInteractiveConfig(outputPath string) error {
	utils.PrintHeader("BCRDF Interactive Configuration")

	fmt.Println("Welcome to BCRDF! This wizard will help you create an optimized configuration.")
	fmt.Println("Press Enter to use default values shown in brackets.")

	config := &utils.Config{}

	// Configuration du stockage
	if err := configureStorageInteractive(config); err != nil {
		return err
	}

	// Configuration de la sauvegarde
	if err := configureBackupInteractive(config); err != nil {
		return err
	}

	// Configuration de la rétention
	if err := configureRetentionInteractive(config); err != nil {
		return err
	}

	// Créer le répertoire parent si nécessaire
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("error creating directory: %w", err)
	}

	// Sauvegarder la configuration
	if err := utils.WriteConfig(config, outputPath); err != nil {
		return fmt.Errorf("error saving configuration: %w", err)
	}

	utils.PrintSuccess(fmt.Sprintf("Configuration saved to: %s", outputPath))
	utils.PrintInfo("You can now test your configuration with: bcrdf init --test --config " + outputPath)

	return nil
}

// configureStorageInteractive configure le stockage de manière interactive
func configureStorageInteractive(config *utils.Config) error {
	utils.PrintSection("Storage Configuration")

	// Choix du type de stockage
	storageTypes := []string{
		"S3 (Amazon S3, Scaleway, DigitalOcean Spaces, MinIO, etc.)",
		"WebDAV (Nextcloud, ownCloud, Hetzner Storage Box, etc.)",
	}

	choice := utils.PromptChoice("Select your storage type:", storageTypes, 0)

	switch choice {
	case 0:
		config.Storage.Type = "s3"
		return configureS3Interactive(config)
	case 1:
		config.Storage.Type = "webdav"
		return configureWebDAVInteractive(config)
	}

	return nil
}

// configureS3Interactive configure S3 de manière interactive
func configureS3Interactive(config *utils.Config) error {
	utils.PrintInfo("Configuring S3-compatible storage...")

	// Presets pour les services populaires
	presets := []string{
		"Amazon S3 (AWS)",
		"Scaleway Object Storage",
		"DigitalOcean Spaces",
		"MinIO (self-hosted)",
		"Custom S3-compatible service",
	}

	preset := utils.PromptChoice("Choose a preset or custom configuration:", presets, 0)

	switch preset {
	case 0: // AWS S3
		config.Storage.Endpoint = ""
		region := utils.PromptString("AWS Region", "eu-west-3")
		config.Storage.Region = region
		config.Storage.Endpoint = fmt.Sprintf("https://s3.%s.amazonaws.com", region)
	case 1: // Scaleway
		config.Storage.Region = utils.PromptString("Scaleway Region (fr-par, nl-ams, pl-waw)", "fr-par")
		config.Storage.Endpoint = fmt.Sprintf("https://s3.%s.scw.cloud", config.Storage.Region)
	case 2: // DigitalOcean
		config.Storage.Region = utils.PromptString("DigitalOcean Region (nyc3, fra1, sgp1)", "fra1")
		config.Storage.Endpoint = fmt.Sprintf("https://%s.digitaloceanspaces.com", config.Storage.Region)
	case 3: // MinIO
		config.Storage.Endpoint = utils.PromptString("MinIO Server URL", "https://minio.example.com")
		config.Storage.Region = utils.PromptString("Region", "us-east-1")
	case 4: // Custom
		config.Storage.Endpoint = utils.PromptString("S3 Endpoint URL", "https://s3.example.com")
		config.Storage.Region = utils.PromptString("Region", "us-east-1")
	}

	config.Storage.Bucket = utils.PromptString("Bucket name", "my-backup-bucket")
	config.Storage.AccessKey = utils.PromptString("Access Key", "")
	config.Storage.SecretKey = utils.PromptPassword("Secret Key")

	return nil
}

// configureWebDAVInteractive configure WebDAV de manière interactive
func configureWebDAVInteractive(config *utils.Config) error {
	utils.PrintInfo("Configuring WebDAV storage...")

	// Presets pour les services populaires
	presets := []string{
		"Nextcloud",
		"ownCloud",
		"Hetzner Storage Box",
		"Custom WebDAV server",
	}

	preset := utils.PromptChoice("Choose a preset or custom configuration:", presets, 0)

	switch preset {
	case 0: // Nextcloud
		server := utils.PromptString("Nextcloud server URL (e.g., https://cloud.example.com)", "")
		username := utils.PromptString("Username", "")
		config.Storage.Endpoint = fmt.Sprintf("%s/remote.php/dav/files/%s/", strings.TrimSuffix(server, "/"), username)
	case 1: // ownCloud
		server := utils.PromptString("ownCloud server URL (e.g., https://cloud.example.com)", "")
		config.Storage.Endpoint = fmt.Sprintf("%s/remote.php/webdav/", strings.TrimSuffix(server, "/"))
	case 2: // Hetzner
		username := utils.PromptString("Hetzner Storage Box username (e.g., u123456-sub1)", "")
		config.Storage.Endpoint = fmt.Sprintf("https://%s.your-storagebox.de/", username)
	case 3: // Custom
		config.Storage.Endpoint = utils.PromptString("WebDAV URL", "https://webdav.example.com/")
	}

	config.Storage.Username = utils.PromptString("Username", "")
	config.Storage.Password = utils.PromptPassword("Password")

	return nil
}

// configureBackupInteractive configure la sauvegarde de manière interactive
func configureBackupInteractive(config *utils.Config) error {
	utils.PrintSection("Backup Configuration")

	// Génération automatique de la clé de chiffrement
	if utils.PromptYesNo("Generate a secure encryption key automatically?", true) {
		encryptionKey, err := crypto.GenerateKeyV2(crypto.AES256GCM)
		if err != nil {
			return fmt.Errorf("error generating encryption key: %w", err)
		}
		config.Backup.EncryptionKey = hex.EncodeToString([]byte(encryptionKey))
		utils.PrintSuccess("Encryption key generated automatically")
	} else {
		config.Backup.EncryptionKey = utils.PromptString("Encryption key (64 hex characters)", "")
	}

	// Algorithme de chiffrement
	algorithms := []string{
		"AES-256-GCM (recommended, hardware accelerated)",
		"XChaCha20-Poly1305 (recommended for older hardware)",
	}
	algoChoice := utils.PromptChoice("Choose encryption algorithm:", algorithms, 0)
	if algoChoice == 0 {
		config.Backup.EncryptionAlgo = "aes-256-gcm"
	} else {
		config.Backup.EncryptionAlgo = "xchacha20-poly1305"
	}

	// Performance settings
	utils.PrintInfo("Performance settings (optimized defaults):")

	config.Backup.MaxWorkers = utils.PromptInt("Number of parallel workers", 32, 1, 100)

	checksumModes := []string{
		"fast (recommended - 5x faster, very secure)",
		"full (slowest - maximum security)",
		"metadata (fastest - 10x faster, good security)",
	}
	checksumChoice := utils.PromptChoice("Choose checksum mode:", checksumModes, 0)
	switch checksumChoice {
	case 0:
		config.Backup.ChecksumMode = "fast"
	case 1:
		config.Backup.ChecksumMode = "full"
	case 2:
		config.Backup.ChecksumMode = "metadata"
	}

	config.Backup.CompressionLevel = utils.PromptInt("Compression level (1=fast, 9=best)", 3, 1, 9)
	config.Backup.BufferSize = utils.PromptString("I/O buffer size", "64MB")
	config.Backup.BatchSize = utils.PromptInt("Batch size for small files", 50, 1, 1000)
	config.Backup.BatchSizeLimit = utils.PromptString("Batch size limit", "10MB")

	// Skip patterns
	if utils.PromptYesNo("Use recommended skip patterns (temporary files, caches, etc.)?", true) {
		config.Backup.SkipPatterns = []string{
			"*.tmp", "*.cache", "*.log", ".DS_Store", "Thumbs.db",
			"*.swp", "*.swo", "node_modules/", ".git/", "__pycache__/",
		}
		utils.PrintSuccess("Skip patterns configured")
	}

	return nil
}

// configureRetentionInteractive configure la rétention de manière interactive
func configureRetentionInteractive(config *utils.Config) error {
	utils.PrintSection("Retention Configuration")

	config.Retention.Days = utils.PromptInt("Retention period in days", 30, 1, 3650)
	config.Retention.MaxBackups = utils.PromptInt("Maximum number of backups to keep", 10, 1, 1000)

	return nil
}

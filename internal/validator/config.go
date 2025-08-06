package validator

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

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
		return fmt.Errorf("erreur de connectivité du stockage: %w", err)
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
		return fmt.Errorf("type de stockage non supporté: %s", storageConfig.Type)
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
		return fmt.Errorf("région requise pour S3")
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
		return fmt.Errorf("clé d'accès requise pour S3")
	}

	if storageConfig.SecretKey == "" {
		return fmt.Errorf("clé secrète requise pour S3")
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

	// Vérifier le chemin source (si spécifié)
	if backup.SourcePath != "" && backup.SourcePath != "/path/to/backup" && backup.SourcePath != "/chemin/vers/sauvegarde" {
		if _, err := os.Stat(backup.SourcePath); os.IsNotExist(err) {
			return fmt.Errorf("chemin source inexistant: %s", backup.SourcePath)
		}
	}

	// Vérifier la clé de chiffrement
	if backup.EncryptionKey == "" {
		return fmt.Errorf("clé de chiffrement requise")
	}

	// Valider la clé selon l'algorithme
	algorithm := crypto.EncryptionAlgorithm(backup.EncryptionAlgo)
	if algorithm == "" {
		algorithm = crypto.AES256GCM // Par défaut
	}

	if err := crypto.ValidateKeyV2(backup.EncryptionKey, algorithm); err != nil {
		return fmt.Errorf("clé de chiffrement invalide: %w", err)
	}

	// Vérifier l'algorithme de chiffrement
	switch algorithm {
	case crypto.AES256GCM, crypto.XChaCha20Poly1305:
		// OK
	default:
		return fmt.Errorf("algorithme de chiffrement non supporté: %s", backup.EncryptionAlgo)
	}

	// Vérifier le niveau de compression
	if backup.CompressionLevel < 1 || backup.CompressionLevel > 9 {
		return fmt.Errorf("niveau de compression invalide (1-9): %d", backup.CompressionLevel)
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
		return fmt.Errorf("erreur lors de la création du client de stockage: %w", err)
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
		return fmt.Errorf("impossible d'écrire sur le stockage: %w", err)
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
		return fmt.Errorf("erreur lors de la génération de la clé: %w", err)
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
		return fmt.Errorf("type de stockage non supporté: %s", storageType)
	}

	config.Backup.SourcePath = "/path/to/backup"
	config.Backup.EncryptionKey = hex.EncodeToString([]byte(encryptionKey))
	config.Backup.EncryptionAlgo = "aes-256-gcm"
	config.Backup.CompressionLevel = 3
	config.Backup.MaxWorkers = 10
	config.Backup.ChecksumMode = "fast"

	config.Retention.Days = 30
	config.Retention.MaxBackups = 10

	// Créer le répertoire parent si nécessaire
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("erreur lors de la création du répertoire: %w", err)
	}

	// Sauvegarder la configuration
	if err := utils.WriteConfig(config, outputPath); err != nil {
		return fmt.Errorf("erreur lors de la sauvegarde: %w", err)
	}

	return nil
}

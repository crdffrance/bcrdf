package utils

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// Config représente la configuration de l'application
type Config struct {
	Storage struct {
		Type string `mapstructure:"type"`
		// S3 fields
		Bucket    string `mapstructure:"bucket"`
		Region    string `mapstructure:"region"`
		AccessKey string `mapstructure:"access_key"`
		SecretKey string `mapstructure:"secret_key"`
		// Common fields
		Endpoint string `mapstructure:"endpoint"`
		// WebDAV fields
		Username string `mapstructure:"username"`
		Password string `mapstructure:"password"`
	} `mapstructure:"storage"`

	Backup struct {
		EncryptionKey    string   `mapstructure:"encryption_key"`
		EncryptionAlgo   string   `mapstructure:"encryption_algo"`
		CompressionLevel int      `mapstructure:"compression_level"`
		MaxWorkers       int      `mapstructure:"max_workers"`
		ChecksumMode     string   `mapstructure:"checksum_mode"` // "full", "fast", "metadata"
		SkipPatterns     []string `mapstructure:"skip_patterns"`
		BufferSize       string   `mapstructure:"buffer_size"`
		BatchSize        int      `mapstructure:"batch_size"`       // Number of files to batch together
		BatchSizeLimit   string   `mapstructure:"batch_size_limit"` // Max size for batch upload (e.g., "10MB")
		ChunkSize        string   `mapstructure:"chunk_size"`       // Chunk size for streaming operations
		MemoryLimit      string   `mapstructure:"memory_limit"`     // Memory limit for large files
		NetworkTimeout   int      `mapstructure:"network_timeout"`  // Network timeout in seconds
		RetryAttempts    int      `mapstructure:"retry_attempts"`   // Number of retry attempts
		RetryDelay       int      `mapstructure:"retry_delay"`      // Delay between retries in seconds
	} `mapstructure:"backup"`

	Retention struct {
		Days       int `mapstructure:"days"`
		MaxBackups int `mapstructure:"max_backups"`
	} `mapstructure:"retention"`
}

// LoadConfig charge la configuration depuis un fichier
func LoadConfig(configFile string) (*Config, error) {
	viper.SetConfigFile(configFile)
	viper.SetConfigType("yaml")

	// Valeurs par défaut
	viper.SetDefault("storage.type", "s3")
	viper.SetDefault("storage.region", "us-east-1")
	viper.SetDefault("backup.encryption_algo", "aes-256-gcm")
	viper.SetDefault("backup.compression_level", 3)
	viper.SetDefault("backup.max_workers", 10)
	viper.SetDefault("retention.days", 30)
	viper.SetDefault("retention.max_backups", 10)

	// Lecture du fichier
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Créer un fichier de configuration par défaut
			return createDefaultConfig(configFile)
		}
		return nil, fmt.Errorf("error reading file de configuration: %w", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error decoding configuration: %w", err)
	}

	// Validation de la configuration
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("configuration invalide: %w", err)
	}

	return &config, nil
}

// createDefaultConfig crée un fichier de configuration par défaut
func createDefaultConfig(configFile string) (*Config, error) {
	Debug("Création d'un fichier de configuration par défaut: %s", configFile)

	defaultConfig := `# BCRDF Configuration - Optimized for Performance
storage:
  type: "s3"
  bucket: "my-backup-bucket"
  region: "eu-west-3"
  endpoint: "https://s3.eu-west-3.amazonaws.com"
  access_key: "YOUR_ACCESS_KEY"
  secret_key: "YOUR_SECRET_KEY"

backup:
  encryption_key: "your-encryption-key-here"  # Generate with: bcrdf init --interactive
  encryption_algo: "aes-256-gcm"  # Options: "aes-256-gcm", "xchacha20-poly1305"
  compression_level: 3  # GZIP level (1-9)
  max_workers: 32  # Number of parallel workers (optimized for performance)
  checksum_mode: "fast"  # Options: "full" (slow, secure), "fast" (recommended), "metadata" (fastest)
  buffer_size: "64MB"  # Buffer size for I/O operations
  batch_size: 50  # Number of small files to batch together
  batch_size_limit: "10MB"  # Maximum size for batch uploads
  
  # Advanced performance optimizations
  chunk_size: "64MB"  # Chunk size for streaming compression/decompression
  memory_limit: "512MB"  # Memory limit for processing large files
  network_timeout: 300  # Network timeout in seconds (5 minutes)
  retry_attempts: 3  # Number of retry attempts for failed uploads
  retry_delay: 5  # Delay between retries in seconds
  
  skip_patterns:  # Patterns to skip during backup (performance optimization)
    - "*.tmp"
    - "*.cache"
    - "*.log"
    - ".DS_Store"
    - "Thumbs.db"
    - "*.swp"
    - "*.swo"
    - "node_modules/"
    - ".git/"
    - "__pycache__/"
    - "*.zip"
    - "*.tar.gz"
    - "*.rar"
    - "*.7z"
    - "*.iso"
    - "*.vmdk"
    - "*.vdi"
    - "*.qcow2"
    - "*.raw"

retention:
  days: 30  # Retention period in days
  max_backups: 10  # Maximum number of backups
`

	if err := os.WriteFile(configFile, []byte(defaultConfig), 0600); err != nil {
		return nil, fmt.Errorf("error creating configuration file: %w", err)
	}

	Info("Configuration file created: %s", configFile)
	Warn("Please configure your S3 parameters and encryption key")
	Info("For optimal setup, run: bcrdf init --interactive")

	var config Config
	viper.SetConfigFile(configFile)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading file de configuration: %w", err)
	}

	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error decoding configuration: %w", err)
	}

	return &config, nil
}

// validateConfig valide la configuration de base (validation légère)
func validateConfig(config *Config) error {
	// Validation du type de stockage
	if config.Storage.Type != "s3" && config.Storage.Type != "webdav" {
		return fmt.Errorf("unsupported storage type: %s", config.Storage.Type)
	}

	// Validation spécifique au type de stockage
	switch config.Storage.Type {
	case "s3":
		return validateS3Config(config)
	case "webdav":
		return validateWebDAVConfig(config)
	}

	return nil
}

// validateS3Config valide la configuration S3
func validateS3Config(config *Config) error {
	if config.Storage.Bucket == "" {
		return fmt.Errorf("le nom du bucket S3 est requis")
	}

	if config.Storage.AccessKey == "" {
		// Essayer de récupérer depuis les variables d'environnement
		if accessKey := os.Getenv("AWS_ACCESS_KEY_ID"); accessKey != "" {
			config.Storage.AccessKey = accessKey
		} else {
			return fmt.Errorf("AWS access key is required (AWS_ACCESS_KEY_ID ou access_key dans la config)")
		}
	}

	if config.Storage.SecretKey == "" {
		// Essayer de récupérer depuis les variables d'environnement
		if secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY"); secretKey != "" {
			config.Storage.SecretKey = secretKey
		} else {
			return fmt.Errorf("AWS secret key is required (AWS_SECRET_ACCESS_KEY ou secret_key dans la config)")
		}
	}

	return validateCommonConfig(config)
}

// validateWebDAVConfig valide la configuration WebDAV
func validateWebDAVConfig(config *Config) error {
	if config.Storage.Endpoint == "" {
		return fmt.Errorf("l'URL du serveur WebDAV est requise")
	}

	if config.Storage.Username == "" {
		return fmt.Errorf("le nom d'utilisateur WebDAV est requis")
	}

	if config.Storage.Password == "" {
		return fmt.Errorf("le mot de passe WebDAV est requis")
	}

	return validateCommonConfig(config)
}

// validateCommonConfig valide les paramètres communs
func validateCommonConfig(config *Config) error {
	if config.Backup.EncryptionKey == "" || config.Backup.EncryptionKey == "your-encryption-key-here" {
		return fmt.Errorf("encryption key is required")
	}

	if config.Backup.CompressionLevel < 1 || config.Backup.CompressionLevel > 22 {
		return fmt.Errorf("compression level must be between 1 et 22")
	}

	if config.Backup.MaxWorkers < 1 {
		return fmt.Errorf("number of workers must be greater than 0")
	}

	// Validate new performance optimization fields
	if config.Backup.NetworkTimeout < 30 {
		return fmt.Errorf("network timeout must be at least 30 seconds")
	}

	if config.Backup.RetryAttempts < 0 || config.Backup.RetryAttempts > 10 {
		return fmt.Errorf("retry attempts must be between 0 and 10")
	}

	if config.Backup.RetryDelay < 1 || config.Backup.RetryDelay > 60 {
		return fmt.Errorf("retry delay must be between 1 and 60 seconds")
	}

	return nil
}

// WriteConfig écrit une configuration dans un fichier YAML
func WriteConfig(config *Config, configFile string) error {
	viper.Reset()

	// Configurer viper pour écrire
	viper.SetConfigFile(configFile)
	viper.SetConfigType("yaml")

	// Définir les valeurs de configuration
	viper.Set("storage.type", config.Storage.Type)
	viper.Set("storage.bucket", config.Storage.Bucket)
	viper.Set("storage.region", config.Storage.Region)
	viper.Set("storage.endpoint", config.Storage.Endpoint)
	viper.Set("storage.access_key", config.Storage.AccessKey)
	viper.Set("storage.secret_key", config.Storage.SecretKey)
	viper.Set("storage.username", config.Storage.Username)
	viper.Set("storage.password", config.Storage.Password)

	viper.Set("backup.encryption_key", config.Backup.EncryptionKey)
	viper.Set("backup.encryption_algo", config.Backup.EncryptionAlgo)
	viper.Set("backup.compression_level", config.Backup.CompressionLevel)
	viper.Set("backup.max_workers", config.Backup.MaxWorkers)
	viper.Set("backup.checksum_mode", config.Backup.ChecksumMode)
	viper.Set("backup.buffer_size", config.Backup.BufferSize)
	viper.Set("backup.batch_size", config.Backup.BatchSize)
	viper.Set("backup.batch_size_limit", config.Backup.BatchSizeLimit)
	viper.Set("backup.skip_patterns", config.Backup.SkipPatterns)
	viper.Set("backup.chunk_size", config.Backup.ChunkSize)
	viper.Set("backup.memory_limit", config.Backup.MemoryLimit)
	viper.Set("backup.network_timeout", config.Backup.NetworkTimeout)
	viper.Set("backup.retry_attempts", config.Backup.RetryAttempts)
	viper.Set("backup.retry_delay", config.Backup.RetryDelay)

	viper.Set("retention.days", config.Retention.Days)
	viper.Set("retention.max_backups", config.Retention.MaxBackups)

	// Écrire le fichier
	return viper.WriteConfig()
}

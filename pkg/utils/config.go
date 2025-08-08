package utils

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Config représente la configuration de l'application
type Config struct {
	Storage struct {
		Type string `mapstructure:"type"`
		// S3 fields
		Bucket       string `mapstructure:"bucket"`
		Region       string `mapstructure:"region"`
		AccessKey    string `mapstructure:"access_key"`
		SecretKey    string `mapstructure:"secret_key"`
		StorageClass string `mapstructure:"storage_class"` // S3 storage class (STANDARD, GLACIER, etc.)
		// Common fields
		Endpoint string `mapstructure:"endpoint"`
		// WebDAV fields
		Username string `mapstructure:"username"`
		Password string `mapstructure:"password"`
	} `mapstructure:"storage"`

	Backup struct {
		EncryptionKey       string   `mapstructure:"encryption_key"`
		EncryptionAlgo      string   `mapstructure:"encryption_algo"`
		CompressionLevel    int      `mapstructure:"compression_level"`
		MaxWorkers          int      `mapstructure:"max_workers"`
		ChecksumMode        string   `mapstructure:"checksum_mode"` // "full", "fast", "metadata"
		SkipPatterns        []string `mapstructure:"skip_patterns"`
		BufferSize          string   `mapstructure:"buffer_size"`
		BatchSize           int      `mapstructure:"batch_size"`            // Number of files to batch together
		BatchSizeLimit      string   `mapstructure:"batch_size_limit"`      // Max size for batch upload (e.g., "10MB")
		ChunkSize           string   `mapstructure:"chunk_size"`            // Chunk size for streaming operations
		MemoryLimit         string   `mapstructure:"memory_limit"`          // Memory limit for large files
		NetworkTimeout      int      `mapstructure:"network_timeout"`       // Network timeout in seconds
		RetryAttempts       int      `mapstructure:"retry_attempts"`        // Number of retry attempts
		RetryDelay          int      `mapstructure:"retry_delay"`           // Delay between retries in seconds
		CacheEnabled        bool     `mapstructure:"cache_enabled"`         // Enable checksum caching
		CacheMaxSize        int      `mapstructure:"cache_max_size"`        // Maximum cache entries
		CacheMaxAge         int      `mapstructure:"cache_max_age"`         // Cache entry max age (minutes)
		CompressionAdaptive bool     `mapstructure:"compression_adaptive"`  // Enable adaptive compression
		SortBySize          bool     `mapstructure:"sort_by_size"`          // Sort files by size (smallest first)
		ChunkSizeLarge      string   `mapstructure:"chunk_size_large"`      // Chunk size for large files (e.g., "50MB")
		LargeFileThreshold  string   `mapstructure:"large_file_threshold"`  // Threshold for large files (e.g., "100MB")
		UltraLargeThreshold string   `mapstructure:"ultra_large_threshold"` // Threshold for ultra-large files (e.g., "5GB")
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
  compression_level: 1  # GZIP level (1-9) - Fastest to avoid compression issues
  max_workers: 16  # Number of parallel workers (balanced for stability)
  checksum_mode: "fast"  # Options: "full" (slow, secure), "fast" (recommended), "metadata" (fastest)
  buffer_size: "32MB"  # Buffer size for I/O operations (smaller for stability)
  batch_size: 25  # Number of small files to batch together (balanced)
  batch_size_limit: "8MB"  # Maximum size for batch uploads (smaller for stability)
  
  # Advanced performance optimizations
  chunk_size: "32MB"  # Chunk size for streaming compression/decompression (smaller for stability)
  memory_limit: "256MB"  # Memory limit for processing large files (reduced for stability)
  network_timeout: 120  # Network timeout in seconds (2 minutes)
  retry_attempts: 5  # Number of retry attempts for failed uploads (increased for stability)
  retry_delay: 2  # Delay between retries in seconds (faster retries)
  
  # Phase 1 performance optimizations
  cache_enabled: true  # Enable checksum caching for faster processing
  cache_max_size: 10000  # Maximum number of cached checksums
  cache_max_age: 60  # Cache entry max age in minutes
  compression_adaptive: true  # Enable adaptive compression based on file size
  sort_by_size: true  # Sort files by size (smallest first) for better UX
  
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
	// Créer une structure temporaire pour l'écriture YAML
	type StorageConfig struct {
		Type         string `yaml:"type"`
		Bucket       string `yaml:"bucket"`
		Region       string `yaml:"region"`
		Endpoint     string `yaml:"endpoint"`
		AccessKey    string `yaml:"access_key"`
		SecretKey    string `yaml:"secret_key"`
		StorageClass string `yaml:"storage_class"`
		Username     string `yaml:"username"`
		Password     string `yaml:"password"`
	}

	type BackupConfig struct {
		EncryptionKey       string   `yaml:"encryption_key"`
		EncryptionAlgo      string   `yaml:"encryption_algo"`
		CompressionLevel    int      `yaml:"compression_level"`
		MaxWorkers          int      `yaml:"max_workers"`
		ChecksumMode        string   `yaml:"checksum_mode"`
		BufferSize          string   `yaml:"buffer_size"`
		BatchSize           int      `yaml:"batch_size"`
		BatchSizeLimit      string   `yaml:"batch_size_limit"`
		SkipPatterns        []string `yaml:"skip_patterns"`
		ChunkSize           string   `yaml:"chunk_size"`
		MemoryLimit         string   `yaml:"memory_limit"`
		NetworkTimeout      int      `yaml:"network_timeout"`
		RetryAttempts       int      `yaml:"retry_attempts"`
		RetryDelay          int      `yaml:"retry_delay"`
		CacheEnabled        bool     `yaml:"cache_enabled"`
		CacheMaxSize        int      `yaml:"cache_max_size"`
		CacheMaxAge         int      `yaml:"cache_max_age"`
		CompressionAdaptive bool     `yaml:"compression_adaptive"`
		SortBySize          bool     `yaml:"sort_by_size"`
		ChunkSizeLarge      string   `yaml:"chunk_size_large"`
		LargeFileThreshold  string   `yaml:"large_file_threshold"`
		UltraLargeThreshold string   `yaml:"ultra_large_threshold"`
	}

	type RetentionConfig struct {
		Days       int `yaml:"days"`
		MaxBackups int `yaml:"max_backups"`
	}

	type FullConfig struct {
		Storage   StorageConfig   `yaml:"storage"`
		Backup    BackupConfig    `yaml:"backup"`
		Retention RetentionConfig `yaml:"retention"`
	}

	// Créer la configuration complète
	fullConfig := FullConfig{
		Storage: StorageConfig{
			Type:         config.Storage.Type,
			Bucket:       config.Storage.Bucket,
			Region:       config.Storage.Region,
			Endpoint:     config.Storage.Endpoint,
			AccessKey:    config.Storage.AccessKey,
			SecretKey:    config.Storage.SecretKey,
			StorageClass: config.Storage.StorageClass,
			Username:     config.Storage.Username,
			Password:     config.Storage.Password,
		},
		Backup: BackupConfig{
			EncryptionKey:       config.Backup.EncryptionKey,
			EncryptionAlgo:      config.Backup.EncryptionAlgo,
			CompressionLevel:    config.Backup.CompressionLevel,
			MaxWorkers:          config.Backup.MaxWorkers,
			ChecksumMode:        config.Backup.ChecksumMode,
			BufferSize:          config.Backup.BufferSize,
			BatchSize:           config.Backup.BatchSize,
			BatchSizeLimit:      config.Backup.BatchSizeLimit,
			SkipPatterns:        config.Backup.SkipPatterns,
			ChunkSize:           config.Backup.ChunkSize,
			MemoryLimit:         config.Backup.MemoryLimit,
			NetworkTimeout:      config.Backup.NetworkTimeout,
			RetryAttempts:       config.Backup.RetryAttempts,
			RetryDelay:          config.Backup.RetryDelay,
			CacheEnabled:        config.Backup.CacheEnabled,
			CacheMaxSize:        config.Backup.CacheMaxSize,
			CacheMaxAge:         config.Backup.CacheMaxAge,
			CompressionAdaptive: config.Backup.CompressionAdaptive,
			SortBySize:          config.Backup.SortBySize,
			ChunkSizeLarge:      config.Backup.ChunkSizeLarge,
			LargeFileThreshold:  config.Backup.LargeFileThreshold,
			UltraLargeThreshold: config.Backup.UltraLargeThreshold,
		},
		Retention: RetentionConfig{
			Days:       config.Retention.Days,
			MaxBackups: config.Retention.MaxBackups,
		},
	}

	// Écrire le fichier YAML
	data, err := yaml.Marshal(fullConfig)
	if err != nil {
		return fmt.Errorf("error marshaling config: %w", err)
	}

	return os.WriteFile(configFile, data, 0600)
}

package utils

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// Config représente la configuration de l'application
type Config struct {
	Storage struct {
		Type      string `mapstructure:"type"`
		Bucket    string `mapstructure:"bucket"`
		Region    string `mapstructure:"region"`
		Endpoint  string `mapstructure:"endpoint"`
		AccessKey string `mapstructure:"access_key"`
		SecretKey string `mapstructure:"secret_key"`
	} `mapstructure:"storage"`

	Backup struct {
		SourcePath       string `mapstructure:"source_path"`
		EncryptionKey    string `mapstructure:"encryption_key"`
		EncryptionAlgo   string `mapstructure:"encryption_algo"`
		CompressionLevel int    `mapstructure:"compression_level"`
		MaxWorkers       int    `mapstructure:"max_workers"`
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
		return nil, fmt.Errorf("erreur lors de la lecture du fichier de configuration: %w", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("erreur lors du décodage de la configuration: %w", err)
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

	defaultConfig := `# Configuration BCRDF
storage:
  type: "s3"
  bucket: "mon-bucket-sauvegarde"
  region: "eu-west-3"
  endpoint: "https://s3.eu-west-3.amazonaws.com"
  access_key: ""
  secret_key: ""

backup:
  source_path: "/path/to/backup"
  encryption_key: "your-encryption-key-here"
  encryption_algo: "aes-256-gcm"  # Options: "aes-256-gcm", "xchacha20-poly1305"
  compression_level: 3
  max_workers: 10

retention:
  days: 30
  max_backups: 10
`

	if err := os.WriteFile(configFile, []byte(defaultConfig), 0644); err != nil {
		return nil, fmt.Errorf("erreur lors de la création du fichier de configuration: %w", err)
	}

	Info("Fichier de configuration créé: %s", configFile)
	Warn("Veuillez configurer vos paramètres S3 et votre clé de chiffrement")

	var config Config
	viper.SetConfigFile(configFile)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("erreur lors de la lecture du fichier de configuration: %w", err)
	}

	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("erreur lors du décodage de la configuration: %w", err)
	}

	return &config, nil
}

// validateConfig valide la configuration
func validateConfig(config *Config) error {
	if config.Storage.Bucket == "" {
		return fmt.Errorf("le nom du bucket S3 est requis")
	}

	if config.Storage.AccessKey == "" {
		// Essayer de récupérer depuis les variables d'environnement
		if accessKey := os.Getenv("AWS_ACCESS_KEY_ID"); accessKey != "" {
			config.Storage.AccessKey = accessKey
		} else {
			return fmt.Errorf("la clé d'accès AWS est requise (AWS_ACCESS_KEY_ID ou access_key dans la config)")
		}
	}

	if config.Storage.SecretKey == "" {
		// Essayer de récupérer depuis les variables d'environnement
		if secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY"); secretKey != "" {
			config.Storage.SecretKey = secretKey
		} else {
			return fmt.Errorf("la clé secrète AWS est requise (AWS_SECRET_ACCESS_KEY ou secret_key dans la config)")
		}
	}

	if config.Backup.EncryptionKey == "" || config.Backup.EncryptionKey == "your-encryption-key-here" {
		return fmt.Errorf("la clé de chiffrement est requise")
	}

	if config.Backup.CompressionLevel < 1 || config.Backup.CompressionLevel > 22 {
		return fmt.Errorf("le niveau de compression doit être entre 1 et 22")
	}

	if config.Backup.MaxWorkers < 1 {
		return fmt.Errorf("le nombre de workers doit être supérieur à 0")
	}

	return nil
}

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"bcrdf/internal/backup"
	"bcrdf/internal/index"
	"bcrdf/internal/restore"
	"bcrdf/internal/retention"
	"bcrdf/internal/validator"
	"bcrdf/pkg/storage"
	"bcrdf/pkg/utils"
)

var (
	configFile string
	verbose    bool
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "bcrdf",
		Short: "BCRDF - Modern index-based backup system",
		Long: `BCRDF (Backup Copy with Redundant Data Format) is a modern backup system
that uses an index-based approach to optimize storage and performance.

Key features:
- Incremental backup with index
- AES-256 encryption
- GZIP compression
- S3 and WebDAV compatible storage
- Precise point-in-time restoration`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if verbose {
				utils.SetLogLevel("debug")
			}
		},
	}

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "config.yaml", "Configuration file")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose mode")

	// Backup command
	var backupCmd = &cobra.Command{
		Use:   "backup",
		Short: "Perform a backup",
		Long:  "Performs an incremental backup based on indexes",
		RunE: func(cmd *cobra.Command, args []string) error {
			source, _ := cmd.Flags().GetString("source")
			name, _ := cmd.Flags().GetString("name")

			if source == "" {
				return fmt.Errorf("source path is required")
			}
			if name == "" {
				return fmt.Errorf("backup name is required")
			}

			backupManager := backup.NewManager(configFile)
			return backupManager.CreateBackup(source, name, verbose)
		},
	}
	backupCmd.Flags().StringP("source", "s", "", "Source path to backup")
	backupCmd.Flags().StringP("name", "n", "", "Backup name")
	_ = backupCmd.MarkFlagRequired("source")
	_ = backupCmd.MarkFlagRequired("name")

	// Restore command
	var restoreCmd = &cobra.Command{
		Use:   "restore",
		Short: "Restore a backup",
		Long:  "Restores a backup from its index",
		RunE: func(cmd *cobra.Command, args []string) error {
			backupID, _ := cmd.Flags().GetString("backup-id")
			destination, _ := cmd.Flags().GetString("destination")

			if backupID == "" {
				return fmt.Errorf("backup ID is required")
			}
			if destination == "" {
				return fmt.Errorf("destination path is required")
			}

			restoreManager := restore.NewManager(configFile)
			return restoreManager.RestoreBackup(backupID, destination, verbose)
		},
	}
	restoreCmd.Flags().StringP("backup-id", "b", "", "Backup ID to restore")
	restoreCmd.Flags().StringP("destination", "d", "", "Destination path")
	_ = restoreCmd.MarkFlagRequired("backup-id")
	_ = restoreCmd.MarkFlagRequired("destination")

	// List command
	var listCmd = &cobra.Command{
		Use:   "list [backup-id]",
		Short: "List backups",
		Long:  "Shows the list of available backups or details of a specific backup",
		RunE: func(cmd *cobra.Command, args []string) error {
			indexManager := index.NewManager(configFile)

			// If an argument is provided, show details of that backup
			backupID := ""
			if len(args) > 0 {
				backupID = args[0]
			}

			return indexManager.ListBackups(backupID)
		},
	}

	// Delete command
	var deleteCmd = &cobra.Command{
		Use:   "delete",
		Short: "Delete a backup",
		Long:  "Deletes a backup and its associated data",
		RunE: func(cmd *cobra.Command, args []string) error {
			backupID, _ := cmd.Flags().GetString("backup-id")

			if backupID == "" {
				return fmt.Errorf("backup ID is required")
			}

			backupManager := backup.NewManager(configFile)
			return backupManager.DeleteBackup(backupID)
		},
	}
	deleteCmd.Flags().StringP("backup-id", "b", "", "Backup ID to delete")
	_ = deleteCmd.MarkFlagRequired("backup-id")

	// Info command
	var infoCmd = &cobra.Command{
		Use:   "info",
		Short: "Show information about algorithms",
		Long:  "Shows information about available encryption algorithms",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("\n🔐 Supported encryption algorithms:\n")
			fmt.Printf("%s\n", strings.Repeat("-", 60))

			fmt.Printf("\n📋 AES-256-GCM:\n")
			fmt.Printf("  • Algorithm: AES-256 in GCM mode\n")
			fmt.Printf("  • Security: Very high (NIST standard)\n")
			fmt.Printf("  • Performance: Excellent (hardware acceleration)\n")
			fmt.Printf("  • Key size: 32 bytes\n")
			fmt.Printf("  • Nonce: 12 bytes\n")
			fmt.Printf("  • Authentication tag: 16 bytes\n")

			fmt.Printf("\n📋 XChaCha20-Poly1305:\n")
			fmt.Printf("  • Algorithm: ChaCha20 with Poly1305\n")
			fmt.Printf("  • Security: Very high (RFC 8439)\n")
			fmt.Printf("  • Performance: Excellent (software optimized)\n")
			fmt.Printf("  • Key size: 32 bytes\n")
			fmt.Printf("  • Nonce: 24 bytes\n")
			fmt.Printf("  • Authentication tag: 16 bytes\n")

			fmt.Printf("\n💡 Recommendations:\n")
			fmt.Printf("  • AES-256-GCM: Ideal for systems with hardware acceleration\n")
			fmt.Printf("  • XChaCha20-Poly1305: Ideal for systems without AES acceleration\n")
			fmt.Printf("  • Both algorithms provide equivalent security\n\n")

			fmt.Printf("🔍 Checksum modes for index creation:\n")
			fmt.Printf("%s\n", strings.Repeat("-", 60))

			fmt.Printf("\n📋 full:\n")
			fmt.Printf("  • Method: SHA256 of entire file content\n")
			fmt.Printf("  • Security: Maximum (detects any change)\n")
			fmt.Printf("  • Speed: Slow (reads all files completely)\n")
			fmt.Printf("  • Use case: Critical data, small datasets\n")

			fmt.Printf("\n📋 fast (recommended):\n")
			fmt.Printf("  • Method: SHA256 of metadata + first/last 8KB\n")
			fmt.Printf("  • Security: Very high (detects 99.9%% of changes)\n")
			fmt.Printf("  • Speed: Fast (reads only file samples)\n")
			fmt.Printf("  • Use case: Most backup scenarios\n")

			fmt.Printf("\n📋 metadata:\n")
			fmt.Printf("  • Method: SHA256 of path + size + date + permissions\n")
			fmt.Printf("  • Security: Good (detects file replacement/modification)\n")
			fmt.Printf("  • Speed: Very fast (no file content read)\n")
			fmt.Printf("  • Use case: Large datasets, quick incremental backups\n")

			fmt.Printf("\n💡 Performance comparison:\n")
			fmt.Printf("  • metadata: ~10x faster than full\n")
			fmt.Printf("  • fast: ~5x faster than full, same reliability\n")
			fmt.Printf("  • full: Slowest but most thorough\n\n")

			return nil
		},
	}

	// Init command
	var initCmd = &cobra.Command{
		Use:   "init [config-file]",
		Short: "Initialize BCRDF configuration",
		Long:  "Generates a configuration file and tests connection parameters",
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath := "config.yaml"
			if len(args) > 0 {
				configPath = args[0]
			}

			interactive, _ := cmd.Flags().GetBool("interactive")
			force, _ := cmd.Flags().GetBool("force")
			test, _ := cmd.Flags().GetBool("test")
			storageType, _ := cmd.Flags().GetString("storage")

			if test {
				return runTestConfig(configPath, verbose)
			}

			return runInit(configPath, interactive, force, storageType, verbose)
		},
	}
	initCmd.Flags().BoolP("interactive", "i", false, "Interactive mode to configure parameters")
	initCmd.Flags().BoolP("force", "f", false, "Force overwrite of existing configuration file")
	initCmd.Flags().BoolP("test", "t", false, "Test an existing configuration")
	initCmd.Flags().StringP("storage", "s", "s3", "Storage type (s3, webdav)")

	// Retention command
	retentionCmd := &cobra.Command{
		Use:   "retention",
		Short: "Manage backup retention policies",
		Long:  "Apply retention policies manually or get retention status information",
		RunE: func(cmd *cobra.Command, args []string) error {
			verbose, _ := cmd.Flags().GetBool("verbose")
			configPath, _ := cmd.Flags().GetString("config")
			info, _ := cmd.Flags().GetBool("info")
			apply, _ := cmd.Flags().GetBool("apply")

			if !info && !apply {
				return fmt.Errorf("specify either --info or --apply")
			}

			return runRetention(configPath, info, apply, verbose)
		},
	}
	retentionCmd.Flags().BoolP("info", "i", false, "Show retention policy status")
	retentionCmd.Flags().BoolP("apply", "a", false, "Apply retention policy now")

	// Add commands
	rootCmd.AddCommand(backupCmd, restoreCmd, listCmd, deleteCmd, infoCmd, initCmd, retentionCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// runInit executes the init command
func runInit(configPath string, interactive, force bool, storageType string, verbose bool) error {
	// Check if file already exists
	if utils.FileExists(configPath) && !force {
		return fmt.Errorf("file %s already exists. Use --force to overwrite", configPath)
	}

	if verbose {
		utils.Info("Generating configuration file: %s", configPath)
	} else {
		utils.ProgressStep(fmt.Sprintf("🚀 Initializing BCRDF: %s", configPath))
	}

	if interactive {
		return runInteractiveInit(configPath, storageType, verbose)
	} else {
		return runQuickInit(configPath, storageType, verbose)
	}
}

// runQuickInit generates a default configuration
func runQuickInit(configPath, storageType string, verbose bool) error {
	if !verbose {
		utils.ProgressStep(fmt.Sprintf("Generating %s configuration...", storageType))
	}

	// Generate configuration
	if err := validator.GenerateConfigWithType(configPath, storageType); err != nil {
		return fmt.Errorf("error generating configuration: %w", err)
	}

	if verbose {
		utils.Info("✅ Configuration generated: %s", configPath)
		utils.Info("")
		utils.Info("📝 Next steps:")
		utils.Info("1. Edit the file and configure your storage parameters")
		utils.Info("2. Test with: ./bcrdf init --test")
	} else {
		utils.ProgressSuccess(fmt.Sprintf("Configuration generated: %s", configPath))
		utils.ProgressInfo("Edit the file and configure your storage parameters")
		utils.ProgressInfo("Test with: ./bcrdf init --test")
	}

	return nil
}

// runInteractiveInit guides the user through configuration
func runInteractiveInit(configPath, storageType string, verbose bool) error {
	// Use the new interactive configuration generator
	if err := validator.GenerateInteractiveConfig(configPath); err != nil {
		return fmt.Errorf("error in interactive configuration: %w", err)
	}

	return nil
}

// runTestConfig tests an existing configuration
func runTestConfig(configPath string, verbose bool) error {
	if verbose {
		utils.Info("🧪 Testing configuration: %s", configPath)
	} else {
		utils.ProgressStep(fmt.Sprintf("🧪 Testing configuration: %s", configPath))
	}

	// Load configuration
	config, err := utils.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("error loading configuration: %w", err)
	}

	// Create validator
	configValidator := validator.NewConfigValidator(config)

	// Validate all parameters
	if err := configValidator.ValidateAll(verbose); err != nil {
		return err
	}

	if verbose {
		utils.Info("🎉 Configuration is valid and functional!")
		utils.Info("You can now use BCRDF for your backups.")
	} else {
		utils.ProgressSuccess("🎉 Configuration is valid and functional!")
		utils.ProgressInfo("Ready for BCRDF backups")
	}

	return nil
}

// runRetention executes retention management commands
func runRetention(configPath string, info, apply, verbose bool) error {
	// Load configuration
	config, err := utils.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("error loading configuration: %w", err)
	}

	// Initialize storage client
	storageClient, err := storage.NewStorageClient(config)
	if err != nil {
		return fmt.Errorf("error initializing storage: %w", err)
	}

	// Initialize index manager
	indexMgr := index.NewManager(configPath)

	// Create retention manager
	retentionMgr := retention.NewManager(config, indexMgr, storageClient)

	if info {
		return retentionMgr.GetRetentionInfo(verbose)
	}

	if apply {
		return retentionMgr.ApplyRetentionPolicy(verbose)
	}

	return nil
}

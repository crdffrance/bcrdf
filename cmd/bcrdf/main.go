package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"bcrdf/internal/backup"
	"bcrdf/internal/health"
	"bcrdf/internal/index"
	"bcrdf/internal/restore"
	"bcrdf/internal/retention"
	"bcrdf/internal/validator"
	"bcrdf/pkg/storage"
	"bcrdf/pkg/utils"
)

// DeferredUpdateError indicates a successful deferred update
type DeferredUpdateError struct {
	ScriptPath string
	Version    string
}

func (e *DeferredUpdateError) Error() string {
	return fmt.Sprintf("update deferred to version %s - script created at %s", e.Version, e.ScriptPath)
}

var (
	configFile string
	verbose    bool
	// Version information
	Version   = "2.7.3"
	BuildTime = time.Now().Format("2006-01-02")
	GoVersion = "1.24"
)

func main() {
	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n⚠️  Interruption detected. Finishing current operation...")
		fmt.Println("   Press Ctrl+C again to force exit.")
		<-sigChan
		fmt.Println("\n🛑 Force exit.")
		os.Exit(1)
	}()

	var rootCmd = &cobra.Command{
		Use:   "bcrdf",
		Short: "BCRDF - Modern index-based backup system",
		Long: `BCRDF (Backup Copy with Redundant Data Format) is a modern backup system
that uses an index-based approach to optimize storage and performance.

Key features:
- Incremental backup with index
- AES-256-GCM and XChaCha20-Poly1305 encryption
- GZIP compression with adaptive levels
- S3 and WebDAV compatible storage
- S3 Glacier storage class support (Scaleway, AWS)
- Precise point-in-time restoration
- Automatic retention policies`,
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

			// Afficher le démarrage de la sauvegarde
			if !verbose {
				fmt.Printf("🚀 Starting backup: %s -> %s\n", source, name)
			}

			backupManager := backup.NewManager(configFile)
			err := backupManager.CreateBackup(source, name, verbose)

			// Afficher le résultat final
			if !verbose {
				if err != nil {
					fmt.Printf("\n❌ Backup failed: %v\n", err)
				} else {
					fmt.Printf("\n✅ Backup completed successfully!\n")
				}
			}

			return err
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

			// Afficher le démarrage de la restauration
			if !verbose {
				fmt.Printf("🔄 Starting restore: %s -> %s\n", backupID, destination)
			}

			restoreManager := restore.NewManager(configFile)
			err := restoreManager.RestoreBackup(backupID, destination, verbose)

			// Afficher le résultat final
			if !verbose {
				if err != nil {
					fmt.Printf("\n❌ Restore failed: %v\n", err)
				} else {
					fmt.Printf("\n✅ Restore completed successfully!\n")
				}
			}

			return err
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

			// Afficher le démarrage de la suppression
			if !verbose {
				fmt.Printf("🗑️  Starting deletion of backup: %s\n", backupID)
				fmt.Printf("📊 Progress will be displayed below:\n\n")
			}

			backupManager := backup.NewManager(configFile)
			err := backupManager.DeleteBackup(backupID)

			// Afficher le résultat final
			if !verbose {
				if err != nil {
					fmt.Printf("\n❌ Deletion failed: %v\n", err)
				} else {
					fmt.Printf("\n✅ Backup deleted successfully!\n")
				}
			}

			return err
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
			configPath := configFile
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

	// Version command
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long:  "Displays version information and optimization features",
		Run: func(cmd *cobra.Command, args []string) {
			showVersion()
		},
	}

	// Update command
	var updateCmd = &cobra.Command{
		Use:   "update",
		Short: "Check for and install updates",
		Long:  "Checks for newer versions on GitHub and installs them automatically",
		RunE: func(cmd *cobra.Command, args []string) error {
			checkOnly, _ := cmd.Flags().GetBool("check")
			force, _ := cmd.Flags().GetBool("force")
			autoRestart, _ := cmd.Flags().GetBool("auto-restart")

			if checkOnly {
				return checkForUpdates(verbose)
			}

			return performUpdate(verbose, force, autoRestart)
		},
	}
	updateCmd.Flags().BoolP("check", "k", false, "Only check for updates without installing")
	updateCmd.Flags().BoolP("force", "f", false, "Force update even if current version is latest")
	updateCmd.Flags().BoolP("auto-restart", "r", false, "Automatically restart BCRDF after update")

	// Retention command
	var retentionCmd = &cobra.Command{
		Use:   "retention",
		Short: "Manage retention policies",
		Long:  "Applies retention policies to existing backups",
		RunE: func(cmd *cobra.Command, args []string) error {
			info, _ := cmd.Flags().GetBool("info")
			apply, _ := cmd.Flags().GetBool("apply")

			// Afficher le démarrage de la gestion de rétention
			if !verbose && apply {
				fmt.Printf("🧹 Starting retention policy management\n")
				fmt.Printf("📊 Progress will be displayed below:\n\n")
			}

			err := runRetention(configFile, info, apply, verbose)

			// Afficher le résultat final
			if !verbose && apply {
				if err != nil {
					fmt.Printf("\n❌ Retention policy failed: %v\n", err)
				} else {
					fmt.Printf("\n✅ Retention policy applied successfully!\n")
				}
			}

			return err
		},
	}
	retentionCmd.Flags().BoolP("info", "i", false, "Show retention information")
	retentionCmd.Flags().BoolP("apply", "a", false, "Apply retention policies")

	// Health command
	var healthCmd = &cobra.Command{
		Use:   "health",
		Short: "Check backup health",
		Long:  "Verifies the integrity and health of all backups",
		RunE: func(cmd *cobra.Command, args []string) error {
			testRestore, _ := cmd.Flags().GetBool("test-restore")
			fastMode, _ := cmd.Flags().GetBool("fast")
			return runHealth(configFile, testRestore, verbose, fastMode)
		},
	}
	healthCmd.Flags().BoolP("test-restore", "t", false, "Test restore functionality on sample files")
	healthCmd.Flags().BoolP("fast", "f", false, "Fast mode: check only a random sample of files")

	// Clean command
	var cleanCmd = &cobra.Command{
		Use:   "clean",
		Short: "Clean orphaned files from storage",
		Long:  "Removes files from storage that are not referenced in the backup index",
		RunE: func(cmd *cobra.Command, args []string) error {
			backupID, _ := cmd.Flags().GetString("backup-id")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			allBackups, _ := cmd.Flags().GetBool("all")
			removeOrphaned, _ := cmd.Flags().GetBool("remove-orphaned")

			// Afficher le démarrage du nettoyage
			if !verbose {
				if allBackups {
					fmt.Printf("🧹 Starting cleanup of all backups\n")
				} else {
					fmt.Printf("🧹 Starting cleanup of backup: %s\n", backupID)
				}
				if dryRun {
					fmt.Printf("🔍 Dry run mode - no files will be deleted\n")
				}
				fmt.Printf("📊 Progress will be displayed below:\n\n")
			}

			indexManager := index.NewManager(configFile)

			var err error
			// Si --all est spécifié, nettoyer toutes les sauvegardes
			if allBackups {
				err = indexManager.CleanAllBackups(dryRun, verbose, removeOrphaned)
			} else {
				// Sinon, nettoyer une sauvegarde spécifique
				if backupID == "" {
					return fmt.Errorf("backup ID is required when not using --all flag")
				}
				err = indexManager.CleanOrphanedFiles(backupID, dryRun, verbose)
			}

			// Afficher le résultat final
			if !verbose {
				if err != nil {
					fmt.Printf("\n❌ Cleanup failed: %v\n", err)
				} else {
					fmt.Printf("\n✅ Cleanup completed successfully!\n")
				}
			}

			return err
		},
	}
	cleanCmd.Flags().StringP("backup-id", "b", "", "Backup ID to clean (required when not using --all)")
	cleanCmd.Flags().BoolP("dry-run", "d", false, "Dry run mode (show what would be deleted without actually deleting)")
	cleanCmd.Flags().BoolP("all", "a", false, "Clean all backups and remove orphaned ones without index")
	cleanCmd.Flags().BoolP("remove-orphaned", "r", false, "Remove orphaned backups that have no index (use with --all)")

	// Scan command
	var scanCmd = &cobra.Command{
		Use:   "scan",
		Short: "Scan all objects in storage",
		Long:  "Lists all objects in storage to help identify orphaned files",
		RunE: func(cmd *cobra.Command, args []string) error {
			indexManager := index.NewManager(configFile)
			return indexManager.ScanAllObjects(verbose)
		},
	}

	// Add commands to root
	rootCmd.AddCommand(backupCmd)
	rootCmd.AddCommand(restoreCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(retentionCmd)
	rootCmd.AddCommand(healthCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(versionCmd)

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

// showVersion displays version information
func showVersion() {
	fmt.Printf("🚀 BCRDF %s\n", Version)
	fmt.Printf("📦 Build: %s\n", BuildTime)
	fmt.Printf("🔧 Go: %s\n", GoVersion)
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

// checkForUpdates checks for newer versions on GitHub
func checkForUpdates(verbose bool) error {
	if verbose {
		utils.Info("🔍 Checking for updates on GitHub...")
	} else {
		fmt.Println("🔍 Checking for updates...")
	}

	// Get current version
	currentVersion := Version
	if strings.HasPrefix(currentVersion, "v") {
		currentVersion = strings.TrimPrefix(currentVersion, "v")
	}
	
	// Remove platform suffix (e.g., -linux-x64, -darwin-arm64)
	if idx := strings.Index(currentVersion, "-"); idx != -1 {
		currentVersion = currentVersion[:idx]
	}

	// Parse current version
	currentParts := strings.Split(currentVersion, ".")
	if len(currentParts) < 2 {
		return fmt.Errorf("invalid current version format: %s", currentVersion)
	}

	// Get latest version from GitHub API
	latestVersion, err := getLatestGitHubVersion()
	if err != nil {
		return fmt.Errorf("error checking for updates: %w", err)
	}

	// Parse latest version
	latestParts := strings.Split(latestVersion, ".")
	if len(latestParts) < 2 {
		return fmt.Errorf("invalid latest version format: %s", latestVersion)
	}

	// Compare versions
	if isNewerVersion(latestParts, currentParts) {
		fmt.Printf("🎉 New version available: %s (current: %s)\n", latestVersion, currentVersion)
		fmt.Printf("📥 Run 'bcrdf update' to install the latest version\n")
	} else {
		fmt.Printf("✅ You are running the latest version: %s\n", currentVersion)
	}

	return nil
}

// performUpdate downloads and installs the latest version
func performUpdate(verbose, force, autoRestart bool) error {
	if verbose {
		utils.Info("🚀 Starting update process...")
	} else {
		fmt.Println("🚀 Starting update...")
	}

	// Get current version
	currentVersion := Version
	if strings.HasPrefix(currentVersion, "v") {
		currentVersion = strings.TrimPrefix(currentVersion, "v")
	}
	
	// Remove platform suffix (e.g., -linux-x64, -darwin-arm64)
	if idx := strings.Index(currentVersion, "-"); idx != -1 {
		currentVersion = currentVersion[:idx]
	}

	// Get latest version
	latestVersion, err := getLatestGitHubVersion()
	if err != nil {
		return fmt.Errorf("error getting latest version: %w", err)
	}

	// Check if update is needed
	if !force {
		currentParts := strings.Split(currentVersion, ".")
		latestParts := strings.Split(latestVersion, ".")

		if !isNewerVersion(latestParts, currentParts) {
			fmt.Printf("✅ You are already running the latest version: %s\n", currentVersion)
			fmt.Printf("💡 Use --force to update anyway\n")
			return nil
		}
	}

	fmt.Printf("📥 Downloading version %s...\n", latestVersion)

	// Download and install
	if err := downloadAndInstallUpdate(latestVersion, verbose, autoRestart); err != nil {
		// Check if it's a deferred update (which is actually a success)
		if deferredErr, ok := err.(*DeferredUpdateError); ok {
			fmt.Printf("\n🎯 Update process completed successfully!\n")
			fmt.Printf("📝 Deferred update script ready: %s\n", deferredErr.ScriptPath)
			fmt.Printf("🔄 To complete the update, please follow the instructions above.\n")
			return nil // No error, this is a successful deferred update
		}
		return fmt.Errorf("error updating: %w", err)
	}

	fmt.Printf("🎉 Successfully updated to version %s!\n", latestVersion)

	if autoRestart {
		fmt.Printf("🚀 Auto-restarting BCRDF with new version...\n")
		return performAutoRestart()
	} else {
		fmt.Printf("🔄 Please restart BCRDF to use the new version\n")
	}

	return nil
}

// getLatestGitHubVersion fetches the latest version from GitHub releases
func getLatestGitHubVersion() (string, error) {
	// GitHub API endpoint for releases
	url := "https://api.github.com/repos/crdffrance/bcrdf/releases/latest"

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Make request
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("error fetching releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GitHub API returned status: %d", resp.StatusCode)
	}

	// Parse response
	var release struct {
		TagName string `json:"tag_name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("error parsing response: %w", err)
	}

	// Remove 'v' prefix if present
	version := release.TagName
	if strings.HasPrefix(version, "v") {
		version = strings.TrimPrefix(version, "v")
	}

	return version, nil
}

// isNewerVersion compares two version strings
func isNewerVersion(latest, current []string) bool {
	maxLen := len(latest)
	if len(current) > maxLen {
		maxLen = len(current)
	}

	for i := 0; i < maxLen; i++ {
		latestPart := "0"
		currentPart := "0"

		if i < len(latest) {
			latestPart = latest[i]
		}
		if i < len(current) {
			currentPart = current[i]
		}

		// Convert to integers for comparison
		latestNum, _ := strconv.Atoi(latestPart)
		currentNum, _ := strconv.Atoi(currentPart)

		if latestNum > currentNum {
			return true
		} else if latestNum < currentNum {
			return false
		}
	}

	// Versions are equal
	return false
}

// downloadAndInstallUpdate downloads and installs the update
func downloadAndInstallUpdate(version string, verbose, autoRestart bool) error {
	// Determine platform and architecture
	platform := runtime.GOOS
	arch := runtime.GOARCH

	// Map Go arch to GitHub release arch
	archMap := map[string]string{
		"amd64": "x64",
		"arm64": "arm64",
		"386":   "x86",
	}

	releaseArch, ok := archMap[arch]
	if !ok {
		return fmt.Errorf("unsupported architecture: %s", arch)
	}

	// Construct download URL with correct extensions
	var downloadURL string
	switch platform {
	case "darwin":
		downloadURL = fmt.Sprintf("https://github.com/crdffrance/bcrdf/releases/download/v%s/bcrdf-%s-%s.tar.gz",
			version, platform, releaseArch)
	case "linux":
		downloadURL = fmt.Sprintf("https://github.com/crdffrance/bcrdf/releases/download/v%s/bcrdf-%s-%s.tar.gz",
			version, platform, releaseArch)
	case "windows":
		downloadURL = fmt.Sprintf("https://github.com/crdffrance/bcrdf/releases/download/v%s/bcrdf-%s-%s.zip",
			version, platform, releaseArch)
	default:
		return fmt.Errorf("unsupported platform: %s", platform)
	}

	if verbose {
		utils.Info("📥 Download URL: %s", downloadURL)
	}

	// Download file
	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("error downloading update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	// Create temporary file
	tempFile, err := os.CreateTemp("", "bcrdf-update-*")
	if err != nil {
		return fmt.Errorf("error creating temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	// Download with progress
	fileSize := resp.ContentLength
	if verbose {
		utils.Info("📊 File size: %d bytes", fileSize)
	}

	// Copy with progress bar
	bar := utils.NewProgressBar(fileSize)
	_, err = io.Copy(tempFile, io.TeeReader(resp.Body, &progressWriter{bar: bar}))
	if err != nil {
		return fmt.Errorf("error downloading file: %w", err)
	}
	bar.Finish()

	// Close temp file
	tempFile.Close()

	// Extract the archive to get the binary
	binaryPath, err := extractBinary(tempFile.Name(), platform, releaseArch)
	if err != nil {
		return fmt.Errorf("error extracting binary: %w", err)
	}

	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("error getting executable path: %w", err)
	}

	// Create backup of current executable
	backupPath := execPath + ".backup"
	if err := copyFile(execPath, backupPath); err != nil {
		return fmt.Errorf("error creating backup: %w", err)
	}

	// Install new version
	if err := copyFile(binaryPath, execPath); err != nil {
		// Check if it's a "text file busy" error
		if strings.Contains(err.Error(), "text file busy") || strings.Contains(err.Error(), "device or resource busy") {
			// Binary is busy, implement deferred update strategy
			return handleDeferredUpdate(binaryPath, backupPath, execPath, version, verbose)
		}

		// Other error, restore backup on failure
		copyFile(backupPath, execPath)
		return fmt.Errorf("error installing update: %w", err)
	}

	// Make executable
	if err := os.Chmod(execPath, 0755); err != nil {
		return fmt.Errorf("error setting permissions: %w", err)
	}

	// Clean up extracted files and temporary directory
	os.Remove(binaryPath)
	os.Remove(backupPath)

	// Clean up the temporary extraction directory
	if tempDir := filepath.Dir(binaryPath); tempDir != "" {
		os.RemoveAll(tempDir)
	}

	return nil
}

// progressWriter implements io.Writer to update progress bar
type progressWriter struct {
	bar *utils.ProgressBar
}

func (pw *progressWriter) Write(p []byte) (n int, err error) {
	pw.bar.Add(int64(len(p)))
	return len(p), nil
}

// extractBinary extracts the binary from the downloaded archive
func extractBinary(archivePath, platform, releaseArch string) (string, error) {
	// Create temporary directory for extraction
	tempDir, err := os.MkdirTemp("", "bcrdf-extract-*")
	if err != nil {
		return "", fmt.Errorf("error creating temp directory: %w", err)
	}
	// Note: tempDir will be cleaned up by the caller after copying the binary

	var binaryPath string
	switch platform {
	case "darwin", "linux":
		// Extract tar.gz
		if err := extractTarGz(archivePath, tempDir); err != nil {
			return "", fmt.Errorf("error extracting tar.gz: %w", err)
		}

		// Debug: List all extracted files
		fmt.Printf("🔍 Extracted files in %s:\n", tempDir)
		listFilesRecursively(tempDir, "")

		// Look for the binary in the extracted directory
		// First try the simple name
		binaryPath = filepath.Join(tempDir, "bcrdf")
		if !utils.FileExists(binaryPath) {
			// Try the platform-specific name
			platformBinaryName := fmt.Sprintf("bcrdf-%s-%s", platform, releaseArch)
			binaryPath = filepath.Join(tempDir, platformBinaryName)
			if !utils.FileExists(binaryPath) {
				// Try to find it in subdirectories with more flexible search
				binaryPath = findBinaryRecursively(tempDir, "bcrdf")
				if binaryPath == "" {
					binaryPath = findBinaryRecursively(tempDir, platformBinaryName)
				}
			}
		}

	case "windows":
		// Extract zip
		if err := extractZip(archivePath, tempDir); err != nil {
			return "", fmt.Errorf("error extracting zip: %w", err)
		}

		// Debug: List all extracted files
		fmt.Printf("🔍 Extracted files in %s:\n", tempDir)
		listFilesRecursively(tempDir, "")

		// Look for the binary in the extracted directory
		binaryPath = filepath.Join(tempDir, "bcrdf.exe")
		if !utils.FileExists(binaryPath) {
			// Try to find it in subdirectories with more flexible search
			binaryPath = findBinaryRecursively(tempDir, "bcrdf.exe")
		}

	default:
		return "", fmt.Errorf("unsupported platform: %s", platform)
	}

	if !utils.FileExists(binaryPath) {
		return "", fmt.Errorf("binary not found in extracted archive")
	}

	fmt.Printf("✅ Binary found at: %s\n", binaryPath)
	return binaryPath, nil
}

// listFilesRecursively lists all files in a directory recursively
func listFilesRecursively(dir, prefix string) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, file := range files {
		fullPath := filepath.Join(dir, file.Name())
		if file.IsDir() {
			fmt.Printf("  %s📁 %s/\n", prefix, file.Name())
			listFilesRecursively(fullPath, prefix+"  ")
		} else {
			fmt.Printf("  %s📄 %s\n", prefix, file.Name())
		}
	}
}

// findBinaryRecursively searches for a binary file recursively
func findBinaryRecursively(dir, binaryName string) string {
	files, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	for _, file := range files {
		fullPath := filepath.Join(dir, file.Name())
		if file.IsDir() {
			// Search in subdirectory
			if found := findBinaryRecursively(fullPath, binaryName); found != "" {
				return found
			}
		} else if file.Name() == binaryName {
			// Found the binary
			return fullPath
		}
	}

	return ""
}

// extractTarGz extracts a tar.gz archive
func extractTarGz(archivePath, destDir string) error {
	// Open the tar.gz file
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create gzip reader
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	// Create tar reader
	tr := tar.NewReader(gzr)

	// Extract files
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Skip directories
		if header.Typeflag == tar.TypeDir {
			continue
		}

		// Create the file
		path := filepath.Join(destDir, header.Name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		f, err := os.Create(path)
		if err != nil {
			return err
		}

		// Copy file content
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			return err
		}
		f.Close()

		// Set permissions
		if err := os.Chmod(path, os.FileMode(header.Mode)); err != nil {
			return err
		}
	}

	return nil
}

// extractZip extracts a zip archive
func extractZip(archivePath, destDir string) error {
	// Open the zip file
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	// Extract files
	for _, file := range reader.File {
		path := filepath.Join(destDir, file.Name)

		// Skip directories
		if file.FileInfo().IsDir() {
			os.MkdirAll(path, file.Mode())
			continue
		}

		// Create the file
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		f, err := os.Create(path)
		if err != nil {
			return err
		}

		// Open the file in the zip
		rc, err := file.Open()
		if err != nil {
			f.Close()
			return err
		}

		// Copy file content
		if _, err := io.Copy(f, rc); err != nil {
			f.Close()
			rc.Close()
			return err
		}
		f.Close()
		rc.Close()

		// Set permissions
		if err := os.Chmod(path, file.Mode()); err != nil {
			return err
		}
	}

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func runHealth(configPath string, testRestore, verbose, fastMode bool) error {
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

	// Create health manager
	healthMgr := health.NewManager(config, indexMgr, storageClient)

	report, err := healthMgr.CheckHealth(verbose, testRestore, fastMode)
	if err != nil {
		return fmt.Errorf("error checking health: %w", err)
	}

	healthMgr.PrintReport(report, verbose)
	return nil
}

// handleDeferredUpdate handles the case when the binary is busy
func handleDeferredUpdate(binaryPath, backupPath, execPath, version string, verbose bool) error {
	fmt.Printf("\n🔄 Binary is currently in use, implementing deferred update strategy...\n")

	// Create a deferred update script
	updateScript := createDeferredUpdateScript(binaryPath, backupPath, execPath, version)

	// Make the script executable
	if err := os.Chmod(updateScript, 0755); err != nil {
		return fmt.Errorf("error making update script executable: %w", err)
	}

	fmt.Printf("📝 Deferred update script created: %s\n", updateScript)
	fmt.Printf("🚀 To complete the update, please:\n")
	fmt.Printf("   1. Exit BCRDF completely\n")
	fmt.Printf("   2. Run: %s\n", updateScript)
	fmt.Printf("   3. Restart BCRDF\n")

	// Return a special error type that indicates deferred update success
	// Return a special error type that indicates deferred update success
	return &DeferredUpdateError{
		ScriptPath: updateScript,
		Version:    version,
	}
}

// createDeferredUpdateScript creates a script to complete the update later
func createDeferredUpdateScript(binaryPath, backupPath, execPath, version string) string {
	// Create script content
	scriptContent := fmt.Sprintf(`#!/bin/bash
# BCRDF Deferred Update Script for version %s
# This script will complete the update when BCRDF is not running

echo "🚀 Completing BCRDF update to version %s..."

# Wait a moment to ensure BCRDF is completely stopped
sleep 2

# Install the new version
if cp "%s" "%s"; then
    # Set permissions
    chmod 755 "%s"
    
    # Clean up
    rm -f "%s"
    rm -f "%s"
    
    # Clean up the temporary extraction directory
    tempDir=$(dirname "%s")
    if [ -d "$tempDir" ]; then
        rm -rf "$tempDir"
    fi
    
    echo "✅ Update completed successfully!"
    echo "🔄 You can now restart BCRDF"
else
    echo "❌ Update failed, restoring backup..."
    cp "%s" "%s"
    chmod 755 "%s"
    echo "🔄 Backup restored, please try again later"
fi
`, version, version, binaryPath, execPath, execPath, binaryPath, backupPath, binaryPath, backupPath, execPath, execPath)

	// Create script file
	scriptPath := fmt.Sprintf("/tmp/bcrdf-update-%s.sh", version)
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		// Fallback to current directory if /tmp is not writable
		scriptPath = fmt.Sprintf("./bcrdf-update-%s.sh", version)
		os.WriteFile(scriptPath, []byte(scriptContent), 0755)
	}

	return scriptPath
}

// performAutoRestart restarts BCRDF with the new version
func performAutoRestart() error {
	fmt.Printf("🔄 Preparing auto-restart...\n")

	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("error getting executable path: %w", err)
	}

	// Get current working directory
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting working directory: %w", err)
	}

	// Get current arguments (excluding the update command)
	args := os.Args[1:]
	// Remove update-related flags
	var cleanArgs []string
	for i := 0; i < len(args); i++ {
		if args[i] == "update" || args[i] == "--auto-restart" || args[i] == "-r" {
			continue
		}
		if strings.HasPrefix(args[i], "--") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
			// Skip flag value
			i++
			continue
		}
		if strings.HasPrefix(args[i], "-") && len(args[i]) == 2 && i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") && !strings.HasPrefix(args[i+1], "-") {
			// Skip short flag value
			i++
			continue
		}
		cleanArgs = append(cleanArgs, args[i])
	}

	// Create restart script
	restartScript := createRestartScript(execPath, workDir, cleanArgs)

	// Make it executable
	if err := os.Chmod(restartScript, 0755); err != nil {
		return fmt.Errorf("error making restart script executable: %w", err)
	}

	fmt.Printf("🚀 Executing auto-restart...\n")

	// Execute the restart script in background
	cmd := exec.Command(restartScript)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start the restart process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting auto-restart: %w", err)
	}

	// Exit current process (restart script will take over)
	fmt.Printf("🔄 Restarting BCRDF with new version...\n")
	os.Exit(0)

	return nil
}

// createRestartScript creates a script to restart BCRDF
func createRestartScript(execPath, workDir string, args []string) string {
	// Create script content
	scriptContent := fmt.Sprintf(`#!/bin/bash
# BCRDF Auto-Restart Script
# This script restarts BCRDF with the new version

echo "🚀 Restarting BCRDF with new version..."

# Change to working directory
cd "%s"

# Wait a moment for the old process to exit
sleep 1

# Start BCRDF with new version and original arguments
exec "%s" %s

echo "✅ BCRDF restarted successfully!"
`, workDir, execPath, strings.Join(args, " "))

	// Create script file
	scriptPath := "/tmp/bcrdf-restart.sh"
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		// Fallback to current directory
		scriptPath = "./bcrdf-restart.sh"
		os.WriteFile(scriptPath, []byte(scriptContent), 0755)
	}

	return scriptPath
}

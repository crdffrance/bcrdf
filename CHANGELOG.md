# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.7.4] - 2025-01-16

### Fixed
- **Version Comparison Bug**: Fixed critical issue where version comparison failed due to platform suffixes
- **Update Command Reliability**: Resolved bug where update command downloaded same version repeatedly
- **Platform Suffix Handling**: Remove platform suffixes (-linux-x64, -darwin-arm64) before version comparison

### Enhanced
- **Update Command Accuracy**: Both update and update --check now correctly detect when on latest version
- **Version Detection**: Improved version parsing and comparison logic
- **User Experience**: Eliminates unnecessary downloads of identical versions

### Technical Details
- Fixed version comparison by cleaning platform suffixes before comparison
- Enhanced version parsing in both checkForUpdates and performUpdate functions
- Improved reliability of update system across all platforms

## [2.7.3] - 2025-01-16

### Fixed
- **Update Command Version Detection**: Fixed critical issue where update command was downloading the same version when already on latest
- **Version Comparison Logic**: Resolved bug preventing proper detection of current version vs latest version
- **Update System Reliability**: Command now correctly identifies when no update is needed

### Enhanced
- **Update Command Consistency**: Both `update` and `update --check` now work consistently
- **User Experience**: Clear messaging when already on latest version
- **Error Prevention**: Eliminates unnecessary downloads of identical versions

### Technical Details
- Fixed version comparison logic in `isNewerVersion` function
- Improved update command reliability and consistency
- Enhanced user feedback for update operations

## [2.7.2] - 2025-01-16

### Fixed
- **Critical Update Command Bug**: Fixed missing update command implementation that was referenced but not defined
- **Update System Reliability**: Resolved 'text file busy' errors during updates by properly implementing deferred update strategy
- **Command Integration**: Added complete update command with --check, --force, and --auto-restart flags

### Enhanced
- **Update Command Functionality**: Command now properly detects when already on latest version
- **Error Handling**: Improved error handling for update operations with proper version detection
- **User Experience**: Users can now successfully check for updates and install them without errors

### Technical Details
- Added missing updateCmd definition in main.go
- Integrated with existing update functions (checkForUpdates, performUpdate)
- Fixed critical bug preventing update command from working
- Enhanced update system reliability and user experience

## [2.7.1] - 2025-08-15

### Fixed
- **Binary Detection in Archives**: Fixed issue where binary files were not found after archive extraction
- **Temporary File Management**: Corrected premature deletion of temporary files during update process
- **Platform-Specific Binary Names**: Added support for platform-specific binary names (e.g., bcrdf-darwin-arm64)
- **Update Process Reliability**: Resolved critical issue preventing successful updates from GitHub releases

### Enhanced
- **Update System Debug**: Added detailed file listing during archive extraction for better troubleshooting
- **Binary Search Algorithm**: Improved recursive search for binaries within extracted archives
- **Error Recovery**: Better error handling and cleanup during the update process
- **Cross-Platform Support**: Enhanced compatibility for all supported platforms and architectures

### Technical Details
- Fixed binary detection to handle platform-specific naming conventions
- Improved temporary file lifecycle management during updates
- Added debug output for archive extraction process
- Enhanced recursive file search for binary detection
- Optimized cleanup process for temporary files and directories

## [2.7.0] - 2025-08-15

### Fixed
- **Update System URLs**: Fixed incorrect download URLs for GitHub releases
- **Archive Extraction**: Added support for .tar.gz and .zip archives from GitHub releases
- **Cross-Platform Compatibility**: Corrected file extensions for Linux (.tar.gz), macOS (.tar.gz), and Windows (.zip)
- **Binary Detection**: Improved binary extraction and detection within compressed archives

### Enhanced
- **Update Reliability**: System now correctly downloads and extracts binaries from GitHub releases
- **Error Handling**: Better error messages and recovery for update failures
- **Archive Support**: Full support for compressed archives (tar.gz, zip) with automatic extraction

### Technical Details
- Fixed download URL construction for different platforms
- Added tar.gz extraction support for Linux and macOS
- Added zip extraction support for Windows
- Improved binary path detection within extracted archives
- Enhanced error handling for archive extraction

## [2.6.0] - 2025-08-15

### Added
- **Automatic Update System**: New `update` command for checking and installing updates from GitHub releases
- **GitHub Integration**: Automatic version checking via GitHub API
- **Cross-Platform Updates**: Support for macOS, Linux, and Windows with automatic architecture detection
- **Secure Update Process**: Automatic backup and rollback in case of update failure
- **Progress Tracking**: Real-time progress bar during update downloads

### Features
- **Update Check**: `bcrdf update --check` to verify available updates
- **Automatic Installation**: `bcrdf update` to download and install latest version
- **Force Update**: `bcrdf update --force` to update even if already on latest version
- **Platform Detection**: Automatic OS and architecture detection for correct binary selection
- **Error Recovery**: Automatic restoration of previous version if update fails

### Technical Details
- GitHub API integration for version checking
- Multi-platform binary download support
- Secure file replacement with backup/restore
- Progress bar integration for download tracking
- Automatic permission management (0755)

## [2.5.0] - 2025-08-15

### Added
- **Automatic S3 Object Cleanup**: New `cleanupUnreferencedObjects` function to prevent `NoSuchKey` errors during restore
- **Enhanced Backup Integrity**: Automatic cleanup of orphaned S3 objects after each backup operation
- **Improved Restore Reliability**: Eliminated `NoSuchKey` errors that occurred with incremental backups

### Fixed
- **Critical Restore Errors**: Fixed `NoSuchKey` errors that prevented successful restoration of specific backup points
- **Incremental Backup Consistency**: Resolved inconsistency between backup indexes and actual S3 objects
- **Object Lifecycle Management**: Automatic cleanup of S3 objects that are no longer referenced by current backups

### Changed
- **Backup Process Enhancement**: Added cleanup step after file backup to ensure S3 storage consistency
- **Error Handling**: Improved error handling for S3 operations with automatic cleanup on failures
- **Version Update**: Bumped to version 2.6.0 for this critical fix release

### Technical Details
- Implemented `cleanupUnreferencedObjects` in `internal/backup/manager.go`
- Added automatic cleanup call in `executeBackup` function
- Enhanced logging to track cleanup operations
- Improved S3 object lifecycle management for incremental backups

## [2.4.0] - 2024-08-08

### Added
- **S3 Glacier Storage Class Support**: Added support for S3 storage classes including GLACIER, DEEP_ARCHIVE, and INTELLIGENT_TIERING
- **Scaleway S3 Glacier Integration**: Optimized configuration for Scaleway S3 Glacier storage
- **Storage Class Validation**: Added validation for all supported S3 storage classes
- **Glacier-Optimized Configuration**: Created `config-scaleway-glacier.yaml` with settings optimized for long-term archival
- **Enhanced S3 Client**: Added `UploadWithStorageClass` method for storage class specification
- **Factory Support**: Updated storage factory to handle storage class configuration

### Changed
- **Version Bump**: Updated to version 2.4.0
- **Documentation**: Updated README with Glacier storage class information
- **Configuration Examples**: Enhanced S3 configuration examples with all storage classes

### Technical Details
- Added `StorageClass` field to S3 configuration structure
- Implemented storage class validation in config validator
- Created `NewS3AdapterWithStorageClass` for Glacier support
- Updated factory to detect and use storage class configuration
- Added comprehensive Glacier configuration with optimized settings

## [2.3.2] - 2024-08-08

### Fixed
- **GitHub Actions Release**: Fixed missing permissions in release workflow
- **Documentation Updates**: Updated all documentation to reflect current version and Go version
- **Release Automation**: Improved release automation and tag management

### Changed
- **Version Update**: Updated to version 2.3.2
- **Documentation**: Corrected version references throughout documentation
- **Download Links**: Updated all download links to point to v2.3.2

## [2.3.0] - 2024-08-07

### Added
- **Debug Logging System**: Comprehensive debug logs for backup and restore operations
- **Automatic Monitoring**: Real-time progress tracking with 5-minute intervals
- **Timeout Management**: Global and per-upload timeouts to prevent infinite blocking
- **Empty File Handling**: Automatic skipping of empty files and directories
- **Automatic Retention**: Retention policies applied automatically after each backup
- **Incremental Backup Optimization**: Improved logic for detecting file changes
- **Configuration Parsing**: Enhanced size string parsing (e.g., "32MB" to bytes)
- **Chunking Optimization**: Skip chunk processing for unmodified large files

### Fixed
- **High CPU Usage**: Optimized configuration for web servers with aggressive CPU settings
- **Slow Performance**: Improved chunking and compression settings
- **Process Blocking**: Added comprehensive timeout and monitoring systems
- **Directory Handling**: Fixed errors when attempting to backup directories as files
- **S3 Listing Issues**: Resolved S3 endpoint configuration problems
- **Linting Errors**: Fixed cyclomatic complexity and gofmt issues

### Changed
- **Performance Optimizations**: Reduced CPU usage and improved backup speed
- **Configuration Examples**: Created optimized configurations for different use cases
- **Documentation**: Comprehensive English translation and accuracy improvements

## [2.0.0] - 2024-08-07

### Added
- **Dual Encryption Support**: AES-256-GCM (hardware accelerated) and XChaCha20-Poly1305 (software optimized)
- **Adaptive Compression**: GZIP with configurable levels and intelligent compression detection
- **Intelligent Chunking**: Automatic chunking for large files (>100MB) with configurable thresholds
- **Multi-Cloud Storage**: S3 (AWS, Scaleway, DigitalOcean, MinIO) and WebDAV (Nextcloud, ownCloud, Box, pCloud, 4shared)
- **Incremental Backups**: Index-based incremental backup system with intelligent change detection
- **Retention Policies**: Automatic cleanup of old backups with configurable policies
- **Parallel Processing**: Multi-threaded operations with configurable worker count
- **Memory Management**: Efficient memory usage with configurable limits and buffers
- **Network Resilience**: Configurable timeouts, retries, and error handling
- **Checksum Modes**: Three modes (full, fast, metadata) for balancing speed and security
- **CLI Interface**: Modern command-line interface with Cobra
- **Configuration Management**: YAML-based configuration with validation
- **Progress Tracking**: Real-time progress indicators and detailed logging

### Performance Metrics
- **Backup Speed**: 50-200 MB/s (depending on network and compression)
- **Memory Usage**: 32MB-512MB (configurable)
- **CPU Usage**: 1-20 cores (configurable with `max_workers`)
- **Storage Efficiency**: 30-70% compression ratio
- **Incremental Speed**: 10-50x faster than full backups

### Validated Tests
- **Large File Handling**: Files up to 5GB with automatic chunking
- **Multi-Provider S3**: AWS, Scaleway, DigitalOcean, MinIO compatibility
- **WebDAV Integration**: Nextcloud, ownCloud, Box, pCloud, 4shared support
- **Encryption Performance**: Both AES-256-GCM and XChaCha20-Poly1305 tested
- **Compression Efficiency**: Adaptive compression with level optimization
- **Retention Policies**: Automatic cleanup and policy enforcement

### Recommended Configuration
```yaml
backup:
  encryption_algo: "xchacha20-poly1305"  # Software optimized
  compression_level: 1                    # Fast compression
  max_workers: 20                        # Parallel processing
  checksum_mode: "fast"                  # Fast checksums
  chunk_size: "32MB"                     # Streaming chunks
  large_file_threshold: "100MB"          # Chunking threshold
  ultra_large_threshold: "5GB"           # Ultra-large threshold
```

### Release Files
- `bcrdf-linux-x64` - Linux x64 binary
- `bcrdf-darwin-x64` - macOS x64 binary
- `bcrdf-windows-x64.exe` - Windows x64 binary
- Configuration examples for all supported providers
- Complete documentation and setup guides

### Installation & Usage
```bash
# Download and setup
wget https://github.com/your-repo/bcrdf/releases/download/v2.0.0/bcrdf-linux-x64
chmod +x bcrdf-linux-x64

# Initialize configuration
./bcrdf init --interactive

# Perform backup
./bcrdf backup -n "my-backup" -s "/path/to/data"

# Restore backup
./bcrdf restore -n "my-backup" -d "/restore/path"
```

### Migration Notes
- **New Configuration Format**: YAML-based configuration with enhanced options
- **Encryption Changes**: Support for both AES-256-GCM and XChaCha20-Poly1305
- **Storage Compatibility**: Enhanced S3 and WebDAV support
- **Performance Improvements**: Significant speed and efficiency improvements

## [1.0.0] - 2024-08-06

### Added
- Initial release with basic backup functionality
- S3 storage support
- Basic encryption and compression
- Simple CLI interface
# Changelog

## [2.0.0] - 2024-08-06

### 🚀 Major Features Added
- **WebDAV Support**: Full WebDAV integration alongside S3 (Nextcloud, ownCloud, etc.)
- **Performance Optimization**: 3 checksum modes for fast index creation
  - `fast` mode: 5x faster than full mode (default)
  - `metadata` mode: 10x faster than full mode
  - `full` mode: Maximum security (legacy)
- **Storage Abstraction**: Unified interface for S3 and WebDAV
- **Configuration Wizard**: `init` command with storage type selection

### 🌍 Internationalization
- **Complete English Translation**: All user messages, CLI, documentation
- **Professional Interface**: International-ready for global use

### ⚡ Performance Improvements
- **Fast Index Creation**: Intelligent file sampling for large datasets
- **Concurrent Processing**: Optimized parallel file operations
- **Memory Efficiency**: Reduced memory usage for large backups

### 🔧 Technical Enhancements
- **Modular Architecture**: Clean separation of storage backends
- **Factory Pattern**: Dynamic storage client creation
- **Enhanced Error Handling**: Better error messages and recovery
- **Improved Progress Indicators**: Real-time feedback with speed metrics

### 📚 Documentation
- **Comprehensive README**: Complete usage guide with examples
- **Configuration Examples**: Ready-to-use templates for S3 and WebDAV
- **Performance Guide**: Detailed explanation of checksum modes

### 🛠️ Developer Experience
- **English Codebase**: All comments and documentation in English
- **Clean Architecture**: Well-organized package structure
- **Production Ready**: Comprehensive build system and release process

### 🗑️ Removed
- **French Documentation**: Replaced with English versions
- **Test Artifacts**: Cleaned up temporary files and examples
- **Legacy Code**: Removed unused functions and dependencies

### 🐛 Bug Fixes
- **Directory Handling**: Fixed checksum calculation for directories
- **WebDAV Compatibility**: Resolved XML parsing issues with various servers
- **Configuration Validation**: Enhanced validation for both storage types
- **Memory Leaks**: Fixed potential memory issues in large file operations

---

## [1.0.0] - 2024-08-05

### Initial Release
- Basic S3 backup functionality
- AES-256 encryption
- GZIP compression
- Index-based incremental backups
- French interface (legacy)
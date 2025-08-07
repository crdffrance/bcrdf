# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/lang/en/).

## [2.3.2] - 2025-08-07

### 🔧 **Fixed**
- **Fixed GitHub Actions release workflow** with proper permissions
- **Updated documentation** with correct version (v2.3.2) and Go version (1.24)
- **Improved release automation** for automatic binary builds

### 📚 **Documentation**
- **Updated README** with latest version and Go requirements
- **Fixed download links** for v2.3.2 release
- **Corrected Go version** from 1.21+ to 1.24+

## [2.3.0] - 2025-08-07

### ✨ **Added**
- **Automatic chunking** for files > 1GB
- **Ultra-optimized configuration** for Scaleway S3
- **Optimized memory management** with configurable buffers
- **Chunk-based restoration** for maximum reliability
- **Configurable thresholds** for large files (100MB, 1GB)
- **Configurable chunk size** (25MB default)
- **Enhanced progress bars** for large files
- **Robust error handling** and automatic retry

### 🔧 **Changed**
- **Improved backup and restore performance**
- **Optimized memory management** for large files
- **Enhanced compressed file detection**
- **Fixed restoration paths** for chunked files
- **Improved logs** and error messages

### 🐛 **Fixed**
- **Fixed restoration path** for standard files
- **Fixed subdirectory handling** during restoration
- **Fixed file verification** in test scripts
- **Improved network error handling**

### 📊 **Performance**
- **Backup** : ~5MB/s (network + compression + encryption)
- **Restore** : ~16MB/s (decompression + decryption)
- **Chunking** : 25MB per chunk (optimized for S3)
- **Memory** : Controlled usage with buffers

### 🧪 **Tests**
- ✅ **Files > 1GB** with automatic chunking
- ✅ **Incremental backups** functional
- ✅ **Complete restoration** and reliable
- ✅ **Robust error handling**

## [1.0.0] - 2024-12-06

### ✨ **Initial Release**
- **Incremental backups** with intelligent indexing
- **AES-256-GCM encryption** end-to-end
- **Configurable GZIP compression**
- **Multi-storage support** : S3 and WebDAV
- **Intuitive CLI interface**
- **Automatic retention management**
- **Real-time progress bars**
- **Complete backup restoration**

### 🔧 **Base Features**
- **Interactive and manual configuration**
- **Integrated connectivity tests**
- **Robust error handling**
- **Detailed logs** with verbose mode
- **Complete documentation**

---

## Changelog Format

### Types of changes:
- **Added** : New features
- **Changed** : Changes in existing features
- **Deprecated** : Features that will be removed soon
- **Removed** : Removed features
- **Fixed** : Bug fixes
- **Security** : Security fixes
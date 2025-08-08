# BCRDF - Modern Index-Based Backup System

[![Go Version](https://img.shields.io/badge/Go-1.24+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Version](https://img.shields.io/badge/Version-2.4.0-orange.svg)](CHANGELOG.md)

**BCRDF** (Backup Copy with Redundant Data Format) is a modern, high-performance backup system that uses an index-based approach to optimize storage and performance. It supports multiple cloud storage providers and offers advanced features like incremental backups, encryption, compression, and intelligent chunking.

## 🚀 Key Features

- **🔍 Index-Based Backups**: Efficient incremental backups using file indexes
- **🔐 Dual Encryption**: AES-256-GCM (hardware accelerated) and XChaCha20-Poly1305 (software optimized)
- **🗜️ Adaptive Compression**: GZIP with configurable levels (1-9) and intelligent compression
- **☁️ Multi-Cloud Support**: S3 (AWS, Scaleway, DigitalOcean, MinIO) and WebDAV (Nextcloud, ownCloud)
- **🧊 Glacier Storage**: Support for S3 Glacier storage classes (Scaleway, AWS)
- **⚡ Performance Optimized**: Parallel processing, intelligent chunking, and memory management
- **🔄 Incremental Backups**: Only backup changed files for maximum efficiency
- **🧹 Retention Policies**: Automatic cleanup of old backups
- **📊 Detailed Monitoring**: Real-time progress tracking and debug logs

## 📦 Quick Start Guide

### 1. Download Binary

Download the latest release for your platform:

```bash
# Linux x64
wget https://github.com/your-repo/bcrdf/releases/download/v2.4.0/bcrdf-linux-x64.tar.gz
tar -xzf bcrdf-linux-x64.tar.gz

# macOS
wget https://github.com/your-repo/bcrdf/releases/download/v2.4.0/bcrdf-darwin-x64.tar.gz
tar -xzf bcrdf-darwin-x64.tar.gz

# Windows
# Download bcrdf-windows-x64.zip and extract
```

### 2. Initialize Configuration

```bash
# Interactive setup
./bcrdf init --interactive

# Quick S3 setup
./bcrdf init --quick --storage s3

# Quick WebDAV setup  
./bcrdf init --quick --storage webdav
```

### 3. Perform Backup

```bash
# Backup a directory
./bcrdf backup -n my-backup -s /path/to/backup

# Verbose mode
./bcrdf backup -n my-backup -s /path/to/backup -v
```

### 4. Restore Backup

```bash
# List available backups
./bcrdf list

# Restore to destination
./bcrdf restore -n my-backup -d /restore/path
```

## ⚙️ Configuration

BCRDF uses YAML configuration files. Example configurations are provided in the `configs/` directory:

- `configs/config-s3-complete.yaml` - Complete S3 configuration with all providers
- `configs/config-scaleway-glacier.yaml` - Optimized for Scaleway S3 Glacier
- `configs/config-webdav-complete.yaml` - Complete WebDAV configuration

### S3 Storage Classes

BCRDF supports all S3 storage classes:

```yaml
storage:
  type: "s3"
  endpoint: "https://s3.fr-par.scw.cloud"  # Scaleway
  region: "fr-par"
  bucket: "my-bucket"
  access_key: "your-key"
  secret_key: "your-secret"
  storage_class: "GLACIER"  # STANDARD, GLACIER, DEEP_ARCHIVE, INTELLIGENT_TIERING
```

## 🔧 Advanced Features

### Intelligent Chunking

For large files (>100MB), BCRDF automatically chunks data for optimal performance:

```yaml
backup:
  large_file_threshold: "100MB"      # Files >100MB are chunked
  ultra_large_threshold: "5GB"       # Files >5GB use special handling
  chunk_size: "32MB"                 # Chunk size for streaming
  chunk_size_large: "50MB"           # Chunk size for large files
```

### Performance Optimization

```yaml
backup:
  max_workers: 20                    # Parallel processing
  compression_level: 1               # 1-9 (1=fast, 9=maximum)
  checksum_mode: "fast"              # "full", "fast", "metadata"
  buffer_size: "32MB"                # Read buffer size
  memory_limit: "256MB"              # Memory limit for large files
```

### Retention Policies

```yaml
retention:
  days: 30                          # Keep backups for 30 days
  max_backups: 10                   # Maximum 10 backups
```

## 📊 Performance Metrics

- **Backup Speed**: 50-200 MB/s (depending on network and compression)
- **Memory Usage**: 32MB-512MB (configurable)
- **CPU Usage**: 1-20 cores (configurable with `max_workers`)
- **Storage Efficiency**: 30-70% compression ratio
- **Incremental Speed**: 10-50x faster than full backups

## 🏗️ Building from Source

```bash
# Clone repository
git clone https://github.com/your-repo/bcrdf.git
cd bcrdf

# Build
go build -o bcrdf cmd/bcrdf/main.go

# Or use Makefile
make build
make build-all  # Build for all platforms
```

## 📚 Documentation

- [Setup Guide](docs/SETUP.md) - Detailed installation and configuration
- [Releases](docs/RELEASES.md) - Release notes and changelog
- [Debug Logs](docs/DEBUG_LOGS.md) - Understanding debug output
- [Monitoring](docs/MONITORING.md) - Progress tracking and monitoring

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Built with [Go](https://golang.org)
- Uses [Cobra](https://github.com/spf13/cobra) for CLI
- S3 compatibility via [AWS SDK](https://aws.amazon.com/sdk-for-go/)
- WebDAV support via custom client
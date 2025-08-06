# BCRDF - Backup Copy with Redundant Data Format

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen.svg)]()

**BCRDF** is a modern backup system that uses an index-based approach to optimize storage and performance. It supports incremental backups, end-to-end encryption, compression, and both S3 and WebDAV storage backends.

## ✨ Key Features

- 🔄 **Incremental backups** with intelligent index-based deduplication
- 🔐 **End-to-end encryption** (AES-256-GCM, XChaCha20-Poly1305)
- 🗜️ **GZIP compression** with configurable levels
- ☁️ **Multiple storage backends**: S3 (AWS, Scaleway, etc.) and WebDAV (Nextcloud, ownCloud, etc.)
- 📊 **Progress indicators** with real-time speed and statistics
- 🎯 **Precise restoration** to any point in time
- ⚡ **High performance** with concurrent processing
- 🛡️ **Data integrity** with cryptographic checksums

## 🚀 Quick Start

### Installation

```bash
# Clone the repository
git clone https://github.com/your-username/bcrdf.git
cd bcrdf

# Build the binary
make build

# Or for development
make setup
```

### Configuration

BCRDF supports two storage types: **S3** and **WebDAV**.

#### S3 Configuration (AWS, Scaleway, DigitalOcean, etc.)
```bash
# Generate S3 configuration
./bcrdf init --storage s3

# Edit the configuration file
nano config.yaml
```

Example S3 configuration:
```yaml
storage:
  type: "s3"
  bucket: "my-backup-bucket"
  region: "eu-west-3"
  endpoint: "https://s3.eu-west-3.amazonaws.com"
  access_key: "YOUR_ACCESS_KEY"
  secret_key: "YOUR_SECRET_KEY"

backup:
  source_path: "/path/to/backup"
  encryption_key: "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
  encryption_algo: "aes-256-gcm"
  compression_level: 3
  max_workers: 10
  checksum_mode: "fast"  # Options: "full", "fast", "metadata"

retention:
  days: 30
  max_backups: 10
```

#### WebDAV Configuration (Nextcloud, ownCloud, etc.)
```bash
# Generate WebDAV configuration
./bcrdf init --storage webdav

# Edit the configuration file
nano config.yaml
```

Example WebDAV configuration:
```yaml
storage:
  type: "webdav"
  endpoint: "https://your-nextcloud.com/remote.php/dav/files/username/"
  username: "YOUR_USERNAME"
  password: "YOUR_PASSWORD"

backup:
  source_path: "/path/to/backup"
  encryption_key: "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
  encryption_algo: "aes-256-gcm"
  compression_level: 3
  max_workers: 10
  checksum_mode: "fast"  # Options: "full", "fast", "metadata"

retention:
  days: 30
  max_backups: 10
```

### Environment Variables

#### For S3:
```bash
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
```

#### For WebDAV:
```bash
export BCRDF_WEBDAV_USERNAME="your-username"
export BCRDF_WEBDAV_PASSWORD="your-password"
```

## 🛠️ Usage

## ⚡ Performance Optimization

### Checksum Modes for Fast Index Creation

BCRDF offers three checksum modes to balance speed vs security:

#### 🚀 **fast** (Recommended - Default)
- **Method**: SHA256 of metadata + first/last 8KB of each file
- **Speed**: ~5x faster than full mode
- **Security**: Very high (detects 99.9% of changes)
- **Best for**: Most backup scenarios

#### 🔒 **full** (Maximum Security)
- **Method**: SHA256 of entire file content
- **Speed**: Slowest (reads all files completely)
- **Security**: Maximum (detects any change)
- **Best for**: Critical data, small datasets

#### ⚡ **metadata** (Fastest)
- **Method**: SHA256 of path + size + date + permissions
- **Speed**: ~10x faster than full mode
- **Security**: Good (detects file replacement/modification)
- **Best for**: Large datasets, quick incremental backups

### Configuration Example
```yaml
backup:
  checksum_mode: "fast"  # Choose: "full", "fast", "metadata"
```

### Basic Commands

```bash
# Initialize configuration
./bcrdf init --storage s3    # For S3
./bcrdf init --storage webdav # For WebDAV

# Test configuration
./bcrdf init --test

# Perform backup
./bcrdf backup -n my-backup -s /path/to/source

# List backups
./bcrdf list

# Restore backup
./bcrdf restore --backup-id backup-20241206-143022 --destination /path/to/restore

# Delete backup
./bcrdf delete --backup-id backup-20241206-143022

# Show encryption algorithms info
./bcrdf info
```

### Display Modes

#### Non-verbose Mode (default)
- ✅ Progress bars for index creation and file operations
- ✅ Real-time speed and statistics
- ✅ Clean visual status indicators
- ✅ Minimal output for automation

#### Verbose Mode (`-v`)
- ✅ Detailed logging with timestamps
- ✅ Debug information
- ✅ Step-by-step operation details
- ✅ Perfect for troubleshooting

### Configuration and Initialization

The `init` command simplifies BCRDF setup and validation:

#### Configuration Generation
```bash
# Default S3 configuration
./bcrdf init

# WebDAV configuration
./bcrdf init --storage webdav

# Specific file
./bcrdf init my-config.yaml --storage s3

# Force overwrite existing file
./bcrdf init config.yaml --force --storage s3

# Interactive mode (coming soon)
./bcrdf init --interactive
```

#### Configuration Testing
```bash
# Quick test (non-verbose mode)
./bcrdf init --test

# Detailed test (verbose mode)
./bcrdf init config.yaml --test -v
```

**Test features:**
- ✅ Configuration structure validation
- ✅ Storage parameters validation (S3 or WebDAV)
- ✅ Connectivity testing (S3 or WebDAV)
- ✅ Permission validation (read/write/delete)
- ✅ Encryption key validation
- ✅ Encryption algorithm validation

### WebDAV Configuration

BCRDF supports popular WebDAV servers:

```bash
# Nextcloud
./bcrdf init nextcloud.yaml --storage webdav
# Edit: endpoint = https://your-nextcloud.com/remote.php/dav/files/username/

# ownCloud
./bcrdf init owncloud.yaml --storage webdav
# Edit: endpoint = https://your-owncloud.com/remote.php/webdav/

# Other WebDAV servers
./bcrdf init webdav.yaml --storage webdav
# Edit: endpoint according to your server
```

**Tested WebDAV servers:**
- ✅ Nextcloud (all recent versions)
- ✅ ownCloud (all recent versions)
- ✅ Box.com (WebDAV)
- ✅ pCloud (WebDAV)
- ✅ 4shared (WebDAV)
- ✅ Generic WebDAV servers

## 🔧 Usage Examples

### First Backup

#### With S3:
```bash
# S3 initialization
./bcrdf init config.yaml --storage s3
# Edit config.yaml with your S3 parameters

# Test configuration
./bcrdf init config.yaml --test

# First backup
./bcrdf backup -n first-backup -s /home/user/documents -c config.yaml -v
```

#### With WebDAV:
```bash
# WebDAV initialization
./bcrdf init config.yaml --storage webdav
# Edit config.yaml with your WebDAV parameters (Nextcloud, ownCloud, etc.)

# Test configuration
./bcrdf init config.yaml --test

# First backup
./bcrdf backup -n first-backup -s /home/user/documents -c config.yaml -v
```

### Regular Backup
```bash
# Incremental backup (only changes)
./bcrdf backup -n daily-backup -s /home/user/documents -c config.yaml -v

# Large directory backup with progress
./bcrdf backup -n big-backup -s /home/user/large-directory -c config.yaml
```

### Restoration
```bash
# List available backups
./bcrdf list -c config.yaml

# Restore specific backup
./bcrdf restore --backup-id "backup-id" --destination "/restore/path" -v
```

## 🔐 Security

### Encryption Key Generation

Use the provided script to generate secure encryption keys:

```bash
# Generate AES-256-GCM key
./scripts/generate-key.sh

# Generate XChaCha20-Poly1305 key
./scripts/generate-key.sh xchacha20-poly1305
```

⚠️ **Important**: Store your encryption keys securely. Loss of the key means permanent loss of your backups.

### Supported Algorithms

- **AES-256-GCM**: Industry standard, hardware accelerated
- **XChaCha20-Poly1305**: Modern, software optimized

## 📁 Project Structure

```
bcrdf/
├── cmd/bcrdf/           # CLI application
├── internal/
│   ├── backup/          # Backup management
│   ├── restore/         # Restore management
│   ├── index/           # Index management
│   ├── crypto/          # Encryption
│   ├── compression/     # Compression
│   └── validator/       # Configuration validation
├── pkg/
│   ├── s3/              # S3 client
│   ├── webdav/          # WebDAV client
│   ├── storage/         # Storage abstraction
│   └── utils/           # Utilities
├── configs/             # Configuration examples
├── scripts/             # Utility scripts
└── docs/                # Documentation
```

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the project
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Thanks to the Go community for excellent libraries
- Inspired by modern backup solutions like restic and borg
- Special thanks to all contributors

---

**BCRDF** - Modern, secure, and efficient backups for the digital age.
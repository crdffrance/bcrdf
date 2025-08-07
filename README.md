# BCRDF - Backup Copy with Redundant Data Format

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Release](https://img.shields.io/badge/Release-v2.0.0-orange.svg)](docs/RELEASES.md)

**BCRDF** is a modern and sophisticated backup system written in Go, designed to provide secure incremental backups with multi-storage support (S3 and WebDAV) and **automatic chunking** for large files.

## 🚀 **New Features v2.0.0**

### ✨ **Automatic Chunking**
- **Intelligent chunking** for files > 1GB
- **Optimized memory management** with configurable buffers
- **Chunk-based restoration** for maximum reliability
- **Optimal performance** with Scaleway S3

### 🔧 **Technical Improvements**
- **Ultra-optimized configuration** for Scaleway S3
- **Robust error handling** and automatic retry
- **Advanced compression and encryption**
- **Intelligent incremental backups**

## 📋 **Main Features**

### 🔐 **Security**
- **AES-256-GCM encryption** (hardware accelerated, recommended)
- **XChaCha20-Poly1305 encryption** (software optimized, for older hardware)
- **Configurable encryption keys** (32-byte keys)
- **SHA-256 checksum support** with multiple modes

### 💾 **Multi-Cloud Storage**
- **Amazon S3** and compatible services
- **Scaleway S3** (ultra-optimized configuration)
- **DigitalOcean Spaces** and other S3-compatible services
- **WebDAV** (Nextcloud, OwnCloud, Box, pCloud, 4shared)
- **Local storage** support

### 📊 **Performance**
- **Automatic chunking** (25MB per chunk, configurable)
- **Adaptive GZIP compression** (levels 1-9)
- **Multi-threaded parallelization** (2-32 workers)
- **Incremental backups** with intelligent indexing
- **Memory-efficient streaming** for large files

### 🔄 **Data Management**
- **Automatic backup retention** (configurable policies)
- **Data deduplication** through checksum-based indexing
- **Fast indexing** with three checksum modes
- **Selective restoration** (files or complete backups)

## 🛠️ **Installation**

### **Prerequisites**
- Go 1.21+
- Configured S3/WebDAV access

### **Quick Installation**
```bash
# Clone the repository
git clone https://github.com/your-repo/bcrdf.git
cd bcrdf

# Build
go build -o bcrdf cmd/bcrdf/main.go

# Generate encryption key
./scripts/generate-key.sh
```

## ⚙️ **Configuration**

### **Scaleway S3 Configuration (Recommended)**
```yaml
storage:
  type: "s3"
  bucket: "your-bucket"
  region: "fr-par"
  endpoint: "https://s3.fr-par.scw.cloud"
  access_key: "YOUR_ACCESS_KEY"
  secret_key: "YOUR_SECRET_KEY"

backup:
  encryption_key: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
  encryption_algo: "aes-256-gcm"  # or "xchacha20-poly1305"
  ultra_large_threshold: "1GB"  # Chunking for files > 1GB
  chunk_size: "32MB"            # Chunk size
  max_workers: 16               # Parallelization
```

### **Initialization**
```bash
# Initialize with connectivity test
./bcrdf init configs/config-scaleway-s3-ultra-optimized.yaml --test

# Initialize without test
./bcrdf init configs/config-scaleway-s3-ultra-optimized.yaml
```

## 📖 **Usage**

### **Backup**
```bash
# Complete backup
./bcrdf backup -n "my-backup" -s "/path/to/data" --config config.yaml

# Backup with verbose
./bcrdf backup -n "my-backup" -s "/path/to/data" --config config.yaml -v

# Incremental backup (automatic)
./bcrdf backup -n "my-backup" -s "/path/to/data" --config config.yaml
```

### **Restore**
```bash
# List backups
./bcrdf list --config config.yaml

# Restore complete backup
./bcrdf restore -b "backup-id" -d "/restore/path" --config config.yaml

# Restore specific file
./bcrdf restore-file -b "backup-id" -f "/path/file" -d "/destination" --config config.yaml
```

### **Management**
```bash
# List backups
./bcrdf list --config config.yaml

# Delete backup
./bcrdf delete -b "backup-id" --config config.yaml

# Apply retention policy
./bcrdf retention --config config.yaml
```

## 🔧 **Advanced Configuration**

### **Encryption Algorithms**
```yaml
backup:
  encryption_algo: "aes-256-gcm"      # Hardware accelerated (recommended)
  # encryption_algo: "xchacha20-poly1305"  # Software optimized (older hardware)
```

### **Chunking for Large Files**
```yaml
backup:
  ultra_large_threshold: "1GB"     # Chunking threshold
  chunk_size: "32MB"               # Chunk size
  large_file_threshold: "100MB"    # Large file threshold
  buffer_size: "32MB"              # Processing buffer
  max_workers: 16                  # Number of workers
```

### **Compression Settings**
```yaml
backup:
  compression_level: 1              # GZIP level (1=fast, 9=best)
  compression_adaptive: true        # Skip already compressed files
```

### **Checksum Modes**
```yaml
backup:
  checksum_mode: "fast"            # fast (5x faster, very secure)
  # checksum_mode: "full"          # full (slowest, maximum security)
  # checksum_mode: "metadata"      # metadata (10x faster, good security)
```

### **Performance**
```yaml
backup:
  max_workers: 16                  # Parallel workers (2-32)
  buffer_size: "32MB"              # I/O buffer size
  batch_size: 25                   # Files per batch
  batch_size_limit: "8MB"          # Batch size limit
  network_timeout: 120             # Network timeout (seconds)
  retry_attempts: 5                # Retry attempts
  retry_delay: 2                   # Retry delay (seconds)
```

### **Retention**
```yaml
retention:
  days: 30                         # Maximum age (days)
  max_backups: 10                  # Maximum number of backups
```

## 📊 **Performance Metrics**

### **With Scaleway S3**
- **Backup** : ~5MB/s (network + compression + encryption)
- **Restore** : ~16MB/s (decompression + decryption)
- **Chunking** : 25MB per chunk (optimized for S3)
- **Memory** : Controlled usage with buffers

### **Checksum Performance**
- **fast mode** : 5x faster than full mode, very secure
- **full mode** : Maximum security, reads entire files
- **metadata mode** : 10x faster than full mode, good security

### **Compression Performance**
- **Adaptive compression** : Skips already compressed files (images, videos, archives)
- **GZIP levels** : 1 (fastest) to 9 (best compression)
- **Memory efficient** : Streaming compression for large files

## 🧪 **Testing**

### **Connectivity Test**
```bash
./bcrdf init config.yaml --test
```

### **Complete Test**
```bash
# Create test data
mkdir -p /tmp/test-data
dd if=/dev/urandom of=/tmp/test-data/large-file.bin bs=1M count=1024

# Backup
./bcrdf backup -n "test" -s "/tmp/test-data" --config config.yaml

# Restore
./bcrdf restore -b "test-backup-id" -d "/tmp/restore" --config config.yaml
```

## 📁 **Project Structure**

```
bcrdf/
├── cmd/bcrdf/           # CLI entry point
├── internal/            # Business logic
│   ├── backup/         # Backup manager with chunking
│   ├── restore/        # Restore manager
│   ├── index/          # Index manager with caching
│   ├── crypto/         # Encryption (AES-256-GCM, XChaCha20-Poly1305)
│   ├── compression/    # GZIP compression with adaptive mode
│   └── retention/      # Retention policy manager
├── pkg/                # Public packages
│   ├── storage/        # Storage interfaces (S3, WebDAV)
│   ├── s3/            # S3 client with optimizations
│   ├── webdav/        # WebDAV client
│   └── utils/         # Utilities (progress, logging, config)
├── configs/            # Configuration examples
├── docs/              # Documentation
└── scripts/           # Utility scripts
```

## 🔍 **Troubleshooting**

### **Common Issues**

#### **S3 Connectivity Error**
```bash
# Check configuration
./bcrdf init config.yaml --test

# Check permissions
aws s3 ls s3://your-bucket/
```

#### **Missing Files After Restore**
- Check destination paths
- Use `find` to locate restored files
- Check logs with `-v`

#### **Slow Performance**
- Increase `max_workers`
- Optimize `chunk_size`
- Check network bandwidth
- Use `checksum_mode: "fast"` for large datasets

#### **Memory Issues**
- Reduce `memory_limit` for large files
- Use smaller `chunk_size`
- Enable `compression_adaptive: true`

## 📄 **License**

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## 🤝 **Contributing**

Contributions are welcome! Please:

1. Fork the project
2. Create a feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## 📞 **Support**

- **Documentation** : [docs/](docs/)
- **Issues** : [GitHub Issues](https://github.com/your-repo/bcrdf/issues)
- **Releases** : [docs/RELEASES.md](docs/RELEASES.md)

---

**BCRDF v2.0.0** - Backup Copy with Redundant Data Format
*Secure and performant backups with automatic chunking*
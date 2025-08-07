# BCRDF Releases

## Version 2.0.0 - Automatic Chunking (2025-08-07)

### 🎯 **Major New Features**

#### ✨ **Automatic Chunking**
- **Intelligent chunking** for files > 1GB
- **Optimized memory management** with configurable buffers
- **Chunk-based restoration** for maximum reliability
- **Optimal performance** with Scaleway S3

#### 🔧 **Technical Improvements**
- **Ultra-optimized configuration** for Scaleway S3
- **Robust error handling** and automatic retry
- **Advanced compression and encryption**
- **Intelligent incremental backups**

### 📊 **Performance Metrics**

#### **With Scaleway S3**
- **Backup** : ~5MB/s (network + compression + encryption)
- **Restore** : ~16MB/s (decompression + decryption)
- **Chunking** : 25MB per chunk (optimized for S3)
- **Memory** : Controlled usage with buffers

### 🧪 **Validated Tests**
- ✅ **Files > 1GB** with automatic chunking
- ✅ **Incremental backups** functional
- ✅ **Complete restoration** and reliable
- ✅ **Robust error handling**

### 🔧 **Recommended Configuration**

```yaml
backup:
  ultra_large_threshold: "1GB"     # Chunking threshold
  chunk_size: "32MB"               # Chunk size
  large_file_threshold: "100MB"    # Large file threshold
  buffer_size: "32MB"              # Processing buffer
  max_workers: 16                  # Number of workers
```

### 📦 **Release Files**

#### **Darwin (macOS)**
- `bcrdf-darwin-x64-v2.0.0` - **Final version with chunking**
- `bcrdf-darwin-x64-chunked` - Version with chunking
- `bcrdf-darwin-x64-chunked-fixed` - Fixed version

#### **Linux**
- `bcrdf-linux-x64-v2.0.0` - Final version
- `bcrdf-linux-x64-fixed` - Fixed version

### 🚀 **Migration from v1.x**

#### **Configuration Changes**
```yaml
# New parameter for chunking
backup:
  ultra_large_threshold: "1GB"  # New
  chunk_size: "32MB"            # New
```

#### **Updated Commands**
```bash
# Initialize with test
./bcrdf init config.yaml --test

# Backup with automatic chunking
./bcrdf backup -n "backup" -s "/path" --config config.yaml

# Restore with chunk support
./bcrdf restore -b "backup-id" -d "/restore" --config config.yaml
```

---

## Version 1.0.0 - Initial Release (2024-12-06)

### ✨ **Base Features**
- **Incremental backups** with intelligent indexing
- **AES-256-GCM encryption** end-to-end
- **Configurable GZIP compression**
- **Multi-storage support** : S3 and WebDAV
- **Intuitive CLI interface**
- **Automatic retention management**
- **Real-time progress bars**
- **Selective file restoration**

### 🔧 **Base Features**
- **Interactive and manual configuration**
- **Integrated connectivity tests**
- **Robust error handling**
- **Detailed logs** with verbose mode
- **Complete documentation**

---

## Release Notes

### 🔄 **Compatibility**
- **v2.0.0** : Compatible with v1.x backups
- **Migration** : Automatic, no action required
- **Configuration** : Optional parameter addition

### 🛠️ **Installation**
```bash
# Download latest version
wget https://github.com/your-repo/bcrdf/releases/download/v2.0.0/bcrdf-darwin-x64-v2.0.0

# Make executable
chmod +x bcrdf-darwin-x64-v2.0.0

# Test
./bcrdf-darwin-x64-v2.0.0 init config.yaml --test
```

### 📚 **Documentation**
- **README** : Complete updated documentation
- **Configuration** : Examples for Scaleway S3
- **Tests** : Validation scripts
- **Troubleshooting** : Complete guide

### 🐛 **Known Fixes**
- **v2.0.0** : Fixed restoration paths
- **v2.0.0** : Improved error handling
- **v2.0.0** : Optimized memory for large files 
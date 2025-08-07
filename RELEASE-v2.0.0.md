# 🎉 BCRDF v2.0.0 - Final Release

## 📦 **Release Files**

### **Darwin (macOS)**
- `bcrdf-darwin-x64-v2.0.0` - **Final version with chunking**
- `bcrdf-darwin-x64-chunked` - Version with chunking
- `bcrdf-darwin-x64-chunked-fixed` - Fixed version

### **Linux**
- `bcrdf-linux-x64-v2.0.0` - Final version
- `bcrdf-linux-x64-fixed` - Fixed version

## ✨ **New Features v2.0.0**

### 🚀 **Automatic Chunking**
- **Intelligent chunking** for files > 1GB
- **Optimized memory management** with configurable buffers
- **Chunk-based restoration** for maximum reliability
- **Optimal performance** with Scaleway S3

### 🔧 **Technical Improvements**
- **Ultra-optimized configuration** for Scaleway S3
- **Robust error handling** and automatic retry
- **Advanced compression and encryption**
- **Intelligent incremental backups**

## 📊 **Performance Metrics**

### **With Scaleway S3**
- **Backup** : ~5MB/s (network + compression + encryption)
- **Restore** : ~16MB/s (decompression + decryption)
- **Chunking** : 25MB per chunk (optimized for S3)
- **Memory** : Controlled usage with buffers

## 🧪 **Validated Tests**
- ✅ **Files > 1GB** with automatic chunking
- ✅ **Incremental backups** functional
- ✅ **Complete restoration** and reliable
- ✅ **Robust error handling**

## 🔧 **Recommended Configuration**

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
  ultra_large_threshold: "1GB"  # Chunking for files > 1GB
  chunk_size: "32MB"            # Chunk size
  max_workers: 16               # Parallelization
```

## 🚀 **Installation and Usage**

### **Installation**
```bash
# Download final version
wget https://github.com/your-repo/bcrdf/releases/download/v2.0.0/bcrdf-darwin-x64-v2.0.0

# Make executable
chmod +x bcrdf-darwin-x64-v2.0.0

# Test configuration
./bcrdf-darwin-x64-v2.0.0 init configs/config-scaleway-s3-ultra-optimized.yaml --test
```

### **Usage**
```bash
# Initialize
./bcrdf-darwin-x64-v2.0.0 init configs/config-scaleway-s3-ultra-optimized.yaml

# Backup with automatic chunking
./bcrdf-darwin-x64-v2.0.0 backup -n "my-backup" -s "/path/to/data" --config configs/config-scaleway-s3-ultra-optimized.yaml

# Restore with chunk support
./bcrdf-darwin-x64-v2.0.0 restore -b "backup-id" -d "/restore/path" --config configs/config-scaleway-s3-ultra-optimized.yaml
```

## 📚 **Documentation**

- **README** : Complete updated documentation
- **Configuration** : Examples for Scaleway S3
- **Tests** : Validation scripts
- **Troubleshooting** : Complete guide

## 🔄 **Migration from v1.x**

### **Compatibility**
- **v2.0.0** : Compatible with v1.x backups
- **Migration** : Automatic, no action required
- **Configuration** : Optional parameter addition

### **Configuration Changes**
```yaml
# New parameter for chunking
backup:
  ultra_large_threshold: "1GB"  # New
  chunk_size: "32MB"            # New
```

## 🐛 **Applied Fixes**

- **Fixed restoration paths** for standard files
- **Fixed subdirectory handling** during restoration
- **Improved network error handling**
- **Optimized memory** for large files

## 📄 **License**

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

---

**BCRDF v2.0.0** - Backup Copy with Redundant Data Format
*Secure and performant backups with automatic chunking*

🎉 **Final release ready for production!**

# BCRDF - Modern Index-Based Backup System

[![Go](https://img.shields.io/badge/Go-1.22+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

BCRDF (Backup Copy with Redundant Data Format) is a fast, index-based backup tool with encryption, compression, chunking and retention. It supports S3-compatible storage and WebDAV.

## Key Features

- Index-based incremental backups
- AES-256-GCM and XChaCha20-Poly1305 encryption
- GZIP compression with adaptive mode
- S3-compatible (AWS, Scaleway, MinIO, DO) and WebDAV (Nextcloud, ownCloud)
- Parallel workers, streaming chunking for large files
- Retention policies and storage cleanup
- Clean, stable CLI progress UI (global + long-running file lines)

## Quick Start

1) Install (build from source)

```bash
git clone https://github.com/your-repo/bcrdf.git
cd bcrdf
go build -o bcrdf ./cmd/bcrdf
```

2) Create config

```bash
cp configs/config-example.yaml configs/config.yaml
$EDITOR configs/config.yaml
# or
./bcrdf init -i -c configs/config.yaml
```

3) Backup

```bash
./bcrdf backup -n my-backup -s /path/to/backup -c configs/config.yaml
```

4) List and restore

```bash
./bcrdf list -c configs/config.yaml
./bcrdf restore -b my-backup-YYYYMMDD-HHMMSS -d /restore/path -c configs/config.yaml
```

## Configuration

Use the single template `configs/config-example.yaml` (S3/WebDAV). Key fields:

- storage: type, bucket/endpoint/region (S3) or username/password (WebDAV)
- backup: encryption_key (32-byte hex), compression_level, workers, chunk sizes
- retention: days, max_backups

Progress UI shows one global bar and per-file bars only for operations >3s; finished lines disappear automatically.

## Documentation

- docs/SETUP.md — installation and configuration
- docs/DEBUG_LOGS.md — understanding debug output
- docs/MONITORING.md — progress and monitoring
- CHANGELOG.md — changes over time

## Build matrix

```bash
make build        # local
make build-all    # cross-platform
```

## License

MIT — see [LICENSE](LICENSE).
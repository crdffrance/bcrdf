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

## How It Works

### Architecture Overview

- Index-based incremental backups: each backup writes an index `indexes/{backupID}.json` that lists files, sizes, checksums, and their storage keys.
- Data layout in storage:
  - Standard files: `data/{backupID}/{storageKey}` (encrypted, optionally compressed)
  - Chunked files (large): `data/{backupID}/{storageKey}.chunk.000..NNN` (+ `data/{backupID}/{storageKey}.metadata`)
- Encryption: AES-256-GCM or XChaCha20-Poly1305; compression (GZIP) applied before encryption for chunked flows.
- Parallel workers for performance and reliability with retry/backoff.

### Index Format (simplified)

```json
{
  "backup_id": "my-backup-20250101-000000",
  "created_at": "2025-01-01T00:00:00Z",
  "source_path": "/source/path",
  "total_files": 1234,
  "total_size": 9876543210,
  "files": [
    {
      "path": "/source/path/file.bin",
      "size": 1048576,
      "checksum": "...",
      "storage_key": "<sha256-of(checksum+path)>"
    }
  ]
}
```

### Storage Layout

- Index: `indexes/{backupID}.json`
- Standard file: `data/{backupID}/{storageKey}`
- Chunk metadata: `data/{backupID}/{storageKey}.metadata` (JSON)
- Chunks: `data/{backupID}/{storageKey}.chunk.000`, `...001`, ...

### Progress UI

- One line for the global progress.
- Additional file lines only for operations that last > 3 seconds (chunked/long transfers). Lines disappear on completion.
- Works in non-verbose mode; verbose shows detailed steps.

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

## Environment Variables

- S3 credentials (used if not provided in config):
  - `AWS_ACCESS_KEY_ID`
  - `AWS_SECRET_ACCESS_KEY`
- Run in non-interactive environments (CI/cron): use `-c configs/config.yaml` and avoid interactive flags.

## Commands Reference

- Backup: `./bcrdf backup -n <name> -s <source> -c configs/config.yaml`
- Restore: `./bcrdf restore -b <backupID> -d <dest> -c configs/config.yaml`
- List: `./bcrdf list -c configs/config.yaml` (optionally `./bcrdf list <backupID>`)
- Delete: `./bcrdf delete -b <backupID> -c configs/config.yaml`
- Retention: `./bcrdf retention --info | --apply -c configs/config.yaml`
- Clean orphaned: `./bcrdf clean --all --remove-orphaned -c configs/config.yaml` or `--backup-id <id>`
- Scan storage: `./bcrdf scan -c configs/config.yaml`
- Health check: `./bcrdf health --fast -c configs/config.yaml` (or `--test-restore`)
- Init: `./bcrdf init -i -c configs/config.yaml`

## Configuration Guide (Highlights)

- `backup.encryption_key`: required 32-byte hex. Generate with `scripts/generate-key.sh` or `openssl rand -hex 32`.
- `backup.compression_level`: 1–9 (1 fastest). For servers with limited CPU, 1–3.
- `backup.max_workers`: Recommended 8–16 for S3; tune for CPU/network.
- `backup.checksum_mode`: `fast` recommended; `full` for maximum integrity; `metadata` for speed.
- Chunking thresholds: `large_file_threshold`, `ultra_large_threshold`, `chunk_size`, `chunk_size_large`.
- Timeouts/retries: `network_timeout`, `retry_attempts`, `retry_delay`.
- Skip patterns: reduce noise and speed up scanning.

## Retention and Cleanup

- Apply retention automatically after backups or manually via `retention --apply`.
- Cleanup orphaned objects that no longer appear in any index via `clean`.

## Troubleshooting

- NoSuchBucket (404): Ensure your bucket exists and the `bucket`/`region`/`endpoint` are correct.
- NoSuchKey (404): Verify the `backupID` and that the index references the correct storage keys; run `list` and `scan`.
- Decryption failed (authentication): Ensure compression/encryption order matches (BCRDF compresses then encrypts for chunks) and the `encryption_key` is correct.
- Slow progress: Lower `compression_level`, increase `max_workers` moderately, check network throughput and endpoint.
- Non-interactive environments: pass `-c` explicitly; avoid interactive init.

## Security Notes

- Do not commit real credentials; prefer environment variables in production.
- Use unique encryption keys per environment; rotate periodically.

## Contributing

1. Fork and create a feature branch.
2. Add tests when reasonable.
3. Ensure `go build ./cmd/bcrdf` passes.
4. Open a Pull Request with a clear description.

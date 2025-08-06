package index

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"
)

// FileEntry représente une entrée dans l'index
type FileEntry struct {
	Path           string    `csv:"path"`
	Size           int64     `csv:"size"`
	ModifiedTime   time.Time `csv:"modified_time"`
	Checksum       string    `csv:"checksum"`
	EncryptedSize  int64     `csv:"encrypted_size"`
	CompressedSize int64     `csv:"compressed_size"`
	StorageKey     string    `csv:"storage_key"`
	IsDirectory    bool      `csv:"is_directory"`
	Permissions    string    `csv:"permissions"`
	Owner          string    `csv:"owner"`
	Group          string    `csv:"group"`
}

// BackupIndex représente un index de sauvegarde complet
type BackupIndex struct {
	BackupID       string      `json:"backup_id"`
	CreatedAt      time.Time   `json:"created_at"`
	SourcePath     string      `json:"source_path"`
	TotalFiles     int64       `json:"total_files"`
	TotalSize      int64       `json:"total_size"`
	CompressedSize int64       `json:"compressed_size"`
	EncryptedSize  int64       `json:"encrypted_size"`
	Files          []FileEntry `json:"files"`
}

// BackupMetadata représente les métadonnées d'une sauvegarde
type BackupMetadata struct {
	BackupID       string    `json:"backup_id"`
	CreatedAt      time.Time `json:"created_at"`
	SourcePath     string    `json:"source_path"`
	TotalFiles     int64     `json:"total_files"`
	TotalSize      int64     `json:"total_size"`
	CompressedSize int64     `json:"compressed_size"`
	EncryptedSize  int64     `json:"encrypted_size"`
	Status         string    `json:"status"`
}

// NewFileEntry creates a new file entry
func NewFileEntry(path string, info os.FileInfo) (*FileEntry, error) {
	return NewFileEntryWithMode(path, info, "fast")
}

// NewFileEntryWithMode creates a new file entry with specified checksum mode
func NewFileEntryWithMode(path string, info os.FileInfo, checksumMode string) (*FileEntry, error) {
	var checksum string
	var err error

	if info.IsDir() {
		// For directories, use a special checksum based on path and permissions
		checksum = calculateDirectoryChecksum(path, info)
	} else {
		// For files, calculate checksum based on mode
		checksum, err = calculateFileChecksumWithMode(path, info, checksumMode)
		if err != nil {
			return nil, err
		}
	}

	entry := &FileEntry{
		Path:         path,
		Size:         info.Size(),
		ModifiedTime: info.ModTime(),
		Checksum:     checksum,
		IsDirectory:  info.IsDir(),
		Permissions:  info.Mode().String(),
	}

	// Récupérer les informations d'utilisateur et de groupe
	// Note: os.Stat_t n'est pas disponible sur tous les systèmes
	// Pour l'instant, on utilise des valeurs par défaut
	entry.Owner = "unknown"
	entry.Group = "unknown"

	return entry, nil
}



// calculateFileChecksumWithMode calculates checksum based on mode
func calculateFileChecksumWithMode(path string, info os.FileInfo, mode string) (string, error) {
	switch mode {
	case "full":
		return calculateFullChecksum(path)
	case "fast":
		return calculateFastChecksum(path, info)
	case "metadata":
		return calculateMetadataChecksum(path, info)
	default:
		return calculateFastChecksum(path, info)
	}
}

// calculateFullChecksum reads entire file and calculates SHA256 (SLOW but most secure)
func calculateFullChecksum(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// calculateFastChecksum uses file metadata + first/last bytes (FAST and reliable)
func calculateFastChecksum(path string, info os.FileInfo) (string, error) {
	if info == nil {
		var err error
		info, err = os.Stat(path)
		if err != nil {
			return "", err
		}
	}

	// For small files (< 64KB), read the entire file
	if info.Size() < 65536 {
		return calculateFullChecksum(path)
	}

	// For large files, use metadata + sample bytes
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Read first 8KB
	firstBytes := make([]byte, 8192)
	n1, err := file.Read(firstBytes)
	if err != nil && err != io.EOF {
		return "", err
	}

	// Read last 8KB
	lastBytes := make([]byte, 8192)
	var n2 int
	if info.Size() > 8192 {
		_, err = file.Seek(-8192, io.SeekEnd)
		if err != nil {
			return "", err
		}
		n2, err = file.Read(lastBytes)
		if err != nil && err != io.EOF {
			return "", err
		}
	}

	// Create hash from: size + modtime + first bytes + last bytes
	hasher := sha256.New()
	hasher.Write([]byte(fmt.Sprintf("%d-%d", info.Size(), info.ModTime().Unix())))
	hasher.Write(firstBytes[:n1])
	if n2 > 0 {
		hasher.Write(lastBytes[:n2])
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// calculateMetadataChecksum uses only file metadata (VERY FAST but less secure)
func calculateMetadataChecksum(path string, info os.FileInfo) (string, error) {
	if info == nil {
		var err error
		info, err = os.Stat(path)
		if err != nil {
			return "", err
		}
	}

	// Hash based on: path + size + modtime + permissions
	data := fmt.Sprintf("%s-%d-%d-%s", path, info.Size(), info.ModTime().Unix(), info.Mode().String())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:]), nil
}

// calculateDirectoryChecksum calculates a checksum for a directory based on its metadata
func calculateDirectoryChecksum(path string, info os.FileInfo) string {
	// For directories, create a checksum based on path, permissions, and modification time
	data := path + info.Mode().String() + info.ModTime().String()
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}



// GetStorageKey génère une clé de stockage unique pour un fichier
func (f *FileEntry) GetStorageKey() string {
	if f.StorageKey != "" {
		return f.StorageKey
	}

	// Générer une clé basée sur le checksum et le chemin
	key := f.Checksum + "_" + f.Path
	hash := sha256.Sum256([]byte(key))
	f.StorageKey = hex.EncodeToString(hash[:])
	return f.StorageKey
}

// IsModified compare avec une autre entrée pour détecter les modifications
func (f *FileEntry) IsModified(other *FileEntry) bool {
	if f.Checksum != other.Checksum {
		return true
	}
	if f.Size != other.Size {
		return true
	}
	if !f.ModifiedTime.Equal(other.ModifiedTime) {
		return true
	}
	return false
}

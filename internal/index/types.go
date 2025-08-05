package index

import (
	"crypto/sha256"
	"encoding/hex"
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

// NewFileEntry crée une nouvelle entrée de fichier
func NewFileEntry(path string, info os.FileInfo) (*FileEntry, error) {
	checksum, err := calculateChecksum(path)
	if err != nil {
		return nil, err
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

// calculateChecksum calcule le checksum SHA256 d'un fichier
func calculateChecksum(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// getUserName récupère le nom d'utilisateur par UID
func getUserName(uid int) string {
	// Implémentation simplifiée - dans un vrai projet, on utiliserait
	// user.LookupId() mais cela nécessite des permissions spéciales
	return "unknown"
}

// getGroupName récupère le nom de groupe par GID
func getGroupName(gid int) string {
	// Implémentation simplifiée - dans un vrai projet, on utiliserait
	// user.LookupGroupId() mais cela nécessite des permissions spéciales
	return "unknown"
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

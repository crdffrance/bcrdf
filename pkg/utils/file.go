package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ReadFile lit un fichier et retourne son contenu
func ReadFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading file %s: %w", path, err)
	}
	return data, nil
}

// WriteFile écrit des données dans un fichier
func WriteFile(path string, data []byte) error {
	// Créer le répertoire parent si nécessaire
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creating directory %s: %w", dir, err)
	}

	// Écrire le fichier
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("error writing file %s: %w", path, err)
	}

	return nil
}

// FileExists vérifie si un fichier existe
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// IsDirectory vérifie si un chemin est un répertoire
func IsDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// GetFileSize retourne la taille d'un fichier
func GetFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("error getting file size %s: %w", path, err)
	}
	return info.Size(), nil
}

// EnsureDirectory crée un répertoire s'il n'existe pas
func EnsureDirectory(path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("error creating directory %s: %w", path, err)
	}
	return nil
}

// RemoveFile supprime un fichier
func RemoveFile(path string) error {
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("error during la suppression du fichier %s: %w", path, err)
	}
	return nil
}

// RemoveDirectory supprime un répertoire et son contenu
func RemoveDirectory(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("error removing directory %s: %w", path, err)
	}
	return nil
}

// ReadFileWithBuffer reads a file with configurable buffer size for better performance
func ReadFileWithBuffer(path string, bufferSize int) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error opening file %s: %w", path, err)
	}
	defer file.Close()

	// Get file size
	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("error getting file stats %s: %w", path, err)
	}

	// Use optimized buffer size
	if bufferSize <= 0 {
		bufferSize = 64 * 1024 // Default 64KB
	}

	// For very large files (> 100MB), use streaming approach
	fileSize := int(stat.Size())
	if fileSize > 100*1024*1024 { // 100MB threshold
		Debug("Large file detected (%d bytes), using streaming read", fileSize)
		return readFileStreaming(file, bufferSize)
	}

	// For small files, use the file size as buffer
	if fileSize < bufferSize {
		bufferSize = fileSize
	}

	// Read with buffer
	data := make([]byte, fileSize)
	buffer := make([]byte, bufferSize)
	totalRead := 0

	for {
		n, err := file.Read(buffer)
		if n > 0 {
			copy(data[totalRead:], buffer[:n])
			totalRead += n
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading file %s: %w", path, err)
		}
	}

	return data[:totalRead], nil
}

// readFileStreaming reads large files in chunks to avoid memory issues
func readFileStreaming(file *os.File, bufferSize int) ([]byte, error) {
	var data []byte
	buffer := make([]byte, bufferSize)

	for {
		n, err := file.Read(buffer)
		if n > 0 {
			data = append(data, buffer[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading file: %w", err)
		}
	}

	return data, nil
}

// ParseBufferSize parses buffer size string (e.g., "64MB", "1GB", "512KB")
func ParseBufferSize(sizeStr string) (int, error) {
	if sizeStr == "" {
		return 64 * 1024 * 1024, nil // Default 64MB
	}

	sizeStr = strings.ToUpper(strings.TrimSpace(sizeStr))

	// Parse number and unit
	var multiplier int64 = 1
	var numStr string

	if strings.HasSuffix(sizeStr, "GB") {
		multiplier = 1024 * 1024 * 1024
		numStr = strings.TrimSuffix(sizeStr, "GB")
	} else if strings.HasSuffix(sizeStr, "MB") {
		multiplier = 1024 * 1024
		numStr = strings.TrimSuffix(sizeStr, "MB")
	} else if strings.HasSuffix(sizeStr, "KB") {
		multiplier = 1024
		numStr = strings.TrimSuffix(sizeStr, "KB")
	} else if strings.HasSuffix(sizeStr, "B") {
		multiplier = 1
		numStr = strings.TrimSuffix(sizeStr, "B")
	} else {
		// Assume bytes if no unit
		numStr = sizeStr
	}

	num, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid buffer size format: %s", sizeStr)
	}

	result := int(num * multiplier)

	// Sanity checks
	if result < 1024 {
		return 1024, nil // Minimum 1KB
	}
	if result > 1024*1024*1024 {
		return 1024 * 1024 * 1024, nil // Maximum 1GB
	}

	return result, nil
}

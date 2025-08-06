package utils

import (
	"fmt"
	"os"
	"path/filepath"
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
	if err := os.WriteFile(path, data, 0644); err != nil {
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

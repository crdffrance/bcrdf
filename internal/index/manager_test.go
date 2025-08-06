package index

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewFileEntry(t *testing.T) {
	// Créer un fichier temporaire pour le test
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")

	// Écrire des données de test
	testData := []byte("Hello, BCRDF!")
	if err := os.WriteFile(testFile, testData, 0600); err != nil {
		t.Fatalf("Erreur lors de la création du fichier de test: %v", err)
	}

	// Obtenir les informations du fichier
	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Erreur lors de la récupération des informations du fichier: %v", err)
	}

	// Créer une entrée de fichier
	entry, err := NewFileEntry(testFile, info)
	if err != nil {
		t.Fatalf("Erreur lors de la création de l'entrée: %v", err)
	}

	// Vérifier les propriétés de l'entrée
	if entry.Path != testFile {
		t.Errorf("Chemin incorrect: attendu %s, obtenu %s", testFile, entry.Path)
	}

	if entry.Size != int64(len(testData)) {
		t.Errorf("Taille incorrecte: attendu %d, obtenu %d", len(testData), entry.Size)
	}

	if entry.IsDirectory {
		t.Error("Le fichier ne devrait pas être marqué comme répertoire")
	}

	if entry.Checksum == "" {
		t.Error("Le checksum ne devrait pas être vide")
	}
}

func TestGetStorageKey(t *testing.T) {
	entry := &FileEntry{
		Path:     "/test/path/file.txt",
		Checksum: "abc123",
	}

	key1 := entry.GetStorageKey()
	key2 := entry.GetStorageKey()

	// La clé devrait être la même à chaque appel
	if key1 != key2 {
		t.Errorf("Les clés de stockage devraient être identiques: %s != %s", key1, key2)
	}

	// La clé ne devrait pas être vide
	if key1 == "" {
		t.Error("La clé de stockage ne devrait pas être vide")
	}
}

func TestIsModified(t *testing.T) {
	entry1 := &FileEntry{
		Path:         "/test/file.txt",
		Size:         100,
		ModifiedTime: time.Now(),
		Checksum:     "abc123",
	}

	entry2 := &FileEntry{
		Path:         "/test/file.txt",
		Size:         100,
		ModifiedTime: entry1.ModifiedTime,
		Checksum:     "abc123",
	}

	// Les entrées identiques ne devraient pas être considérées comme modifiées
	if entry1.IsModified(entry2) {
		t.Error("Les entrées identiques ne devraient pas être considérées comme modifiées")
	}

	// Modifier une propriété
	entry2.Size = 200
	if !entry1.IsModified(entry2) {
		t.Error("Les entrées avec des tailles différentes devraient être considérées comme modifiées")
	}
}

func TestShouldSkipFile(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"fichier normal", "/path/to/file.txt", false},
		{"fichier caché", "/path/to/.hidden", true},
		{"fichier temporaire", "/path/to/file.tmp", true},
		{"fichier de sauvegarde", "/path/to/file.bak", true},
		{"répertoire système", "/proc/file", true},
		{"répertoire système", "/sys/file", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Créer un fichier temporaire pour le test
			tempDir := t.TempDir()
			testFile := filepath.Join(tempDir, filepath.Base(tt.path))

			if err := os.WriteFile(testFile, []byte("test"), 0600); err != nil {
				t.Fatalf("Erreur lors de la création du fichier de test: %v", err)
			}

			info, err := os.Stat(testFile)
			if err != nil {
				t.Fatalf("Erreur lors de la récupération des informations: %v", err)
			}

			result := shouldSkipFile(tt.path, info)
			if result != tt.expected {
				t.Errorf("shouldSkipFile(%s) = %v, attendu %v", tt.path, result, tt.expected)
			}
		})
	}
}

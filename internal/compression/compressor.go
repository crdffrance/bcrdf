package compression

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"

	"bcrdf/pkg/utils"
)

// Compressor gère la compression et décompression des données
type Compressor struct {
	level int
}

// NewCompressor crée un nouveau compresseur avec un niveau de compression
func NewCompressor(level int) (*Compressor, error) {
	if level < 1 || level > 9 {
		return nil, fmt.Errorf("niveau de compression invalide: %d (doit être entre 1 et 9)", level)
	}

	return &Compressor{
		level: level,
	}, nil
}

// Compress compresse des données avec GZIP
func (c *Compressor) Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer

	// Créer un writer GZIP avec le niveau spécifié
	writer, err := gzip.NewWriterLevel(&buf, c.level)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la création du writer GZIP: %w", err)
	}

	// Écrire les données
	if _, err := writer.Write(data); err != nil {
		return nil, fmt.Errorf("erreur lors de l'écriture des données: %w", err)
	}

	// Fermer le writer pour finaliser la compression
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("erreur lors de la fermeture du writer: %w", err)
	}

	compressed := buf.Bytes()
	utils.Debug("Données compressées: %d bytes -> %d bytes (ratio: %.2f%%)",
		len(data), len(compressed), float64(len(compressed))/float64(len(data))*100)

	return compressed, nil
}

// Decompress décompresse des données avec GZIP
func (c *Compressor) Decompress(data []byte) ([]byte, error) {
	// Créer un reader GZIP
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la création du reader GZIP: %w", err)
	}
	defer reader.Close()

	// Lire toutes les données décompressées
	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la décompression: %w", err)
	}

	utils.Debug("Données décompressées: %d bytes -> %d bytes",
		len(data), len(decompressed))

	return decompressed, nil
}

// CompressFile compresse un fichier complet
func (c *Compressor) CompressFile(inputPath, outputPath string) error {
	utils.Info("Compression du fichier: %s", inputPath)

	// Lire le fichier source
	data, err := utils.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("erreur lors de la lecture du fichier: %w", err)
	}

	// Compresser les données
	compressedData, err := c.Compress(data)
	if err != nil {
		return fmt.Errorf("erreur lors de la compression: %w", err)
	}

	// Écrire le fichier compressé
	if err := utils.WriteFile(outputPath, compressedData); err != nil {
		return fmt.Errorf("erreur lors de l'écriture du fichier compressé: %w", err)
	}

	utils.Info("Fichier compressé sauvegardé: %s", outputPath)
	return nil
}

// DecompressFile décompresse un fichier complet
func (c *Compressor) DecompressFile(inputPath, outputPath string) error {
	utils.Info("Décompression du fichier: %s", inputPath)

	// Lire le fichier compressé
	compressedData, err := utils.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("erreur lors de la lecture du fichier compressé: %w", err)
	}

	// Décompresser les données
	decompressedData, err := c.Decompress(compressedData)
	if err != nil {
		return fmt.Errorf("erreur lors de la décompression: %w", err)
	}

	// Écrire le fichier décompressé
	if err := utils.WriteFile(outputPath, decompressedData); err != nil {
		return fmt.Errorf("erreur lors de l'écriture du fichier décompressé: %w", err)
	}

	utils.Info("Fichier décompressé sauvegardé: %s", outputPath)
	return nil
}

// CompressStream compresse un flux de données
func (c *Compressor) CompressStream(input io.Reader, output io.Writer) error {
	// Lire toutes les données
	data, err := io.ReadAll(input)
	if err != nil {
		return fmt.Errorf("erreur lors de la lecture des données: %w", err)
	}

	// Compresser les données
	compressedData, err := c.Compress(data)
	if err != nil {
		return fmt.Errorf("erreur lors de la compression: %w", err)
	}

	// Écrire les données compressées
	if _, err := output.Write(compressedData); err != nil {
		return fmt.Errorf("erreur lors de l'écriture des données compressées: %w", err)
	}

	return nil
}

// DecompressStream décompresse un flux de données
func (c *Compressor) DecompressStream(input io.Reader, output io.Writer) error {
	// Lire toutes les données compressées
	compressedData, err := io.ReadAll(input)
	if err != nil {
		return fmt.Errorf("erreur lors de la lecture des données compressées: %w", err)
	}

	// Décompresser les données
	decompressedData, err := c.Decompress(compressedData)
	if err != nil {
		return fmt.Errorf("erreur lors de la décompression: %w", err)
	}

	// Écrire les données décompressées
	if _, err := output.Write(decompressedData); err != nil {
		return fmt.Errorf("erreur lors de l'écriture des données décompressées: %w", err)
	}

	return nil
}

// GetCompressionRatio calcule le ratio de compression
func (c *Compressor) GetCompressionRatio(originalSize, compressedSize int64) float64 {
	if originalSize == 0 {
		return 0.0
	}
	return float64(compressedSize) / float64(originalSize) * 100.0
}

// GetCompressionLevel retourne le niveau de compression actuel
func (c *Compressor) GetCompressionLevel() int {
	return c.level
}

// SetCompressionLevel définit un nouveau niveau de compression
func (c *Compressor) SetCompressionLevel(level int) error {
	if level < 1 || level > 9 {
		return fmt.Errorf("niveau de compression invalide: %d (doit être entre 1 et 9)", level)
	}
	c.level = level
	return nil
}

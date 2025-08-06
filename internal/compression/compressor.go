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
		return nil, fmt.Errorf("invalid compression level: %d (must be between 1 and 9)", level)
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
		return nil, fmt.Errorf("error creating GZIP writer: %w", err)
	}

	// Écrire les données
	if _, err := writer.Write(data); err != nil {
		return nil, fmt.Errorf("error writing data: %w", err)
	}

	// Fermer le writer pour finaliser la compression
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("error closing writer: %w", err)
	}

	compressed := buf.Bytes()
	utils.Debug("Data compressed: %d bytes -> %d bytes (ratio: %.2f%%)",
		len(data), len(compressed), float64(len(compressed))/float64(len(data))*100)

	return compressed, nil
}

// Decompress décompresse des données avec GZIP
func (c *Compressor) Decompress(data []byte) ([]byte, error) {
	// Créer un reader GZIP
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("error creating GZIP reader: %w", err)
	}
	defer reader.Close()

	// Lire toutes les données decompressed
	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("error decompressing: %w", err)
	}

	utils.Debug("Data decompressed: %d bytes -> %d bytes",
		len(data), len(decompressed))

	return decompressed, nil
}

// CompressFile compresse un fichier complet
func (c *Compressor) CompressFile(inputPath, outputPath string) error {
	utils.Info("Compression du fichier: %s", inputPath)

	// Lire le fichier source
	data, err := utils.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	// Compress les données
	compressedData, err := c.Compress(data)
	if err != nil {
		return fmt.Errorf("error compressing: %w", err)
	}

	// Écrire le fichier compressed
	if err := utils.WriteFile(outputPath, compressedData); err != nil {
		return fmt.Errorf("error writing file compressed: %w", err)
	}

	utils.Info("Compressed file saved: %s", outputPath)
	return nil
}

// DecompressFile décompresse un fichier complet
func (c *Compressor) DecompressFile(inputPath, outputPath string) error {
	utils.Info("Decompressing file: %s", inputPath)

	// Lire le fichier compressed
	compressedData, err := utils.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("error reading compressed file: %w", err)
	}

	// Decompress les données
	decompressedData, err := c.Decompress(compressedData)
	if err != nil {
		return fmt.Errorf("error decompressing: %w", err)
	}

	// Écrire le fichier décompressed
	if err := utils.WriteFile(outputPath, decompressedData); err != nil {
		return fmt.Errorf("error writing decompressed file: %w", err)
	}

	utils.Info("Decompressed file saved: %s", outputPath)
	return nil
}

// CompressStream compresse un flux de données
func (c *Compressor) CompressStream(input io.Reader, output io.Writer) error {
	// Lire toutes les données
	data, err := io.ReadAll(input)
	if err != nil {
		return fmt.Errorf("error reading data: %w", err)
	}

	// Compress les données
	compressedData, err := c.Compress(data)
	if err != nil {
		return fmt.Errorf("error compressing: %w", err)
	}

	// Écrire les données compressedes
	if _, err := output.Write(compressedData); err != nil {
		return fmt.Errorf("error writing data compressedes: %w", err)
	}

	return nil
}

// DecompressStream décompresse un flux de données
func (c *Compressor) DecompressStream(input io.Reader, output io.Writer) error {
	// Lire toutes les données compressedes
	compressedData, err := io.ReadAll(input)
	if err != nil {
		return fmt.Errorf("error reading data compressedes: %w", err)
	}

	// Decompress les données
	decompressedData, err := c.Decompress(compressedData)
	if err != nil {
		return fmt.Errorf("error decompressing: %w", err)
	}

	// Écrire les données decompressed
	if _, err := output.Write(decompressedData); err != nil {
		return fmt.Errorf("error writing data decompressed: %w", err)
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
		return fmt.Errorf("invalid compression level: %d (must be between 1 and 9)", level)
	}
	c.level = level
	return nil
}

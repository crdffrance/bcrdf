package compression

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"path/filepath"
	"strings"

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

// CompressFile compresses data with adaptive compression based on file type and size
func (c *Compressor) CompressFile(data []byte, filePath string) ([]byte, error) {
	// Check if file should be compressed based on extension
	if !c.shouldCompress(filePath) {
		utils.Debug("Skipping compression for: %s (already compressed format)", filePath)
		return data, nil
	}

	// Adaptive compression based on file size
	return c.CompressAdaptive(data, filePath)
}

// CompressAdaptive uses different compression levels based on file size
func (c *Compressor) CompressAdaptive(data []byte, filePath string) ([]byte, error) {
	fileSize := len(data)

	// Extremely large files (> 1GB): skip compression to avoid memory issues
	if fileSize > 1024*1024*1024 {
		utils.Debug("Extremely large file detected (%d bytes), skipping compression to avoid memory issues", fileSize)
		return data, nil // Skip compression for very large files
	}

	// Very large files (> 100MB): use configured level (usually fast)
	if fileSize > 100*1024*1024 {
		utils.Debug("Large file detected (%d bytes), using configured compression level %d", fileSize, c.level)
		return c.CompressWithLevel(data, c.level) // Use configured level
	}

	// Large files (10MB - 100MB): use configured level
	if fileSize > 10*1024*1024 {
		utils.Debug("Medium file detected (%d bytes), using configured compression level %d", fileSize, c.level)
		return c.CompressWithLevel(data, c.level) // Use configured level
	}

	// Small files (< 10MB): try compression and check if it's beneficial
	utils.Debug("Small file detected (%d bytes), testing compression with level %d", fileSize, c.level)
	compressed, err := c.CompressWithLevel(data, c.level) // Use configured level
	if err != nil {
		return nil, err
	}

	// If compression doesn't help (ratio > 95%), return original data
	compressionRatio := float64(len(compressed)) / float64(fileSize) * 100
	if compressionRatio > 95 {
		utils.Debug("Compression not beneficial (ratio: %.2f%%), keeping original", compressionRatio)
		return data, nil
	}

	utils.Debug("Compression beneficial (ratio: %.2f%%), using compressed data", compressionRatio)
	return compressed, nil
}

// CompressWithLevel compresses data with a specific compression level
func (c *Compressor) CompressWithLevel(data []byte, level int) ([]byte, error) {
	var buf bytes.Buffer

	// Create GZIP writer with specified level
	writer, err := gzip.NewWriterLevel(&buf, level)
	if err != nil {
		return nil, fmt.Errorf("error creating GZIP writer: %w", err)
	}

	// Write data
	if _, err := writer.Write(data); err != nil {
		return nil, fmt.Errorf("error writing data: %w", err)
	}

	// Close writer to finalize compression
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("error closing writer: %w", err)
	}

	compressed := buf.Bytes()
	utils.Debug("Data compressed with level %d: %d bytes -> %d bytes (ratio: %.2f%%)",
		level, len(data), len(compressed), float64(len(compressed))/float64(len(data))*100)

	return compressed, nil
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
// IsCompressed checks if data appears to be GZIP compressed
func (c *Compressor) IsCompressed(data []byte) bool {
	if len(data) < 2 {
		return false
	}
	// GZIP magic number: 0x1f 0x8b
	return data[0] == 0x1f && data[1] == 0x8b
}

func (c *Compressor) Decompress(data []byte) ([]byte, error) {
	// Check if data is actually compressed
	if !c.IsCompressed(data) {
		utils.Debug("Data not compressed, returning as-is: %d bytes", len(data))
		return data, nil
	}

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

// CompressFileToFile compresse un fichier complet vers un autre fichier
func (c *Compressor) CompressFileToFile(inputPath, outputPath string) error {
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

// CompressStreamOptimized compresses data in chunks for better memory efficiency
func (c *Compressor) CompressStreamOptimized(input io.Reader, output io.Writer, chunkSize int) error {
	if chunkSize <= 0 {
		chunkSize = 64 * 1024 * 1024 // 64MB default
	}

	// Create GZIP writer
	gzipWriter, err := gzip.NewWriterLevel(output, c.level)
	if err != nil {
		return fmt.Errorf("error creating GZIP writer: %w", err)
	}
	defer gzipWriter.Close()

	// Process data in chunks
	buffer := make([]byte, chunkSize)
	for {
		n, err := input.Read(buffer)
		if n > 0 {
			if _, writeErr := gzipWriter.Write(buffer[:n]); writeErr != nil {
				return fmt.Errorf("error writing compressed data: %w", writeErr)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading input: %w", err)
		}
	}

	return nil
}

// DecompressStreamOptimized decompresses data in chunks for better memory efficiency
func (c *Compressor) DecompressStreamOptimized(input io.Reader, output io.Writer, chunkSize int) error {
	if chunkSize <= 0 {
		chunkSize = 64 * 1024 * 1024 // 64MB default
	}

	// Create GZIP reader
	gzipReader, err := gzip.NewReader(input)
	if err != nil {
		return fmt.Errorf("error creating GZIP reader: %w", err)
	}
	defer gzipReader.Close()

	// Process data in chunks
	buffer := make([]byte, chunkSize)
	for {
		n, err := gzipReader.Read(buffer)
		if n > 0 {
			if _, writeErr := output.Write(buffer[:n]); writeErr != nil {
				return fmt.Errorf("error writing decompressed data: %w", writeErr)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading compressed data: %w", err)
		}
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

// shouldCompress determines if a file should be compressed based on its extension
func (c *Compressor) shouldCompress(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))

	// Already compressed formats - skip compression
	compressedFormats := map[string]bool{
		// Images
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
		".tiff": true, ".tif": true, ".bmp": true, ".ico": true, ".svg": true,
		// Videos
		".mp4": true, ".avi": true, ".mkv": true, ".mov": true, ".wmv": true,
		".flv": true, ".webm": true, ".m4v": true, ".3gp": true, ".ogv": true,
		// Audio
		".mp3": true, ".aac": true, ".ogg": true, ".wma": true, ".flac": true,
		".m4a": true, ".opus": true, ".wav": false, // WAV can benefit from compression
		// Archives
		".zip": true, ".rar": true, ".7z": true, ".tar.gz": true, ".tgz": true,
		".tar.bz2": true, ".tar.xz": true, ".gz": true, ".bz2": true, ".xz": true,
		// Documents (already compressed)
		".pdf": true, ".docx": true, ".xlsx": true, ".pptx": true,
		".odt": true, ".ods": true, ".odp": true,
		// Executables and binaries
		".exe": false, ".dll": false, ".so": false, ".dylib": false, // Can benefit from compression
		".bin": false, ".iso": false,
	}

	// Check if format is in the map
	if skipCompress, exists := compressedFormats[ext]; exists {
		return !skipCompress
	}

	// For unknown extensions, apply compression
	return true
}

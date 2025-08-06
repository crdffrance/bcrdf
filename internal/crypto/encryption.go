package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	"bcrdf/pkg/utils"
)

// Encryptor gère le chiffrement et déchiffrement des données
type Encryptor struct {
	key []byte
}

// NewEncryptor crée un nouveau chiffreur avec une clé
func NewEncryptor(key string) (*Encryptor, error) {
	// Dériver une clé de 32 bytes (256 bits) à partir de la clé fournie
	hash := sha256.Sum256([]byte(key))

	return &Encryptor{
		key: hash[:],
	}, nil
}

// Encrypt chiffre des données avec AES-256-GCM
func (e *Encryptor) Encrypt(data []byte) ([]byte, error) {
	// Créer un nouveau cipher AES
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la création du cipher AES: %w", err)
	}

	// Créer un GCM cipher
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la création du GCM: %w", err)
	}

	// Créer un nonce aléatoire
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("erreur lors de la génération du nonce: %w", err)
	}

	// Chiffrer les données
	ciphertext := gcm.Seal(nonce, nonce, data, nil)

	utils.Debug("Data encrypted: %d bytes -> %d bytes", len(data), len(ciphertext))
	return ciphertext, nil
}

// Decrypt déchiffre des données avec AES-256-GCM
func (e *Encryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	// Créer un nouveau cipher AES
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la création du cipher AES: %w", err)
	}

	// Créer un GCM cipher
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la création du GCM: %w", err)
	}

	// Extraire le nonce
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext trop court")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Déchiffrer les données
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("erreur lors du déchiffrement: %w", err)
	}

	utils.Debug("Data decrypted: %d bytes -> %d bytes", len(ciphertext), len(plaintext))
	return plaintext, nil
}

// EncryptFile chiffre un fichier complet
func (e *Encryptor) EncryptFile(inputPath, outputPath string) error {
	utils.Info("Chiffrement du fichier: %s", inputPath)

	// Lire le fichier source
	data, err := utils.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("erreur lors de la lecture du fichier: %w", err)
	}

	// Chiffrer les données
	encryptedData, err := e.Encrypt(data)
	if err != nil {
		return fmt.Errorf("erreur lors du chiffrement: %w", err)
	}

	// Écrire le fichier chiffré
	if err := utils.WriteFile(outputPath, encryptedData); err != nil {
		return fmt.Errorf("erreur lors de l'écriture du fichier chiffré: %w", err)
	}

	utils.Info("Encrypted file saved: %s", outputPath)
	return nil
}

// DecryptFile déchiffre un fichier complet
func (e *Encryptor) DecryptFile(inputPath, outputPath string) error {
	utils.Info("Decrypting file: %s", inputPath)

	// Lire le fichier chiffré
	encryptedData, err := utils.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("erreur lors de la lecture du fichier chiffré: %w", err)
	}

	// Déchiffrer les données
	decryptedData, err := e.Decrypt(encryptedData)
	if err != nil {
		return fmt.Errorf("erreur lors du déchiffrement: %w", err)
	}

	// Écrire le fichier déchiffré
	if err := utils.WriteFile(outputPath, decryptedData); err != nil {
		return fmt.Errorf("erreur lors de l'écriture du fichier déchiffré: %w", err)
	}

	utils.Info("Decrypted file saved: %s", outputPath)
	return nil
}

// EncryptStream chiffre un flux de données
func (e *Encryptor) EncryptStream(input io.Reader, output io.Writer) error {
	// Lire toutes les données
	data, err := io.ReadAll(input)
	if err != nil {
		return fmt.Errorf("erreur lors de la lecture des données: %w", err)
	}

	// Chiffrer les données
	encryptedData, err := e.Encrypt(data)
	if err != nil {
		return fmt.Errorf("erreur lors du chiffrement: %w", err)
	}

	// Écrire les données chiffrées
	if _, err := output.Write(encryptedData); err != nil {
		return fmt.Errorf("erreur lors de l'écriture des données chiffrées: %w", err)
	}

	return nil
}

// DecryptStream déchiffre un flux de données
func (e *Encryptor) DecryptStream(input io.Reader, output io.Writer) error {
	// Lire toutes les données chiffrées
	encryptedData, err := io.ReadAll(input)
	if err != nil {
		return fmt.Errorf("erreur lors de la lecture des données chiffrées: %w", err)
	}

	// Déchiffrer les données
	decryptedData, err := e.Decrypt(encryptedData)
	if err != nil {
		return fmt.Errorf("erreur lors du déchiffrement: %w", err)
	}

	// Écrire les données déchiffrées
	if _, err := output.Write(decryptedData); err != nil {
		return fmt.Errorf("erreur lors de l'écriture des données déchiffrées: %w", err)
	}

	return nil
}

// GenerateKey génère une clé de chiffrement aléatoire
func GenerateKey() (string, error) {
	key := make([]byte, 32) // 256 bits
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", fmt.Errorf("erreur lors de la génération de la clé: %w", err)
	}

	return hex.EncodeToString(key), nil
}

// ValidateKey valide une clé de chiffrement
func ValidateKey(key string) error {
	if len(key) < 16 {
		return fmt.Errorf("la clé de chiffrement doit faire au moins 16 caractères")
	}
	return nil
}

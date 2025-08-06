package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
)

// EncryptionAlgorithm représente les algorithmes de chiffrement supportés
type EncryptionAlgorithm string

const (
	AES256GCM         EncryptionAlgorithm = "aes-256-gcm"
	XChaCha20Poly1305 EncryptionAlgorithm = "xchacha20-poly1305"
)

// decodeKey décode une clé hexadécimale ou retourne les bytes bruts
func decodeKey(key string) ([]byte, error) {
	// Si la clé fait 64 caractères et ne contient que des caractères hex, c'est probablement une clé hex
	if len(key) == 64 {
		if _, err := hex.DecodeString(key); err == nil {
			return hex.DecodeString(key)
		}
	}

	// Sinon, traiter comme des bytes bruts
	return []byte(key), nil
}

// EncryptorV2 représente un chiffreur avec support multi-algorithmes
type EncryptorV2 struct {
	key       []byte
	algorithm EncryptionAlgorithm
	aesGCM    cipher.AEAD
	xchacha   cipher.AEAD
}

// NewEncryptorV2 crée un nouveau chiffreur avec l'algorithme spécifié
func NewEncryptorV2(key string, algorithm EncryptionAlgorithm) (*EncryptorV2, error) {
	// Décoder la clé hexadécimale si nécessaire
	keyBytes, err := decodeKey(key)
	if err != nil {
		return nil, fmt.Errorf("error decoding key: %w", err)
	}

	// Valider la longueur de la clé selon l'algorithme
	switch algorithm {
	case AES256GCM:
		if len(keyBytes) != 32 {
			return nil, fmt.Errorf("AES-256-GCM requires a 32-byte key, got %d", len(keyBytes))
		}
	case XChaCha20Poly1305:
		if len(keyBytes) != 32 {
			return nil, fmt.Errorf("XChaCha20-Poly1305 requires a 32-byte key, got %d", len(keyBytes))
		}
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", algorithm)
	}

	encryptor := &EncryptorV2{
		key:       keyBytes,
		algorithm: algorithm,
	}

	// Initialiser l'algorithme spécifique
	if err := encryptor.initialize(); err != nil {
		return nil, err
	}

	return encryptor, nil
}

// initialize initialise les algorithmes de chiffrement
func (e *EncryptorV2) initialize() error {
	switch e.algorithm {
	case AES256GCM:
		block, err := aes.NewCipher(e.key)
		if err != nil {
			return fmt.Errorf("error creating AES cipher: %w", err)
		}
		e.aesGCM, err = cipher.NewGCM(block)
		if err != nil {
			return fmt.Errorf("error creating GCM: %w", err)
		}
	case XChaCha20Poly1305:
		var err error
		e.xchacha, err = chacha20poly1305.NewX(e.key)
		if err != nil {
			return fmt.Errorf("error creating XChaCha20-Poly1305: %w", err)
		}
	}
	return nil
}

// Encrypt chiffre des données avec l'algorithme configuré
func (e *EncryptorV2) Encrypt(plaintext []byte) ([]byte, error) {
	switch e.algorithm {
	case AES256GCM:
		return e.encryptAES(plaintext)
	case XChaCha20Poly1305:
		return e.encryptXChaCha(plaintext)
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", e.algorithm)
	}
}

// Decrypt déchiffre des données avec l'algorithme configuré
func (e *EncryptorV2) Decrypt(ciphertext []byte) ([]byte, error) {
	switch e.algorithm {
	case AES256GCM:
		return e.decryptAES(ciphertext)
	case XChaCha20Poly1305:
		return e.decryptXChaCha(ciphertext)
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", e.algorithm)
	}
}

// encryptAES chiffre avec AES-256-GCM
func (e *EncryptorV2) encryptAES(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, e.aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("error generating nonce: %w", err)
	}

	ciphertext := e.aesGCM.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// decryptAES déchiffre avec AES-256-GCM
func (e *EncryptorV2) decryptAES(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < e.aesGCM.NonceSize() {
		return nil, fmt.Errorf("ciphertext trop court")
	}

	nonce := ciphertext[:e.aesGCM.NonceSize()]
	ciphertext = ciphertext[e.aesGCM.NonceSize():]

	plaintext, err := e.aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("error decrypting AES: %w", err)
	}

	return plaintext, nil
}

// encryptXChaCha chiffre avec XChaCha20-Poly1305
func (e *EncryptorV2) encryptXChaCha(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, e.xchacha.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("error generating nonce: %w", err)
	}

	ciphertext := e.xchacha.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// decryptXChaCha déchiffre avec XChaCha20-Poly1305
func (e *EncryptorV2) decryptXChaCha(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < e.xchacha.NonceSize() {
		return nil, fmt.Errorf("ciphertext trop court")
	}

	nonce := ciphertext[:e.xchacha.NonceSize()]
	ciphertext = ciphertext[e.xchacha.NonceSize():]

	plaintext, err := e.xchacha.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("error decrypting XChaCha: %w", err)
	}

	return plaintext, nil
}

// GetAlgorithm retourne l'algorithme utilisé
func (e *EncryptorV2) GetAlgorithm() EncryptionAlgorithm {
	return e.algorithm
}

// ValidateKeyV2 valide une clé pour l'algorithme spécifié
func ValidateKeyV2(key string, algorithm EncryptionAlgorithm) error {
	keyBytes, err := decodeKey(key)
	if err != nil {
		return fmt.Errorf("error decoding key: %w", err)
	}

	switch algorithm {
	case AES256GCM, XChaCha20Poly1305:
		if len(keyBytes) != 32 {
			return fmt.Errorf("key must be 32 bytes for %s, got %d", algorithm, len(keyBytes))
		}
		return nil
	default:
		return fmt.Errorf("unsupported algorithm: %s", algorithm)
	}
}

// GenerateKeyV2 génère une clé aléatoire pour l'algorithme spécifié
func GenerateKeyV2(algorithm EncryptionAlgorithm) (string, error) {
	var keySize int

	switch algorithm {
	case AES256GCM, XChaCha20Poly1305:
		keySize = 32
	default:
		return "", fmt.Errorf("unsupported algorithm: %s", algorithm)
	}

	key := make([]byte, keySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", fmt.Errorf("error generating key: %w", err)
	}

	return string(key), nil
}

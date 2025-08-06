package storage

import (
	"time"
)

// ObjectInfo représente les informations d'un objet de stockage
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
}

// Client représente une interface commune pour les clients de stockage
type Client interface {
	// Upload télécharge des données vers le stockage
	Upload(key string, data []byte) error

	// Download télécharge des données depuis le stockage
	Download(key string) ([]byte, error)

	// DeleteObject supprime un objet du stockage
	DeleteObject(key string) error

	// ListObjects liste les objets avec un préfixe donné
	ListObjects(prefix string) ([]ObjectInfo, error)

	// TestConnectivity teste la connectivité au stockage
	TestConnectivity() error
}

// StorageType représente le type de stockage
type StorageType string

const (
	S3Storage     StorageType = "s3"
	WebDAVStorage StorageType = "webdav"
)

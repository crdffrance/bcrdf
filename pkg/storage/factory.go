package storage

import (
	"fmt"

	"bcrdf/pkg/utils"
)

// NewStorageClient crée un client de stockage basé sur la configuration
func NewStorageClient(config *utils.Config) (Client, error) {
	switch config.Storage.Type {
	case "s3":
		return NewS3Adapter(
			config.Storage.AccessKey,
			config.Storage.SecretKey,
			config.Storage.Region,
			config.Storage.Endpoint,
			config.Storage.Bucket,
		)

	case "webdav":
		return NewWebDAVAdapter(
			config.Storage.Endpoint,
			config.Storage.Username,
			config.Storage.Password,
		)

	default:
		return nil, fmt.Errorf("type de stockage non supporté: %s", config.Storage.Type)
	}
}

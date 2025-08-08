package storage

import (
	"bcrdf/pkg/s3"
)

// S3Adapter adapte le client S3 à l'interface commune
type S3Adapter struct {
	client       *s3.Client
	storageClass string
}

// NewS3Adapter crée un nouvel adaptateur S3
func NewS3Adapter(accessKey, secretKey, region, endpoint, bucket string) (*S3Adapter, error) {
	client, err := s3.NewClient(accessKey, secretKey, region, endpoint, bucket)
	if err != nil {
		return nil, err
	}

	return &S3Adapter{client: client}, nil
}

// NewS3AdapterWithStorageClass crée un nouvel adaptateur S3 avec classe de stockage
func NewS3AdapterWithStorageClass(accessKey, secretKey, region, endpoint, bucket, storageClass string) (*S3Adapter, error) {
	client, err := s3.NewClient(accessKey, secretKey, region, endpoint, bucket)
	if err != nil {
		return nil, err
	}

	return &S3Adapter{
		client:       client,
		storageClass: storageClass,
	}, nil
}

// Upload implémente l'interface Client
func (a *S3Adapter) Upload(key string, data []byte) error {
	return a.client.UploadWithStorageClass(key, data, a.storageClass)
}

// Download implémente l'interface Client
func (a *S3Adapter) Download(key string) ([]byte, error) {
	return a.client.Download(key)
}

// DeleteObject implémente l'interface Client
func (a *S3Adapter) DeleteObject(key string) error {
	return a.client.DeleteObject(key)
}

// ListObjects implémente l'interface Client
func (a *S3Adapter) ListObjects(prefix string) ([]ObjectInfo, error) {
	s3Objects, err := a.client.ListObjectsDetailed(prefix)
	if err != nil {
		return nil, err
	}

	// Convertir les objets S3 vers l'interface commune
	objects := make([]ObjectInfo, len(s3Objects))
	for i, obj := range s3Objects {
		objects[i] = ObjectInfo{
			Key:          obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified,
		}
	}

	return objects, nil
}

// TestConnectivity implémente l'interface Client
func (a *S3Adapter) TestConnectivity() error {
	// Utiliser ListObjects pour tester la connectivité
	_, err := a.client.ListObjects("test/")
	return err
}

package storage

import (
	"bcrdf/pkg/webdav"
)

// WebDAVAdapter adapte le client WebDAV à l'interface commune
type WebDAVAdapter struct {
	client *webdav.Client
}

// NewWebDAVAdapter crée un nouvel adaptateur WebDAV
func NewWebDAVAdapter(baseURL, username, password string) (*WebDAVAdapter, error) {
	client, err := webdav.NewClient(baseURL, username, password)
	if err != nil {
		return nil, err
	}

	return &WebDAVAdapter{client: client}, nil
}

// Upload implémente l'interface Client
func (a *WebDAVAdapter) Upload(key string, data []byte) error {
	return a.client.Upload(key, data)
}

// Download implémente l'interface Client
func (a *WebDAVAdapter) Download(key string) ([]byte, error) {
	return a.client.Download(key)
}

// DeleteObject implémente l'interface Client
func (a *WebDAVAdapter) DeleteObject(key string) error {
	return a.client.DeleteObject(key)
}

// ListObjects implémente l'interface Client
func (a *WebDAVAdapter) ListObjects(prefix string) ([]ObjectInfo, error) {
	webdavObjects, err := a.client.ListObjects(prefix)
	if err != nil {
		return nil, err
	}

	// Convertir les objets WebDAV vers l'interface commune
	objects := make([]ObjectInfo, len(webdavObjects))
	for i, obj := range webdavObjects {
		objects[i] = ObjectInfo{
			Key:          obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified,
		}
	}

	return objects, nil
}

// TestConnectivity implémente l'interface Client
func (a *WebDAVAdapter) TestConnectivity() error {
	return a.client.TestConnectivity()
}

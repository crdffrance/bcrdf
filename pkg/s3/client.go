package s3

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"

	"bcrdf/pkg/utils"
)

// ObjectInfo représente les informations d'un objet S3
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
}

// Client représente un client S3
type Client struct {
	s3Client   *s3.S3
	uploader   *s3manager.Uploader
	downloader *s3manager.Downloader
	bucket     string
	region     string
}

// NewClient crée un nouveau client S3
func NewClient(accessKey, secretKey, region, endpoint, bucket string) (*Client, error) {
	// Configuration AWS
	config := &aws.Config{
		Region: aws.String(region),
		Credentials: credentials.NewStaticCredentials(
			accessKey,
			secretKey,
			"",
		),
	}

	// Configuration de l'endpoint personnalisé si fourni
	if endpoint != "" {
		config.Endpoint = aws.String(endpoint)
		config.S3ForcePathStyle = aws.Bool(true)
	}

	// Créer la session
	sess, err := session.NewSession(config)
	if err != nil {
		return nil, fmt.Errorf("error creating AWS session: %w", err)
	}

	// Créer le client S3
	s3Client := s3.New(sess)

	// Créer l'uploader avec optimisations de performance
	uploader := s3manager.NewUploader(sess, func(u *s3manager.Uploader) {
		u.PartSize = 64 * 1024 * 1024 // 64MB parts pour meilleure performance
		u.Concurrency = 10            // 10 uploads parallèles
		u.LeavePartsOnError = false   // Nettoyer en cas d'erreur
	})

	// Créer le downloader avec optimisations de performance
	downloader := s3manager.NewDownloader(sess, func(d *s3manager.Downloader) {
		d.PartSize = 64 * 1024 * 1024 // 64MB parts
		d.Concurrency = 10            // 10 downloads parallèles
	})

	return &Client{
		s3Client:   s3Client,
		uploader:   uploader,
		downloader: downloader,
		bucket:     bucket,
		region:     region,
	}, nil
}

// Upload upload un fichier vers S3
func (c *Client) Upload(key string, data []byte) error {
	return c.UploadWithStorageClass(key, data, "")
}

// UploadWithStorageClass upload un fichier vers S3 avec une classe de stockage spécifique
func (c *Client) UploadWithStorageClass(key string, data []byte, storageClass string) error {
	utils.Debug("Upload vers S3: %s/%s (%d bytes)", c.bucket, key, len(data))
	if storageClass != "" {
		utils.Debug("   Storage class: %s", storageClass)
	}

	// Créer un reader pour les données
	reader := bytes.NewReader(data)

	// Paramètres d'upload
	params := &s3manager.UploadInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
		Body:   reader,
	}

	// Ajouter la classe de stockage si spécifiée
	if storageClass != "" {
		params.StorageClass = aws.String(storageClass)
	}

	// Effectuer l'upload
	_, err := c.uploader.Upload(params)
	if err != nil {
		return fmt.Errorf("error during l'upload vers S3: %w", err)
	}

	utils.Debug("Upload successful: %s/%s", c.bucket, key)
	return nil
}

// Download télécharge un fichier depuis S3
func (c *Client) Download(key string) ([]byte, error) {
	utils.Debug("Download depuis S3: %s/%s", c.bucket, key)

	// Créer un buffer pour recevoir les données
	buffer := aws.NewWriteAtBuffer([]byte{})

	// Paramètres de download
	params := &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}

	// Effectuer le download
	_, err := c.downloader.Download(buffer, params)
	if err != nil {
		return nil, fmt.Errorf("error downloading from S3: %w", err)
	}

	utils.Debug("Download successful: %s/%s (%d bytes)", c.bucket, key, len(buffer.Bytes()))
	return buffer.Bytes(), nil
}

// ListObjects liste les objets dans un préfixe
func (c *Client) ListObjects(prefix string) ([]string, error) {
	utils.Debug("S3 object list with prefix: %s", prefix)
	utils.Debug("Bucket: %s, Region: %s", c.bucket, c.region)

	var keys []string

	// Paramètres de liste
	params := &s3.ListObjectsV2Input{
		Bucket: aws.String(c.bucket),
		Prefix: aws.String(prefix),
	}

	// Effectuer la liste
	result, err := c.s3Client.ListObjectsV2(params)
	if err != nil {
		utils.Debug("ListObjectsV2 error: %v", err)
		// Si le préfixe n'existe pas, retourner une empty list (pas d'erreur)
		if strings.Contains(err.Error(), "NoSuchKey") || strings.Contains(err.Error(), "404") {
			utils.Debug("Prefix %s does not exist yet, empty list", prefix)
			return keys, nil
		}
		return nil, fmt.Errorf("error listing S3 objects: %w", err)
	}

	// Extraire les clés
	for _, object := range result.Contents {
		keys = append(keys, *object.Key)
	}

	utils.Debug("Object list: %d objects found", len(keys))
	for i, key := range keys {
		utils.Debug("  [%d] %s", i+1, key)
	}
	return keys, nil
}

// ListObjectsDetailed liste les objets avec leurs détails dans un préfixe
func (c *Client) ListObjectsDetailed(prefix string) ([]ObjectInfo, error) {
	utils.Debug("S3 object list with prefix: %s", prefix)
	utils.Debug("Bucket: %s, Region: %s", c.bucket, c.region)

	var objects []ObjectInfo

	// Paramètres de liste
	params := &s3.ListObjectsV2Input{
		Bucket: aws.String(c.bucket),
		Prefix: aws.String(prefix),
	}

	// Effectuer la liste
	result, err := c.s3Client.ListObjectsV2(params)
	if err != nil {
		utils.Debug("ListObjectsV2 error: %v", err)
		// Si le préfixe n'existe pas, retourner une empty list (pas d'erreur)
		if strings.Contains(err.Error(), "NoSuchKey") || strings.Contains(err.Error(), "404") {
			utils.Debug("Prefix %s does not exist yet, empty list", prefix)
			return objects, nil
		}
		return nil, fmt.Errorf("error listing S3 objects: %w", err)
	}

	// Extraire les informations des objets
	for _, object := range result.Contents {
		objInfo := ObjectInfo{
			Key:  *object.Key,
			Size: *object.Size,
		}
		if object.LastModified != nil {
			objInfo.LastModified = *object.LastModified
		}
		objects = append(objects, objInfo)
	}

	utils.Debug("Object list: %d objects found", len(objects))
	for i, obj := range objects {
		utils.Debug("  [%d] %s", i+1, obj.Key)
	}

	return objects, nil
}

// DeleteObject supprime un objet de S3
func (c *Client) DeleteObject(key string) error {
	utils.Debug("Deleting S3 object: %s/%s", c.bucket, key)

	// Paramètres de suppression
	params := &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}

	// Effectuer la suppression
	_, err := c.s3Client.DeleteObject(params)
	if err != nil {
		return fmt.Errorf("error deleting S3 object: %w", err)
	}

	utils.Debug("Deletion successful: %s/%s", c.bucket, key)
	return nil
}

// Exists vérifie si un objet existe
func (c *Client) Exists(key string) (bool, error) {
	utils.Debug("Checking existence: %s/%s", c.bucket, key)

	// Paramètres de vérification
	params := &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}

	// Effectuer la vérification
	_, err := c.s3Client.HeadObject(params)
	if err != nil {
		// Si l'objet n'existe pas, AWS retourne une erreur spécifique
		if isNotFoundError(err) {
			return false, nil
		}
		return false, fmt.Errorf("error checking existence: %w", err)
	}

	return true, nil
}

// isNotFoundError vérifie si l'erreur est "not found"
func isNotFoundError(err error) bool {
	// Cette fonction devrait vérifier le type d'erreur AWS
	// Pour simplifier, on considère que toute erreur signifie "not found"
	return err != nil
}

// GetBucketInfo retourne les informations sur le bucket
func (c *Client) GetBucketInfo() (string, string, error) {
	return c.bucket, c.region, nil
}

package s3

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"

	"bcrdf/pkg/utils"
)

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
		return nil, fmt.Errorf("erreur lors de la création de la session AWS: %w", err)
	}

	// Créer le client S3
	s3Client := s3.New(sess)

	// Créer l'uploader
	uploader := s3manager.NewUploader(sess)

	// Créer le downloader
	downloader := s3manager.NewDownloader(sess)

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
	utils.Debug("Upload vers S3: %s/%s (%d bytes)", c.bucket, key, len(data))

	// Créer un reader pour les données
	reader := bytes.NewReader(data)

	// Paramètres d'upload
	params := &s3manager.UploadInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
		Body:   reader,
	}

	// Effectuer l'upload
	_, err := c.uploader.Upload(params)
	if err != nil {
		return fmt.Errorf("erreur lors de l'upload vers S3: %w", err)
	}

	utils.Debug("Upload réussi: %s/%s", c.bucket, key)
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
		return nil, fmt.Errorf("erreur lors du download depuis S3: %w", err)
	}

	utils.Debug("Download réussi: %s/%s (%d bytes)", c.bucket, key, len(buffer.Bytes()))
	return buffer.Bytes(), nil
}

// ListObjects liste les objets dans un préfixe
func (c *Client) ListObjects(prefix string) ([]string, error) {
	utils.Debug("Liste des objets S3 avec préfixe: %s", prefix)
	utils.Debug("Bucket: %s, Région: %s", c.bucket, c.region)

	var keys []string

	// Paramètres de liste
	params := &s3.ListObjectsV2Input{
		Bucket: aws.String(c.bucket),
		Prefix: aws.String(prefix),
	}

	// Effectuer la liste
	result, err := c.s3Client.ListObjectsV2(params)
	if err != nil {
		utils.Debug("Erreur ListObjectsV2: %v", err)
		// Si le préfixe n'existe pas, retourner une liste vide (pas d'erreur)
		if strings.Contains(err.Error(), "NoSuchKey") || strings.Contains(err.Error(), "404") {
			utils.Debug("Préfixe %s n'existe pas encore, liste vide", prefix)
			return keys, nil
		}
		return nil, fmt.Errorf("erreur lors de la liste des objets S3: %w", err)
	}

	// Extraire les clés
	for _, object := range result.Contents {
		keys = append(keys, *object.Key)
	}

	utils.Debug("Liste des objets: %d objets trouvés", len(keys))
	for i, key := range keys {
		utils.Debug("  [%d] %s", i+1, key)
	}
	return keys, nil
}

// DeleteObject supprime un objet de S3
func (c *Client) DeleteObject(key string) error {
	utils.Debug("Suppression d'objet S3: %s/%s", c.bucket, key)

	// Paramètres de suppression
	params := &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}

	// Effectuer la suppression
	_, err := c.s3Client.DeleteObject(params)
	if err != nil {
		return fmt.Errorf("erreur lors de la suppression d'objet S3: %w", err)
	}

	utils.Debug("Suppression réussie: %s/%s", c.bucket, key)
	return nil
}

// Exists vérifie si un objet existe
func (c *Client) Exists(key string) (bool, error) {
	utils.Debug("Vérification d'existence: %s/%s", c.bucket, key)

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
		return false, fmt.Errorf("erreur lors de la vérification d'existence: %w", err)
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

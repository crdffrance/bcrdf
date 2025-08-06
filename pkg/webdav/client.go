package webdav

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"bcrdf/pkg/utils"
)

// Client représente un client WebDAV
type Client struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
}

// ObjectInfo représente les informations d'un objet WebDAV
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
}

// Structures XML pour parser les réponses PROPFIND
type MultiStatus struct {
	XMLName   xml.Name   `xml:"multistatus"`
	Responses []Response `xml:"response"`
}

type Response struct {
	XMLName   xml.Name   `xml:"response"`
	Href      string     `xml:"href"`
	PropStats []PropStat `xml:"propstat"`
}

type PropStat struct {
	XMLName xml.Name `xml:"propstat"`
	Prop    Prop     `xml:"prop"`
	Status  string   `xml:"status"`
}

type Prop struct {
	XMLName       xml.Name     `xml:"prop"`
	DisplayName   string       `xml:"displayname"`
	ContentLength string       `xml:"getcontentlength"`
	LastModified  string       `xml:"getlastmodified"`
	ResourceType  ResourceType `xml:"resourcetype"`
}

type ResourceType struct {
	XMLName    xml.Name  `xml:"resourcetype"`
	Collection *struct{} `xml:"collection"`
}

// NewClient crée un nouveau client WebDAV
func NewClient(baseURL, username, password string) (*Client, error) {
	// Valider l'URL
	if _, err := url.Parse(baseURL); err != nil {
		return nil, fmt.Errorf("URL WebDAV invalide: %w", err)
	}

	// S'assurer que l'URL se termine par /
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	return &Client{
		baseURL:  baseURL,
		username: username,
		password: password,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// Upload télécharge un fichier vers WebDAV
func (c *Client) Upload(key string, data []byte) error {
	utils.Debug("Upload vers WebDAV: %s (%d bytes)", key, len(data))

	url := c.baseURL + key

	// Créer les répertoires parents si nécessaire
	if err := c.ensureDirectory(path.Dir(key)); err != nil {
		return fmt.Errorf("erreur lors de la création du répertoire: %w", err)
	}

	req, err := http.NewRequest("PUT", url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("erreur lors de la création de la requête: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("erreur lors de l'upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("échec de l'upload (status %d): %s", resp.StatusCode, string(body))
	}

	utils.Debug("Upload successful: %s", key)
	return nil
}

// Download télécharge un fichier depuis WebDAV
func (c *Client) Download(key string) ([]byte, error) {
	utils.Debug("Download depuis WebDAV: %s", key)

	url := c.baseURL + key

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la création de la requête: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erreur lors du download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("fichier non trouvé: %s", key)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("échec du download (status %d): %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la lecture des données: %w", err)
	}

	utils.Debug("Download successful: %s (%d bytes)", key, len(data))
	return data, nil
}

// DeleteObject supprime un fichier WebDAV
func (c *Client) DeleteObject(key string) error {
	utils.Debug("Suppression d'objet WebDAV: %s", key)

	url := c.baseURL + key

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("erreur lors de la création de la requête: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("erreur lors de la suppression: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		// Le fichier n'existe pas, considérer comme un succès
		utils.Debug("File already deleted or non-existent: %s", key)
		return nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("échec de la suppression (status %d): %s", resp.StatusCode, string(body))
	}

	utils.Debug("Deletion successful: %s", key)
	return nil
}

// ListObjects liste les objets avec un préfixe donné
func (c *Client) ListObjects(prefix string) ([]ObjectInfo, error) {
	utils.Debug("WebDAV object list with prefix: %s", prefix)

	url := c.baseURL + prefix

	// Utiliser PROPFIND pour lister les fichiers
	propfindXML := `<?xml version="1.0" encoding="utf-8" ?>
<D:propfind xmlns:D="DAV:">
    <D:prop>
        <D:displayname/>
        <D:getcontentlength/>
        <D:getlastmodified/>
        <D:resourcetype/>
    </D:prop>
</D:propfind>`

	req, err := http.NewRequest("PROPFIND", url, strings.NewReader(propfindXML))
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la création de la requête: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", "application/xml")
	req.Header.Set("Depth", "1")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la liste: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		// Le répertoire n'existe pas, retourner une empty list
		utils.Debug("Directory does not exist: %s", prefix)
		return []ObjectInfo{}, nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("échec de la liste (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la lecture de la réponse: %w", err)
	}

	// Pour l'instant, retourner une empty list
	// TODO: Parser la réponse XML PROPFIND
	objects := c.parseProFindResponse(string(body), prefix)

	utils.Debug("Object list: %d objects found", len(objects))
	for i, obj := range objects {
		utils.Debug("  [%d] %s", i+1, obj.Key)
	}

	return objects, nil
}

// ensureDirectory crée les répertoires parents si nécessaire
func (c *Client) ensureDirectory(dirPath string) error {
	if dirPath == "" || dirPath == "." {
		return nil
	}

	// Créer récursivement les répertoires parents
	parts := strings.Split(strings.Trim(dirPath, "/"), "/")
	currentPath := ""

	for _, part := range parts {
		if part == "" {
			continue
		}

		if currentPath != "" {
			currentPath += "/"
		}
		currentPath += part

		if err := c.createDirectory(currentPath); err != nil {
			return fmt.Errorf("erreur lors de la création du répertoire %s: %w", currentPath, err)
		}
	}

	return nil
}

// createDirectory crée un répertoire unique
func (c *Client) createDirectory(dirPath string) error {
	url := c.baseURL + dirPath + "/"

	utils.Debug("Creating directory WebDAV: %s", url)

	req, err := http.NewRequest("MKCOL", url, nil)
	if err != nil {
		return fmt.Errorf("erreur lors de la création de la requête MKCOL: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("erreur lors de la création du répertoire: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 201:
		utils.Debug("Directory created: %s", dirPath)
		return nil
	case 405:
		// Méthode non autorisée - probablement que le répertoire existe déjà
		utils.Debug("Directory already exists: %s", dirPath)
		return nil
	case 409:
		// Conflit - le répertoire parent n'existe peut-être pas, ou le répertoire existe déjà
		utils.Debug("Conflict during creation (directory probably exists): %s", dirPath)
		return nil
	default:
		body, _ := io.ReadAll(resp.Body)
		utils.Debug("Directory creation error (status %d): %s", resp.StatusCode, string(body))
		return fmt.Errorf("échec de la création du répertoire (status %d)", resp.StatusCode)
	}
}

// parseProFindResponse parse une réponse PROPFIND XML
func (c *Client) parseProFindResponse(xmlBody, prefix string) []ObjectInfo {
	var objects []ObjectInfo

	utils.Debug("Parsing response PROPFIND XML (%d bytes)", len(xmlBody))

	var multiStatus MultiStatus
	if err := xml.Unmarshal([]byte(xmlBody), &multiStatus); err != nil {
		utils.Debug("Erreur parsing XML: %v", err)
		return objects
	}

	utils.Debug("Found %d responses in XML", len(multiStatus.Responses))

	for _, response := range multiStatus.Responses {
		// Trouver le PropStat avec status 200 OK
		var validPropStat *PropStat
		for _, propStat := range response.PropStats {
			if strings.Contains(propStat.Status, "200") {
				validPropStat = &propStat
				break
			}
		}

		if validPropStat == nil {
			utils.Debug("Ignored (no 200 status)): %s", response.Href)
			continue
		}

		// Ignorer les répertoires (collections)
		if validPropStat.Prop.ResourceType.Collection != nil {
			utils.Debug("Ignored (directory)): %s", response.Href)
			continue
		}

		// Extraire le nom du fichier à partir de l'href
		href := strings.TrimSuffix(response.Href, "/")

		// Enlever l'URL de base pour obtenir le chemin relatif
		baseURL, _ := url.Parse(c.baseURL)
		if baseURL != nil && strings.HasPrefix(href, baseURL.Path) {
			href = strings.TrimPrefix(href, baseURL.Path)
		}

		// Décoder l'URL
		if decodedHref, err := url.QueryUnescape(href); err == nil {
			href = decodedHref
		}

		// Ignorer si does not start with le préfixe
		if !strings.HasPrefix(href, prefix) {
			utils.Debug("Ignored (prefix)): %s (does not start with %s)", href, prefix)
			continue
		}

		// Parser la taille
		var size int64
		if validPropStat.Prop.ContentLength != "" {
			if parsedSize, err := strconv.ParseInt(validPropStat.Prop.ContentLength, 10, 64); err == nil {
				size = parsedSize
			}
		}

		// Parser la date de modification
		var lastModified time.Time
		if validPropStat.Prop.LastModified != "" {
			// Format WebDAV: "Mon, 02 Jan 2006 15:04:05 GMT"
			if parsed, err := time.Parse(time.RFC1123, validPropStat.Prop.LastModified); err == nil {
				lastModified = parsed
			}
		}

		obj := ObjectInfo{
			Key:          href,
			Size:         size,
			LastModified: lastModified,
		}

		objects = append(objects, obj)
		utils.Debug("Added: %s (%d bytes)", obj.Key, obj.Size)
	}

	utils.Debug("XML parsing completed: %d objects extracted", len(objects))
	return objects
}

// TestConnectivity teste la connectivité WebDAV
func (c *Client) TestConnectivity() error {
	// Tester en faisant un PROPFIND sur la racine
	req, err := http.NewRequest("PROPFIND", c.baseURL, strings.NewReader(`<?xml version="1.0"?><propfind xmlns="DAV:"><prop><resourcetype/></prop></propfind>`))
	if err != nil {
		return fmt.Errorf("erreur lors de la création de la requête de test: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", "application/xml")
	req.Header.Set("Depth", "0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("impossible de se connecter au serveur WebDAV: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fmt.Errorf("authentification échouée - vérifiez vos identifiants")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("erreur de connectivité WebDAV (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

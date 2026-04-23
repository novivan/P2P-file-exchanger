package tracker_client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
)

type ManifestMeta struct {
	ID          uuid.UUID `json:"ID"`
	Name        string    `json:"Name"`
	Description string    `json:"Description"`
	CreatedAt   time.Time `json:"CreatedAt"`
}

type PeerInfo struct {
	ID       uuid.UUID `json:"ID"`
	Address  string    `json:"Address"`
	LastSeen time.Time `json:"LastSeen"`
}

type SearchResult struct {
	ID          uuid.UUID `json:"ID"`
	Name        string    `json:"Name"`
	Description string    `json:"Description"`
	Score       float32   `json:"Score"`
	CosineScore float32   `json:"CosineScore"`
	LLMScore    float32   `json:"LLMScore"`
	Explanation string    `json:"Explanation"`
}

type SearchRequest struct {
	Query string `json:"query"`
	TopK  int    `json:"top_k"`
}

type SearchResponse struct {
	Results []SearchResult `json:"results"`
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

func (c *Client) UploadManifest(id uuid.UUID, name, description string, data []byte) error {
	q := url.Values{}
	q.Set("id", id.String())
	q.Set("name", name)
	q.Set("description", description)
	u := c.baseURL + "/manifest?" + q.Encode()
	resp, err := c.httpClient.Post(u, "application/octet-stream", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("Error in UploadManifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("UploadManifest: tracker returned %d: %s", resp.StatusCode, body)
	}
	return nil
}

func (c *Client) GetManifest(id uuid.UUID) ([]byte, error) {
	url := fmt.Sprintf("%s/manifest/%s", c.baseURL, id.String())
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("Error in GetManifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GetManifest: tracker returned %d: %s", resp.StatusCode, body)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("GetManifest: read body: %w", err)
	}
	return data, nil
}

func (c *Client) ListManifests() ([]ManifestMeta, error) {
	url := fmt.Sprintf("%s/manifests", c.baseURL)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("Error in ListManifests: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ListManifests: tracker returned %d: %s", resp.StatusCode, body)
	}

	var metas []ManifestMeta
	if err := json.NewDecoder(resp.Body).Decode(&metas); err != nil {
		return nil, fmt.Errorf("ListManifests: decode: %w", err)
	}
	return metas, nil
}

func (c *Client) RegisterPeer(peerID uuid.UUID, address string) error {
	body, _ := json.Marshal(map[string]string{
		"id":      peerID.String(),
		"address": address,
	})

	url := fmt.Sprintf("%s/peer", c.baseURL)
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("Error in RegisterPeer: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("RegisterPeer: tracker returned %d: %s", resp.StatusCode, b)
	}
	return nil
}

func (c *Client) Announce(manifestID uuid.UUID, peerID uuid.UUID) error {
	body, _ := json.Marshal(map[string]string{
		"manifest_id": manifestID.String(),
		"peer_id":     peerID.String(),
	})

	url := fmt.Sprintf("%s/announce", c.baseURL)
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("Error in Announce: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Announce: tracker returned %d: %s", resp.StatusCode, b)
	}
	return nil
}

func (c *Client) GetSeeders(manifestID uuid.UUID) ([]PeerInfo, error) {
	url := fmt.Sprintf("%s/peers/%s", c.baseURL, manifestID.String())
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("Error in GetSeeders: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GetSeeders: tracker returned %d: %s", resp.StatusCode, b)
	}

	var peers []PeerInfo
	if err := json.NewDecoder(resp.Body).Decode(&peers); err != nil {
		return nil, fmt.Errorf("GetSeeders: decode: %w", err)
	}
	return peers, nil
}

func (c *Client) Search(query string, topK int) ([]SearchResult, error) {
	req := SearchRequest{Query: query, TopK: topK}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("Search: marshal: %w", err)
	}

	url := fmt.Sprintf("%s/search", c.baseURL)
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("Search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Search: tracker returned %d: %s", resp.StatusCode, b)
	}

	var result SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("Search response -  decode: %w", err)
	}

	return result.Results, nil
}

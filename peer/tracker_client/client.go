package tracker_client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type ManifestMeta struct {
	ID        uuid.UUID `json:"ID"`
	Name      string    `json:"Name"`
	CreatedAt time.Time `json:"CreatedAt"`
}

type PeerInfo struct {
	ID       uuid.UUID `json:"ID"`
	Address  string    `json:"Address"`
	LastSeen time.Time `json:"LastSeen"`
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) UploadManifest(id uuid.UUID, name string, data []byte) error {
	url := fmt.Sprintf("%s/manifest?id=%s&name=%s", c.baseURL, id.String(), name)
	resp, err := c.httpClient.Post(url, "application/octet-stream", bytes.NewReader(data))
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

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type ipfsClient struct {
	base   string
	client *http.Client
}

func newIPFSClient(apiBase string) *ipfsClient {
	return &ipfsClient{
		base:   apiBase,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

// PinAdd pins a CID. The IPFS daemon fetches and stores it.
func (c *ipfsClient) PinAdd(cid string) error {
	url := fmt.Sprintf("%s/api/v0/pin/add?arg=%s&progress=false", c.base, cid)
	resp, err := c.client.Post(url, "application/json", nil)
	if err != nil {
		return fmt.Errorf("pin add request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("IPFS API %d: %s", resp.StatusCode, body)
	}
	return nil
}

// PinRm unpins a CID (best-effort).
func (c *ipfsClient) PinRm(cid string) {
	url := fmt.Sprintf("%s/api/v0/pin/rm?arg=%s", c.base, cid)
	resp, _ := c.client.Post(url, "application/json", nil)
	if resp != nil {
		resp.Body.Close()
	}
}

// PeerID returns the peer ID of the local IPFS node.
func (c *ipfsClient) PeerID() (string, error) {
	resp, err := c.client.Post(c.base+"/api/v0/id", "application/json", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result struct {
		ID string `json:"ID"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.ID, nil
}

// Ping checks if the IPFS daemon is reachable.
func (c *ipfsClient) Ping() bool {
	resp, err := c.client.Post(c.base+"/api/v0/version", "application/json", nil)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

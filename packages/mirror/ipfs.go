package main

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

// GetTar fetches CID content as a tar from the IPFS API and extracts it to dstDir.
// DstDir is cleared first. Safe against path traversal.
func (c *ipfsClient) GetTar(cid, dstDir string) error {
	url := fmt.Sprintf("%s/api/v0/get?arg=%s", c.base, cid)
	resp, err := c.client.Post(url, "application/json", nil)
	if err != nil {
		return fmt.Errorf("ipfs get: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ipfs get returned %d: %s", resp.StatusCode, body)
	}

	// Clear destination
	if err := os.RemoveAll(dstDir); err != nil {
		return fmt.Errorf("clear serve dir: %w", err)
	}
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("create serve dir: %w", err)
	}

	tr := tar.NewReader(resp.Body)
	var stripPrefix string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}

		// Strip the top-level directory name (CID or dist/) from all paths.
		if stripPrefix == "" {
			parts := strings.SplitN(hdr.Name, "/", 2)
			if len(parts) > 0 {
				stripPrefix = parts[0] + "/"
			}
		}
		rel := strings.TrimPrefix(hdr.Name, stripPrefix)
		if rel == "" || strings.HasPrefix(rel, ".") {
			continue
		}

		target := filepath.Join(dstDir, rel)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dstDir)+string(os.PathSeparator)) {
			continue // path traversal attempt
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0755)
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return fmt.Errorf("create %s: %w", rel, err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("write %s: %w", rel, err)
			}
			f.Close()
		}
	}
	return nil
}

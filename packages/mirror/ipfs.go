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

// unsafeServeDirs are paths GetTar refuses to operate on, regardless of config.
// Defends against misconfigured SERVE_DIR (e.g. SERVE_DIR=/etc).
var unsafeServeDirs = map[string]bool{
	"/": true, "/etc": true, "/usr": true, "/var": true, "/bin": true,
	"/sbin": true, "/lib": true, "/lib64": true, "/root": true,
	"/home": true, "/boot": true, "/dev": true, "/proc": true,
	"/sys": true, "/opt": true, "/srv": true, "/tmp": true,
}

func validateServeDir(p string) error {
	clean := filepath.Clean(p)
	if clean == "" || clean == "." || clean == "/" {
		return fmt.Errorf("refusing to operate on unsafe serve dir: %q", p)
	}
	if unsafeServeDirs[clean] {
		return fmt.Errorf("refusing to operate on system serve dir: %q", p)
	}
	return nil
}

// GetTar fetches CID content as a tar from the IPFS API and writes it to dstDir.
// Extraction happens in a sibling staging subdir; only after the tar is fully
// extracted are old entries removed and new ones promoted. If extraction fails
// midway, the live serve dir is untouched.
//
// Path traversal entries in the tar are skipped silently.
func (c *ipfsClient) GetTar(cid, dstDir string) error {
	if err := validateServeDir(dstDir); err != nil {
		return err
	}

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

	// Ensure dstDir exists, but do NOT remove it — it may be a bind mount or
	// Docker volume mountpoint, and removing the mountpoint itself breaks the
	// mount or fails with EBUSY.
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("create serve dir: %w", err)
	}

	// Extract into a staging subdirectory inside dstDir so a failed extraction
	// leaves the live content alone.
	staging := filepath.Join(dstDir, ".cjp-staging")
	if err := os.RemoveAll(staging); err != nil {
		return fmt.Errorf("clear staging: %w", err)
	}
	if err := os.MkdirAll(staging, 0755); err != nil {
		return fmt.Errorf("create staging: %w", err)
	}
	// Cleanup staging on any error path.
	success := false
	defer func() {
		if !success {
			os.RemoveAll(staging)
		}
	}()

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

		target := filepath.Join(staging, rel)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(staging)+string(os.PathSeparator)) {
			continue // path traversal attempt
		}

		// Mask file modes from untrusted tar — only honour permission bits we trust.
		switch hdr.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0755)
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
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

	// Promote: remove old top-level entries (except staging itself), then move
	// each new entry from staging into dstDir. Rename is atomic per-file on
	// POSIX so each file flips from old to new instantly.
	oldEntries, err := os.ReadDir(dstDir)
	if err != nil {
		return fmt.Errorf("read serve dir: %w", err)
	}
	for _, e := range oldEntries {
		if e.Name() == ".cjp-staging" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dstDir, e.Name())); err != nil {
			return fmt.Errorf("remove old %s: %w", e.Name(), err)
		}
	}
	newEntries, err := os.ReadDir(staging)
	if err != nil {
		return fmt.Errorf("read staging: %w", err)
	}
	for _, e := range newEntries {
		src := filepath.Join(staging, e.Name())
		dst := filepath.Join(dstDir, e.Name())
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("promote %s: %w", e.Name(), err)
		}
	}
	if err := os.Remove(staging); err != nil {
		// Non-fatal — staging should be empty; if not, next run wipes it.
		os.RemoveAll(staging)
	}
	success = true
	return nil
}

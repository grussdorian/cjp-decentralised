package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Sig struct {
	Signer    string `json:"signer"`
	Signature string `json:"signature"`
}

type Latest struct {
	CID       string `json:"cid"`
	Version   int64  `json:"version"`
	Timestamp int64  `json:"timestamp"`
	Note      string `json:"note,omitempty"`
	// Legacy single-sig fields (v1 format).
	Signer    string `json:"signer,omitempty"`
	Signature string `json:"signature,omitempty"`
	// Multi-sig: each trusted signer adds an entry.
	Signatures []Sig `json:"signatures,omitempty"`
}

// allSigs normalises both formats into a single slice.
func (l *Latest) allSigs() []Sig {
	if len(l.Signatures) > 0 {
		return l.Signatures
	}
	if l.Signer != "" {
		return []Sig{{Signer: l.Signer, Signature: l.Signature}}
	}
	return nil
}

// fetchLatest tries each URL in order and returns the first successfully parsed Latest.
func fetchLatest(urls []string) (*Latest, string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	var lastErr error
	for _, u := range urls {
		l, err := fetchOne(client, u)
		if err != nil {
			lastErr = err
			continue
		}
		return l, u, nil
	}
	return nil, "", fmt.Errorf("all sources failed; last error: %v", lastErr)
}

func fetchOne(client *http.Client, url string) (*Latest, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil, err
	}
	var l Latest
	if err := json.Unmarshal(body, &l); err != nil {
		return nil, fmt.Errorf("parse latest.json: %w", err)
	}
	if l.CID == "" {
		return nil, fmt.Errorf("empty CID in latest.json")
	}
	return &l, nil
}

// buildLatestURLs combines IPNS resolution URL + clearweb fallbacks.
func buildLatestURLs(cfg Config) []string {
	var urls []string
	if cfg.IPNSName != "" {
		urls = append(urls,
			fmt.Sprintf("https://ipfs.io/ipns/%s/latest.json", cfg.IPNSName),
			fmt.Sprintf("https://cloudflare-ipfs.com/ipns/%s/latest.json", cfg.IPNSName),
		)
	}
	urls = append(urls, cfg.FallbackURLs...)
	return urls
}

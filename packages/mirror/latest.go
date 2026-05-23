package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Latest struct {
	CID       string `json:"cid"`
	Version   int64  `json:"version"`
	Timestamp int64  `json:"timestamp"`
	Note      string `json:"note"`
	Signer    string `json:"signer"`
	Signature string `json:"signature"`
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

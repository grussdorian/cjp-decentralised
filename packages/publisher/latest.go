package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Latest mirrors the schema of latest.json.
type Latest struct {
	CID       string `json:"cid"`
	Version   int64  `json:"version"`
	Timestamp int64  `json:"timestamp"`
	Note      string `json:"note"`
	Signer    string `json:"signer"`
	Signature string `json:"signature"`
}

func readLatest(path string) (*Latest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var l Latest
	if err := json.Unmarshal(raw, &l); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &l, nil
}

func writeLatest(path string, l *Latest) error {
	b, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0644)
}

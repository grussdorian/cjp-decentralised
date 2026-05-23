package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Sig struct {
	Signer    string `json:"signer"`
	Signature string `json:"signature"`
}

// Latest mirrors the schema of latest.json.
// Supports both the legacy single-sig format (v1) and the multi-sig format (v2+).
type Latest struct {
	CID       string `json:"cid"`
	Version   int64  `json:"version"`
	Timestamp int64  `json:"timestamp"`
	Note      string `json:"note,omitempty"`
	// Legacy fields — present only when reading old format; never written by new code.
	Signer    string `json:"signer,omitempty"`
	Signature string `json:"signature,omitempty"`
	// Multi-sig: each signer independently adds their entry.
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

// TrustedSigners mirrors trusted-signers.json.
type TrustedSigners struct {
	Threshold int      `json:"threshold"`
	Signers   []string `json:"signers"`
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

func readTrustedSigners(path string) (*TrustedSigners, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ts TrustedSigners
	if err := json.Unmarshal(raw, &ts); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if ts.Threshold <= 0 {
		ts.Threshold = 1
	}
	return &ts, nil
}

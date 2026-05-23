package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
)

type TrustedSigners struct {
	Signers []string `json:"signers"`
}

func loadSigners(path string) ([]ed25519.PublicKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read signers file: %w", err)
	}
	var ts TrustedSigners
	if err := json.Unmarshal(raw, &ts); err != nil {
		return nil, fmt.Errorf("parse signers file: %w", err)
	}
	keys := make([]ed25519.PublicKey, 0, len(ts.Signers))
	for _, h := range ts.Signers {
		b, err := hex.DecodeString(h)
		if err != nil || len(b) != 32 {
			return nil, fmt.Errorf("invalid pubkey in trusted-signers.json: %q", h)
		}
		keys = append(keys, ed25519.PublicKey(b))
	}
	return keys, nil
}

func signingMessage(cid string, version, timestamp int64) []byte {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s\n%d\n%d", cid, version, timestamp)))
	return h[:]
}

func verifyLatest(trustedKeys []ed25519.PublicKey, cid string, version, timestamp int64, signerHex, sigHex string) error {
	sig, err := hex.DecodeString(sigHex)
	if err != nil || len(sig) != 64 {
		return fmt.Errorf("invalid signature encoding")
	}
	msg := signingMessage(cid, version, timestamp)

	// Check that the signing pubkey is trusted
	signerBytes, err := hex.DecodeString(signerHex)
	if err != nil || len(signerBytes) != 32 {
		return fmt.Errorf("invalid signer pubkey encoding")
	}
	signer := ed25519.PublicKey(signerBytes)

	trusted := false
	for _, pk := range trustedKeys {
		if signer.Equal(pk) {
			trusted = true
			break
		}
	}
	if !trusted {
		return fmt.Errorf("signer %s is not in trusted-signers.json", signerHex[:16]+"...")
	}

	if !ed25519.Verify(signer, msg, sig) {
		return fmt.Errorf("signature verification failed")
	}
	return nil
}

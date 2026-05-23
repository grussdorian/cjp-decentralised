package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func cmdSign(args []string) {
	fs := flag.NewFlagSet("sign", flag.ExitOnError)
	keyPath    := fs.String("key",     filepath.Join(os.Getenv("HOME"), ".cjp", "signing.key"), "path to Ed25519 signing key")
	cid        := fs.String("cid",     "", "IPFS CID of the new site build (required)")
	version    := fs.Int64("version",  0,  "version number (auto-incremented if 0)")
	note       := fs.String("note",    "", "human-readable release note")
	latestPath := fs.String("latest",  "latest.json",         "path to latest.json to update")
	signersPath := fs.String("signers", "trusted-signers.json", "path to trusted-signers.json")
	fs.Parse(args)

	if *cid == "" {
		die("--cid is required")
	}

	sk, err := loadPrivateKey(*keyPath)
	if err != nil {
		die("load key: %v", err)
	}
	pkHex := hex.EncodeToString(sk.Public().(ed25519.PublicKey))

	ts, _ := readTrustedSigners(*signersPath)
	if ts == nil {
		ts = &TrustedSigners{Threshold: 1}
	}

	existing, _ := readLatest(*latestPath)

	var l *Latest
	if existing != nil && existing.CID == *cid {
		// Co-signing an existing round: preserve CID, version, timestamp.
		// Migrate legacy single-sig fields to the array.
		l = existing
		l.Signer    = ""
		l.Signature = ""
	} else {
		// New CID — start a fresh signing round.
		v := *version
		if v <= 0 {
			if existing != nil {
				v = existing.Version + 1
			} else {
				v = 1
			}
		}
		l = &Latest{
			CID:       *cid,
			Version:   v,
			Timestamp: time.Now().Unix(),
			Note:      *note,
		}
	}

	// Prevent double-signing.
	for _, s := range l.Signatures {
		if s.Signer == pkHex {
			die("key %s...%s already signed this CID", pkHex[:8], pkHex[len(pkHex)-8:])
		}
	}

	sig := signLatest(sk, l.CID, l.Version, l.Timestamp)
	l.Signatures = append(l.Signatures, Sig{Signer: pkHex, Signature: sig})

	if err := writeLatest(*latestPath, l); err != nil {
		die("write latest.json: %v", err)
	}

	count     := len(l.Signatures)
	threshold := ts.Threshold
	total     := len(ts.Signers)

	fmt.Printf("Signed latest.json:\n")
	fmt.Printf("  CID        : %s\n", l.CID)
	fmt.Printf("  Version    : %d\n", l.Version)
	fmt.Printf("  Timestamp  : %d\n", l.Timestamp)
	fmt.Printf("  This key   : %s\n", pkHex)
	fmt.Printf("  Signatures : %d/%d  (threshold: %d)\n\n", count, total, threshold)

	if count >= threshold {
		fmt.Printf("✓ Threshold met — ready to publish.\n")
		fmt.Println("Next: git add latest.json && git commit && publisher publish --latest", *latestPath)
	} else {
		remaining := threshold - count
		fmt.Printf("⏳ %d more signature(s) needed before publishing.\n", remaining)
		fmt.Printf("Share latest.json with other signers; they run:\n")
		fmt.Printf("  publisher sign --key <key> --cid %s --latest %s\n", l.CID, *latestPath)
	}
}

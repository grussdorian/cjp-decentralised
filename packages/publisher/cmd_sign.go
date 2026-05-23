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
	keyPath := fs.String("key", filepath.Join(os.Getenv("HOME"), ".cjp", "signing.key"), "path to Ed25519 signing key")
	cid := fs.String("cid", "", "IPFS CID of the new site build (required)")
	version := fs.Int64("version", 0, "version number (must be > current version)")
	note := fs.String("note", "", "human-readable note for this release")
	latestPath := fs.String("latest", "latest.json", "path to latest.json to write")
	fs.Parse(args)

	if *cid == "" {
		die("--cid is required")
	}
	if *version <= 0 {
		if existing, err := readLatest(*latestPath); err == nil {
			*version = existing.Version + 1
		} else {
			*version = 1
		}
	}

	sk, err := loadPrivateKey(*keyPath)
	if err != nil {
		die("load key: %v", err)
	}

	ts := time.Now().Unix()
	sig := signLatest(sk, *cid, *version, ts)
	pkHex := hex.EncodeToString(sk.Public().(ed25519.PublicKey))

	l := &Latest{
		CID:       *cid,
		Version:   *version,
		Timestamp: ts,
		Note:      *note,
		Signer:    pkHex,
		Signature: sig,
	}

	if err := writeLatest(*latestPath, l); err != nil {
		die("write latest.json: %v", err)
	}

	fmt.Printf("Signed latest.json:\n")
	fmt.Printf("  CID       : %s\n", l.CID)
	fmt.Printf("  Version   : %d\n", l.Version)
	fmt.Printf("  Timestamp : %d\n", l.Timestamp)
	fmt.Printf("  Signer    : %s\n", l.Signer)
	fmt.Printf("  Signature : %s\n\n", l.Signature[:16]+"...")
	fmt.Println("Next: git add latest.json && git commit -m \"chore: publish v" + fmt.Sprint(l.Version) + "\"")
	fmt.Println("Then: publisher publish --latest", *latestPath)
}

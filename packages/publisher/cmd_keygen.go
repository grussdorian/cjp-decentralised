package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nbd-wtf/go-nostr"
)

func cmdKeygen(args []string) {
	fs := flag.NewFlagSet("keygen", flag.ExitOnError)
	out := fs.String("out", filepath.Join(os.Getenv("HOME"), ".cjp"), "directory to write key files")
	fs.Parse(args)

	if err := os.MkdirAll(*out, 0700); err != nil {
		die("create dir: %v", err)
	}

	// Ed25519 signing key (for latest.json)
	_, sk, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		die("generate ed25519 key: %v", err)
	}
	seed := sk.Seed()
	pk := sk.Public().(ed25519.PublicKey)

	skPath := filepath.Join(*out, "signing.key")
	pkPath := filepath.Join(*out, "signing.pub")
	if err := os.WriteFile(skPath, []byte(hex.EncodeToString(seed)+"\n"), 0600); err != nil {
		die("write signing.key: %v", err)
	}
	if err := os.WriteFile(pkPath, []byte(hex.EncodeToString(pk)+"\n"), 0644); err != nil {
		die("write signing.pub: %v", err)
	}

	// secp256k1 Nostr key (for broadcasting update events)
	nostrSK := nostr.GeneratePrivateKey()
	nostrPK, err := nostr.GetPublicKey(nostrSK)
	if err != nil {
		die("derive nostr pubkey: %v", err)
	}
	nostrSKPath := filepath.Join(*out, "nostr.key")
	nostrPKPath := filepath.Join(*out, "nostr.pub")
	if err := os.WriteFile(nostrSKPath, []byte(nostrSK+"\n"), 0600); err != nil {
		die("write nostr.key: %v", err)
	}
	if err := os.WriteFile(nostrPKPath, []byte(nostrPK+"\n"), 0644); err != nil {
		die("write nostr.pub: %v", err)
	}

	fmt.Println("Keys generated:")
	fmt.Printf("  Ed25519 signing key : %s\n", skPath)
	fmt.Printf("  Ed25519 public key  : %s\n", pkPath)
	fmt.Printf("  Nostr private key   : %s\n", nostrSKPath)
	fmt.Printf("  Nostr public key    : %s\n\n", nostrPKPath)
	fmt.Printf("Add this to trusted-signers.json:\n  %s\n\n", hex.EncodeToString(pk))
	fmt.Printf("Add this Nostr pubkey to party-keys.txt for form encryption:\n  npub: %s\n", nostrPK)
}

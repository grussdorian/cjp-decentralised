package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func cmdPublish(args []string) {
	fs := flag.NewFlagSet("publish", flag.ExitOnError)
	latestPath := fs.String("latest", "latest.json", "path to signed latest.json")
	nostrKeyPath := fs.String("nostr-key", filepath.Join(os.Getenv("HOME"), ".cjp", "nostr.key"), "path to Nostr secp256k1 key")
	ipfsAPI := fs.String("ipfs-api", "http://localhost:5001", "IPFS HTTP API endpoint (empty to skip IPNS)")
	ipnsKey := fs.String("ipns-key", "self", "IPFS key name for IPNS publishing")
	seedDist := fs.String("seed-dist", "", "path to dist/ directory to add to IPFS (bootstrap first pin; requires ipfs in PATH)")
	fs.Parse(args)

	l, err := readLatest(*latestPath)
	if err != nil {
		die("read latest.json: %v", err)
	}
	if l.CID == "" || l.Signature == "" {
		die("latest.json is unsigned or empty — run 'publisher sign' first")
	}

	// 0. Seed content into IPFS (bootstrap step, optional)
	if *seedDist != "" {
		fmt.Printf("Seeding %s into IPFS...\n", *seedDist)
		out, err := exec.Command("ipfs", "add", "-r", "--cid-version=1", "--quieter", *seedDist).Output()
		if err != nil {
			die("ipfs add: %v", err)
		}
		got := strings.TrimSpace(string(out))
		if got != l.CID {
			die("CID mismatch: dist produced %s but latest.json has %s", got, l.CID)
		}
		fmt.Printf("  ✓ content seeded (%s)\n", got)
	}

	// 1. Publish to IPNS (optional)
	if *ipfsAPI != "" {
		fmt.Printf("Publishing to IPNS (key=%s)...\n", *ipnsKey)
		if err := ipnsPublish(*ipfsAPI, *ipnsKey, l.CID); err != nil {
			fmt.Fprintf(os.Stderr, "IPNS publish failed (continuing): %v\n", err)
		} else {
			fmt.Println("  ✓ IPNS updated")
		}
	}

	// 2. Broadcast Nostr update event
	fmt.Println("Broadcasting Nostr update event...")
	rawKey, err := os.ReadFile(*nostrKeyPath)
	if err != nil {
		die("read nostr key: %v", err)
	}
	nostrSK := strings.TrimSpace(string(rawKey))
	if err := broadcastUpdate(nostrSK, l); err != nil {
		die("nostr broadcast: %v", err)
	}

	fmt.Printf("\nDone. CID %s is live.\n", l.CID)
}

func ipnsPublish(apiBase, keyName, cid string) error {
	client := &http.Client{Timeout: 120 * time.Second}
	endpoint := fmt.Sprintf("%s/api/v0/name/publish", apiBase)
	params := url.Values{
		"arg":      {"/ipfs/" + cid},
		"key":      {keyName},
		"lifetime": {"87600h"},
		"ttl":      {"1h"},
	}
	resp, err := client.Post(endpoint+"?"+params.Encode(), "application/json", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("IPFS API returned %d", resp.StatusCode)
	}
	return nil
}

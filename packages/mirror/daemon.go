package main

import (
	"crypto/ed25519"
	"fmt"
	"log"
	"time"
)

type Daemon struct {
	cfg         Config
	ipfs        *ipfsClient
	ts          *TrustedSigners
	trustedKeys []ed25519.PublicKey
	state       *State
	peerID      string
}

func newDaemon(cfg Config) (*Daemon, error) {
	state, err := loadState(cfg.StateFile)
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}
	ensureNostrKey(state)

	ts, keys, err := loadSignersConfig(cfg.SignersFile)
	if err != nil {
		log.Printf("warning: %v — will reject all updates until signers are configured", err)
		ts   = &TrustedSigners{Threshold: 1}
		keys = nil
	}

	ipfs := newIPFSClient(cfg.IPFSApi)

	peerID := ""
	if ipfs.Ping() {
		peerID, _ = ipfs.PeerID()
		log.Printf("IPFS node online, peer ID: %s", peerID)
	} else {
		log.Printf("warning: IPFS daemon not reachable at %s", cfg.IPFSApi)
	}

	return &Daemon{
		cfg:         cfg,
		ipfs:        ipfs,
		ts:          ts,
		trustedKeys: keys,
		state:       state,
		peerID:      peerID,
	}, nil
}

// Run starts the poll loop and heartbeat ticker. Blocks until stop is closed.
func (d *Daemon) Run(stop <-chan struct{}) {
	log.Printf("Mirror daemon started (poll: %s, heartbeat: 30s)", d.cfg.PollInterval)

	d.poll()

	pollTicker := time.NewTicker(d.cfg.PollInterval)
	defer pollTicker.Stop()

	beatTicker := time.NewTicker(30 * time.Second)
	defer beatTicker.Stop()

	for {
		select {
		case <-pollTicker.C:
			d.poll()
		case <-beatTicker.C:
			d.heartbeat()
		case <-stop:
			log.Println("Daemon shutting down.")
			return
		}
	}
}

func (d *Daemon) heartbeat() {
	if d.state.PinnedCID == "" {
		return
	}
	latest := &Latest{CID: d.state.PinnedCID, Version: d.state.PinnedVer}
	d.sendHeartbeat(latest)
}

func (d *Daemon) poll() {
	urls := buildLatestURLs(d.cfg)
	latest, source, err := fetchLatest(urls)
	if err != nil {
		log.Printf("fetch latest.json: %v", err)
		return
	}
	log.Printf("Fetched latest.json v%d from %s (CID: %s)", latest.Version, source, latest.CID[:12]+"...")

	// Verify M-of-N threshold signatures
	if len(d.trustedKeys) == 0 {
		log.Println("No trusted signers configured — skipping pin")
		return
	}
	n, err := verifyThreshold(d.ts, d.trustedKeys, latest)
	if err != nil {
		log.Printf("Signature verification failed: %v — ignoring update", err)
		return
	}
	log.Printf("Verified %d/%d signatures (threshold: %d)", n, len(d.ts.Signers), d.ts.Threshold)

	// Already on this version?
	if latest.CID == d.state.PinnedCID {
		log.Printf("Already pinned v%d, nothing to do", latest.Version)
		d.sendHeartbeat(latest)
		return
	}
	if latest.Version <= d.state.PinnedVer && d.state.PinnedVer > 0 {
		log.Printf("Received v%d but already have v%d — ignoring downgrade", latest.Version, d.state.PinnedVer)
		return
	}

	// Pin new CID
	log.Printf("Pinning new CID %s (v%d)...", latest.CID, latest.Version)
	if !d.ipfs.Ping() {
		log.Println("IPFS daemon unreachable — cannot pin")
		return
	}
	if err := d.ipfs.PinAdd(latest.CID); err != nil {
		log.Printf("pin add failed: %v", err)
		return
	}
	log.Printf("Pinned v%d successfully", latest.Version)

	// Unpin old CID (best-effort, grace period)
	if d.state.PinnedCID != "" && d.state.PinnedCID != latest.CID {
		log.Printf("Unpinning old CID %s...", d.state.PinnedCID[:12]+"...")
		d.ipfs.PinRm(d.state.PinnedCID)
	}

	// Update state
	prev := d.state.PinnedCID
	d.state.PinnedCID = latest.CID
	d.state.PinnedVer = latest.Version
	if err := saveState(d.cfg.StateFile, d.state); err != nil {
		log.Printf("save state: %v", err)
	}
	_ = prev

	// Populate serve directory so clearweb mirrors auto-update.
	if d.cfg.ServeDir != "" && d.cfg.GatewayURL != "" {
		log.Printf("Populating serve dir %s from CID %s...", d.cfg.ServeDir, latest.CID)
		if err := d.ipfs.GetTar(latest.CID, d.cfg.ServeDir); err != nil {
			log.Printf("populate serve dir: %v", err)
		} else {
			log.Printf("Serve dir populated with v%d", latest.Version)
		}
	}

	d.sendHeartbeat(latest)
}

func (d *Daemon) sendHeartbeat(latest *Latest) {
	if d.state.NostrSK == "" {
		return
	}
	if err := broadcastHeartbeat(d.state.NostrSK, d.peerID, latest.CID, d.cfg.Country, d.cfg.MirrorURL, latest.Version); err != nil {
		log.Printf("heartbeat: %v", err)
	} else {
		log.Println("Heartbeat sent to Nostr")
	}
}

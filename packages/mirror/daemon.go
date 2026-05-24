package main

import (
	"crypto/ed25519"
	"fmt"
	"log"
	"sync"
	"time"
)

type Daemon struct {
	cfg         Config
	ipfs        *ipfsClient
	ts          *TrustedSigners
	trustedKeys []ed25519.PublicKey
	state       *State
	peerID      string

	// pollMu serialises poll() — TryLock means a tick fired while a previous
	// poll is still running (slow pin, GetTar over a slow link) is skipped
	// rather than racing it.
	pollMu sync.Mutex

	// stateMu guards mutations to *state from poll() and concurrent reads
	// from heartbeat(). poll() writes; heartbeat reads.
	stateMu sync.RWMutex
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

	pollTicker := time.NewTicker(d.cfg.PollInterval)
	defer pollTicker.Stop()

	beatTicker := time.NewTicker(30 * time.Second)
	defer beatTicker.Stop()

	// Fire initial poll in background so heartbeat ticker starts immediately.
	go d.poll()

	for {
		select {
		case <-pollTicker.C:
			go d.poll()
		case <-beatTicker.C:
			go d.heartbeat()
		case <-stop:
			log.Println("Daemon shutting down.")
			return
		}
	}
}

func (d *Daemon) heartbeat() {
	d.stateMu.RLock()
	cid := d.state.PinnedCID
	ver := d.state.PinnedVer
	d.stateMu.RUnlock()
	if cid == "" {
		return
	}
	d.sendHeartbeat(&Latest{CID: cid, Version: ver})
}

func (d *Daemon) poll() {
	// Skip if a previous poll is still in flight (slow pin or GetTar).
	if !d.pollMu.TryLock() {
		log.Println("poll already in progress, skipping this tick")
		return
	}
	defer d.pollMu.Unlock()

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

	d.stateMu.RLock()
	prevCID := d.state.PinnedCID
	prevVer := d.state.PinnedVer
	d.stateMu.RUnlock()

	// Already on this version?
	if latest.CID == prevCID {
		log.Printf("Already pinned v%d, nothing to do", latest.Version)
		d.sendHeartbeat(latest)
		return
	}
	if latest.Version <= prevVer && prevVer > 0 {
		log.Printf("Received v%d but already have v%d — ignoring downgrade", latest.Version, prevVer)
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
	if prevCID != "" && prevCID != latest.CID {
		log.Printf("Unpinning old CID %s...", prevCID[:12]+"...")
		d.ipfs.PinRm(prevCID)
	}

	// Update state
	d.stateMu.Lock()
	d.state.PinnedCID = latest.CID
	d.state.PinnedVer = latest.Version
	stateCopy := *d.state
	d.stateMu.Unlock()
	if err := saveState(d.cfg.StateFile, &stateCopy); err != nil {
		log.Printf("save state: %v", err)
	}

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
	d.stateMu.RLock()
	sk := d.state.NostrSK
	d.stateMu.RUnlock()
	if sk == "" {
		return
	}
	if err := broadcastHeartbeat(sk, d.peerID, latest.CID, d.cfg.Country, d.cfg.MirrorURL, latest.Version); err != nil {
		log.Printf("heartbeat: %v", err)
	} else {
		log.Println("Heartbeat sent to Nostr")
	}
}

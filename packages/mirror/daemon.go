package main

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"path/filepath"
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

	// pool holds long-lived WebSocket connections to heartbeat relays so each
	// beat is a tiny frame on an existing connection rather than a fresh
	// TLS+WSS handshake. CF rate-limits the handshake pattern after hours.
	pool *RelayPool

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

	// Pool prepends the bundled local relay (if configured) before the public
	// federation set. Local writes never fail; public writes are best-effort.
	relayURLs := []string{}
	if cfg.LocalRelay != "" {
		relayURLs = append(relayURLs, cfg.LocalRelay)
	}
	relayURLs = append(relayURLs, heartbeatRelays...)

	return &Daemon{
		cfg:         cfg,
		ipfs:        ipfs,
		ts:          ts,
		trustedKeys: keys,
		state:       state,
		peerID:      peerID,
		pool:        NewRelayPool(relayURLs),
	}, nil
}

// Run starts the poll loop and heartbeat ticker. Blocks until stop is closed.
func (d *Daemon) Run(stop <-chan struct{}) {
	log.Printf("Mirror daemon started (poll: %s, heartbeat: ~60s ±10s jittered)", d.cfg.PollInterval)

	pollTicker := time.NewTicker(d.cfg.PollInterval)
	defer pollTicker.Stop()

	// Jittered 50-70s heartbeat: time.NewTicker only jitters the *first* tick.
	// We reset the ticker after every beat so the cadence keeps shifting —
	// otherwise we lock into a fixed interval that CF's abuse detection can
	// latch onto across hours.
	beatTicker := time.NewTicker(jitterDuration(60*time.Second, 10*time.Second))
	defer beatTicker.Stop()

	// Periodically refresh propagation.json — distinct DHT providers can
	// change as nodes pin/unpin. Every 10 minutes is a reasonable balance
	// between freshness and DHT walk cost (each walk takes up to 30s).
	propTicker := time.NewTicker(10 * time.Minute)
	defer propTicker.Stop()

	// Fire initial poll in background so heartbeat ticker starts immediately.
	go d.poll()

	// Seed propagation.json shortly after the first poll settles. Without
	// this, propagation.json only exists after the next 10-min propTicker
	// fire (or after the next actual version bump), which can be many
	// minutes after startup for a steady-state mirror.
	go func() {
		time.Sleep(15 * time.Second)
		d.stateMu.RLock()
		cid := d.state.PinnedCID
		d.stateMu.RUnlock()
		if cid != "" {
			d.updatePropagation(cid)
		}
	}()

	for {
		select {
		case <-pollTicker.C:
			go d.poll()
		case <-beatTicker.C:
			go d.heartbeat()
			beatTicker.Reset(jitterDuration(60*time.Second, 10*time.Second))
		case <-propTicker.C:
			d.stateMu.RLock()
			cid := d.state.PinnedCID
			d.stateMu.RUnlock()
			if cid != "" {
				go d.updatePropagation(cid)
			}
		case <-stop:
			log.Println("Daemon shutting down.")
			d.pool.Close()
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

	// Already on this version? Heartbeat ticker will broadcast on its own
	// schedule; no need to fire an extra one from here.
	if latest.CID == prevCID {
		log.Printf("Already pinned v%d, nothing to do", latest.Version)
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

	// Walk the DHT for distinct providers of this CID and publish the count
	// to /propagation.json so the browser can show propagation evidence.
	// Async because findprovs can take ~30s; don't block the daemon.
	go d.updatePropagation(latest.CID)

	// Fire one immediate heartbeat after a successful version bump so the new
	// CID is visible to relays well before the next 30s tick. Subsequent
	// heartbeats are handled by the ticker.
	d.sendHeartbeat(latest)
}

// updatePropagation queries the DHT for providers of the CID and writes the
// result to the configured propagation target. Best-effort: errors are
// logged and any previous file is left intact.
func (d *Daemon) updatePropagation(cid string) {
	target := d.cfg.PropagationFile
	if target == "" && d.cfg.ServeDir != "" {
		target = filepath.Join(d.cfg.ServeDir, "propagation.json")
	}
	if target == "" {
		return
	}
	provs, err := d.ipfs.FindProvs(cid, 40, 30*time.Second)
	if err != nil {
		log.Printf("findprovs %s: %v", cid[:12]+"...", err)
		return
	}
	out := struct {
		CID       string   `json:"cid"`
		Providers int      `json:"providers"`
		PeerIDs   []string `json:"peer_ids,omitempty"`
		Generated int64    `json:"generated"`
	}{
		CID:       cid,
		Providers: len(provs),
		PeerIDs:   provs,
		Generated: time.Now().Unix(),
	}
	b, _ := json.MarshalIndent(&out, "", "  ")
	if err := os.WriteFile(target, b, 0644); err != nil {
		log.Printf("write propagation.json: %v", err)
		return
	}
	log.Printf("propagation.json: %d distinct peers providing %s", len(provs), cid[:12]+"...")
}

func (d *Daemon) sendHeartbeat(latest *Latest) {
	d.stateMu.RLock()
	sk := d.state.NostrSK
	d.stateMu.RUnlock()
	if sk == "" {
		return
	}
	if err := broadcastHeartbeat(d.pool, sk, d.peerID, latest.CID, d.cfg.Country, d.cfg.MirrorURL, d.cfg.MirrorRelayURL, latest.Version); err != nil {
		log.Printf("heartbeat: %v", err)
	} else {
		log.Println("Heartbeat sent to Nostr")
	}
}

func jitterDuration(base, spread time.Duration) time.Duration {
	return base + time.Duration(rand.Int64N(int64(spread*2+1))) - spread
}

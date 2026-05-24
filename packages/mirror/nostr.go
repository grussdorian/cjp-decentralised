package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

// heartbeatRelays is intentionally mixed: CF-fronted and self-hosted/community
// relays side by side so the heartbeat broadcast doesn't depend on any single
// CDN's rate-limiter. CF *for reading* (browser → relays) is reliable; CF
// *for writing* (this daemon → relays every minute) trips abuse detection
// after a few hours from the same IP. Diversify the egress path.
var heartbeatRelays = []string{
	"wss://relay.damus.io",          // CF-fronted (Damus team)
	"wss://relay.primal.net",        // self-hosted (Primal team)
	"wss://nostr.mom",               // community, non-CF
	"wss://nostr.bitcoiner.social",  // community, non-CF
	"wss://nostr-pub.wellorder.net", // community, non-CF
}
// Dropped: nos.lol (now requires NIP-13 28-bit PoW), relay.snort.social
// (unreachable from this network — i/o timeouts on every connect).

// ensureNostrKey generates a Nostr key for this mirror if one doesn't exist yet.
func ensureNostrKey(state *State) {
	if state.NostrSK == "" {
		state.NostrSK = nostr.GeneratePrivateKey()
	}
}

// broadcastHeartbeat publishes a signed Nostr event announcing this mirror is alive.
// url is optional — set it to the public clearweb URL of this mirror (e.g. https://mymirror.example.com)
// so that the mirror registry on the site can list it as a clickable link.
//
// Uses a RelayPool so the connection to each relay survives across heartbeats.
// Without this, every 30-60s tick re-did a full TLS+WSS handshake and CF's
// abuse detection rate-limited the daemon after a few hours.
func broadcastHeartbeat(pool *RelayPool, nostrSK, peerID, cid, country, url string, version int64) error {
	fields := map[string]interface{}{
		"peer_id": peerID,
		"cid":     cid,
		"version": version,
		"country": country,
		"ts":      time.Now().Unix(),
	}
	if url != "" {
		fields["url"] = url
	}
	payload, err := json.Marshal(fields)
	if err != nil {
		return err
	}

	ev := nostr.Event{
		Kind:      1,
		Tags:      nostr.Tags{{"t", "cjp-mirrors"}},
		Content:   string(payload),
		CreatedAt: nostr.Timestamp(time.Now().Unix()),
	}
	if err := ev.Sign(nostrSK); err != nil {
		return fmt.Errorf("sign heartbeat: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if ok := pool.Publish(ctx, ev); ok == 0 {
		return fmt.Errorf("no heartbeat relays responded")
	}
	return nil
}

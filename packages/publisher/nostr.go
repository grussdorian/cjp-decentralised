package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

// publishRelays mirrors the daemon's heartbeatRelays — see
// packages/mirror/nostr.go. Mixed CF/non-CF set so update broadcasts don't
// depend on any single CDN's availability.
var publishRelays = []string{
	"wss://relay.damus.io",          // CF-fronted (Damus team)
	"wss://relay.primal.net",        // self-hosted (Primal team)
	"wss://nostr.mom",               // community, non-CF
	"wss://nostr.bitcoiner.social",  // community, non-CF
	"wss://nostr-pub.wellorder.net", // community, non-CF
}

// broadcastUpdate publishes a signed Nostr event carrying the FULL signed
// latest.json (CID, version, timestamp, signatures). Daemons + browsers
// that can't reach GitHub fetch this event, verify the Ed25519 signatures
// against the hardcoded trusted-signer keys, and treat it as authoritative.
//
// Nostr is just transport here — the security boundary is the M-of-N
// Ed25519 signature scheme. An attacker can publish a Nostr event with
// any content; they cannot forge a valid signature without the trusted
// signing keys.
//
// nostrSK is a hex-encoded secp256k1 private key.
func broadcastUpdate(nostrSK string, l *Latest) error {
	// Serialize the full Latest payload — daemons need timestamp + signatures
	// to verify the update, not just the CID.
	payload, err := json.Marshal(l)
	if err != nil {
		return err
	}

	ev := nostr.Event{
		Kind:      1,
		Tags:      nostr.Tags{{"t", "cjp-update"}},
		Content:   string(payload),
		CreatedAt: nostr.Timestamp(time.Now().Unix()),
	}
	if err := ev.Sign(nostrSK); err != nil {
		return fmt.Errorf("sign nostr event: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ok := 0
	for _, url := range publishRelays {
		relay, err := nostr.RelayConnect(ctx, url)
		if err != nil {
			fmt.Printf("  ✗ %s: %v\n", url, err)
			continue
		}
		if err := relay.Publish(ctx, ev); err != nil {
			fmt.Printf("  ✗ %s: %v\n", url, err)
		} else {
			fmt.Printf("  ✓ %s\n", url)
			ok++
		}
		relay.Close()
	}

	if ok == 0 {
		return fmt.Errorf("no relays accepted the event")
	}
	fmt.Printf("Published to %d/%d relays\n", ok, len(publishRelays))
	return nil
}

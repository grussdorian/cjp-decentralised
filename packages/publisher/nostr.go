package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

var publishRelays = []string{
	"wss://relay.damus.io",
	"wss://nos.lol",
	"wss://nostr.wine",
	"wss://relay.nostr.band",
	"wss://relay.snort.social",
	"wss://offchain.pub",
}

// broadcastUpdate publishes a signed Nostr event announcing a new CID.
// nostrSK is a hex-encoded secp256k1 private key.
func broadcastUpdate(nostrSK string, l *Latest) error {
	payload, err := json.Marshal(map[string]interface{}{
		"cid":     l.CID,
		"version": l.Version,
		"note":    l.Note,
	})
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

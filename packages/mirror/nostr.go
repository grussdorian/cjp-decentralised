package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

var heartbeatRelays = []string{
	"wss://relay.damus.io",
	"wss://nos.lol",
	"wss://nostr.wine",
	"wss://relay.nostr.band",
	"wss://offchain.pub",
}

// ensureNostrKey generates a Nostr key for this mirror if one doesn't exist yet.
func ensureNostrKey(state *State) {
	if state.NostrSK == "" {
		state.NostrSK = nostr.GeneratePrivateKey()
	}
}

// broadcastHeartbeat publishes a signed Nostr event announcing this mirror is alive.
// url is optional — set it to the public clearweb URL of this mirror (e.g. https://mymirror.example.com)
// so that the mirror registry on the site can list it as a clickable link.
func broadcastHeartbeat(nostrSK, peerID, cid, country, url string, version int64) error {
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

	ok := 0
	for _, url := range heartbeatRelays {
		relay, err := nostr.RelayConnect(ctx, url)
		if err != nil {
			continue
		}
		if relay.Publish(ctx, ev) == nil {
			ok++
		}
		relay.Close()
	}
	if ok == 0 {
		return fmt.Errorf("no heartbeat relays responded")
	}
	return nil
}

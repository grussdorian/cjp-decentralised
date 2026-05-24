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

// fetchUpdateFromNostr queries heartbeat relays for kind:1 events tagged
// #cjp-update and returns the highest-versioned event whose embedded
// Latest payload parses cleanly. Returns nil if no event was found within
// the timeout — callers must still verify signatures on the returned
// Latest before trusting it.
//
// This is the GitHub-down fallback: the same federated relay set that
// carries mirror heartbeats also carries signed update announcements.
// As long as ANY relay in the pool has the event, mirrors can pick up
// the new CID without GitHub being reachable.
func fetchUpdateFromNostr(relayURLs []string, since time.Time, timeout time.Duration) *Latest {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	type res struct {
		l *Latest
	}
	resCh := make(chan res, len(relayURLs))
	for _, url := range relayURLs {
		go func(u string) {
			r, err := nostr.RelayConnect(ctx, u)
			if err != nil {
				resCh <- res{}
				return
			}
			defer r.Close()
			filter := nostr.Filter{
				Kinds: []int{1},
				Tags:  nostr.TagMap{"t": []string{"cjp-update"}},
				Since: nostrTimePtr(since),
				Limit: 50,
			}
			sub, err := r.Subscribe(ctx, nostr.Filters{filter})
			if err != nil {
				resCh <- res{}
				return
			}
			var best *Latest
			for {
				select {
				case ev, ok := <-sub.Events:
					if !ok {
						resCh <- res{best}
						return
					}
					var l Latest
					if json.Unmarshal([]byte(ev.Content), &l) != nil {
						continue
					}
					if l.CID == "" || len(l.allSigs()) == 0 {
						continue
					}
					if best == nil || l.Version > best.Version {
						best = &l
					}
				case <-sub.EndOfStoredEvents:
					resCh <- res{best}
					return
				case <-ctx.Done():
					resCh <- res{best}
					return
				}
			}
		}(url)
	}

	var best *Latest
	for range relayURLs {
		select {
		case r := <-resCh:
			if r.l != nil && (best == nil || r.l.Version > best.Version) {
				best = r.l
			}
		case <-ctx.Done():
			return best
		}
	}
	return best
}

func nostrTimePtr(t time.Time) *nostr.Timestamp {
	ts := nostr.Timestamp(t.Unix())
	return &ts
}

// broadcastHeartbeat publishes a signed Nostr event announcing this mirror is alive.
//   url      — optional public clearweb URL of this mirror (e.g. https://mirror.example)
//   relayURL — optional public wss:// URL of this mirror's bundled relay. When set,
//              browsers discover it from the heartbeat and add it to their query
//              pool, so federation grows organically with volunteer count.
//
// Uses a RelayPool so the connection to each relay survives across heartbeats.
// Without this, every 30-60s tick re-did a full TLS+WSS handshake and CF's
// abuse detection rate-limited the daemon after a few hours.
func broadcastHeartbeat(pool *RelayPool, nostrSK, peerID, cid, country, url, relayURL string, version int64) error {
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
	if relayURL != "" {
		fields["relay_url"] = relayURL
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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

// PeerObservation is this mirror's record of another mirror it has seen
// broadcasting heartbeats. Used to build the peer attestation snapshot.
type PeerObservation struct {
	Pubkey     string `json:"pubkey"`
	URL        string `json:"url,omitempty"`
	Country    string `json:"country,omitempty"`
	FirstSeen  int64  `json:"first_seen"`
	LastSeen   int64  `json:"last_seen"`
	Heartbeats int    `json:"heartbeats"`
}

// Attestation is a snapshot of every mirror this node has ever observed,
// published periodically as a NIP-33 parameterized replaceable Nostr event
// so the network can independently verify which peers have been part of
// the federation over time. Defends against fake-mirror campaigns: an
// attacker spinning up an impostor mirror won't appear in any honest
// mirror's attestation history.
type Attestation struct {
	WindowStart int64             `json:"window_start"`
	WindowEnd   int64             `json:"window_end"`
	Peers       []PeerObservation `json:"peers"`
}

// queryHeartbeatsFromRelay opens a short-lived subscription to the given
// relay and pulls every cjp-mirrors event since the given timestamp.
// Returns the raw events; deduplication and aggregation happen above.
func queryHeartbeatsFromRelay(relayURL string, since time.Time, timeout time.Duration) ([]*nostr.Event, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	r, err := nostr.RelayConnect(ctx, relayURL)
	if err != nil {
		return nil, fmt.Errorf("connect %s: %w", relayURL, err)
	}
	defer r.Close()

	filter := nostr.Filter{
		Kinds: []int{1},
		Tags:  nostr.TagMap{"t": []string{"cjp-mirrors"}},
		Since: nostrTimePtr(since),
		Limit: 5000,
	}
	sub, err := r.Subscribe(ctx, nostr.Filters{filter})
	if err != nil {
		return nil, fmt.Errorf("subscribe: %w", err)
	}

	var events []*nostr.Event
	for {
		select {
		case ev, ok := <-sub.Events:
			if !ok {
				return events, nil
			}
			events = append(events, ev)
		case <-sub.EndOfStoredEvents:
			return events, nil
		case <-ctx.Done():
			return events, nil
		}
	}
}

// safePeerURL is an allowlist for url strings written into attestations —
// reject anything that doesn't parse as http(s) so a malicious heartbeat
// can't inject e.g. javascript: URLs that end up rendered in browsers.
func safePeerURL(s string) string {
	if s == "" {
		return ""
	}
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	return u.String()
}

// buildAttestation queries every supplied relay for cjp-mirrors events in
// the lookback window, aggregates per-pubkey first/last-seen and counts,
// and returns a snapshot. The mirror's own pubkey is excluded so each
// mirror's attestation is purely about *other* peers it has observed.
func buildAttestation(relayURLs []string, ownPubkey string, lookback time.Duration) *Attestation {
	since := time.Now().Add(-lookback)
	peers := make(map[string]*PeerObservation)

	for _, u := range relayURLs {
		events, err := queryHeartbeatsFromRelay(u, since, 10*time.Second)
		if err != nil {
			log.Printf("attestation: query %s: %v", u, err)
			continue
		}
		for _, ev := range events {
			if ev.PubKey == ownPubkey {
				continue
			}
			// Parse heartbeat content for url/country.
			var hb struct {
				URL     string `json:"url"`
				Country string `json:"country"`
			}
			_ = json.Unmarshal([]byte(ev.Content), &hb)

			ts := int64(ev.CreatedAt)
			p, ok := peers[ev.PubKey]
			if !ok {
				p = &PeerObservation{
					Pubkey:    ev.PubKey,
					FirstSeen: ts,
					LastSeen:  ts,
				}
				peers[ev.PubKey] = p
			}
			if ts < p.FirstSeen {
				p.FirstSeen = ts
			}
			if ts > p.LastSeen {
				p.LastSeen = ts
			}
			p.Heartbeats++
			if u := safePeerURL(hb.URL); u != "" {
				p.URL = u // most recent URL wins (operators may change hostnames)
			}
			if hb.Country != "" {
				p.Country = hb.Country
			}
		}
	}

	list := make([]PeerObservation, 0, len(peers))
	for _, p := range peers {
		list = append(list, *p)
	}
	// Stable order: oldest first_seen first — long-time peers are most credible.
	sort.Slice(list, func(i, j int) bool {
		if list[i].FirstSeen != list[j].FirstSeen {
			return list[i].FirstSeen < list[j].FirstSeen
		}
		return list[i].Pubkey < list[j].Pubkey
	})

	return &Attestation{
		WindowStart: since.Unix(),
		WindowEnd:   time.Now().Unix(),
		Peers:       list,
	}
}

// publishAttestation publishes the attestation as a NIP-33 parameterized
// replaceable Nostr event. Kind 30078 is "Application-specific Data"; the
// (kind, pubkey, "d") tuple identifies the event — newer events replace
// older ones so we don't accumulate stale attestations on relays.
func publishAttestation(pool *RelayPool, nostrSK string, a *Attestation) error {
	payload, err := json.Marshal(a)
	if err != nil {
		return err
	}
	ev := nostr.Event{
		Kind: 30078,
		Tags: nostr.Tags{
			{"t", "cjp-attestation"},
			{"d", "v1"},
		},
		Content:   string(payload),
		CreatedAt: nostr.Timestamp(time.Now().Unix()),
	}
	if err := ev.Sign(nostrSK); err != nil {
		return fmt.Errorf("sign attestation: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if ok := pool.Publish(ctx, ev); ok == 0 {
		return fmt.Errorf("no relays accepted attestation")
	}
	return nil
}

// writePeersFile dumps the attestation to disk so visitors can fetch it
// over plain HTTP (/peers.json) without needing Nostr literacy. Best-effort.
func writePeersFile(path string, a *Attestation) error {
	b, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Clean(path))
}

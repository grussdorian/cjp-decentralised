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

	// SeenAttestations is the list of Nostr event IDs of OTHER mirrors'
	// attestations that this mirror has ingested. Including someone's event
	// ID is a passive endorsement: "I have observed this attestation
	// existing at this time." Nostr event IDs are content-addressed
	// (sha256 of pubkey+kind+created_at+tags+content) so the referenced
	// event must have existed before being referenced — back-dating an
	// attestation chain requires also forging every event that references it.
	SeenAttestations []SeenAttestation `json:"seen_attestations,omitempty"`
}

// SeenAttestation is one entry in the chain — a referenced event from
// another mirror. EventID + AttesterPubkey + CreatedAt is enough for any
// verifier to fetch the original event and confirm it existed at that time.
type SeenAttestation struct {
	EventID        string `json:"id"`
	AttesterPubkey string `json:"pubkey"`
	CreatedAt      int64  `json:"created_at"`
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

// queryAttestationsFromRelay fetches every kind:30078 #cjp-attestation
// event from a relay so the daemon can include them as "seen" references
// in its own attestation. Same shape as the heartbeat query but a
// different filter.
func queryAttestationsFromRelay(relayURL string, since time.Time, timeout time.Duration) ([]*nostr.Event, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	r, err := nostr.RelayConnect(ctx, relayURL)
	if err != nil {
		return nil, fmt.Errorf("connect %s: %w", relayURL, err)
	}
	defer r.Close()
	filter := nostr.Filter{
		Kinds: []int{30078},
		Tags:  nostr.TagMap{"t": []string{"cjp-attestation"}},
		Since: nostrTimePtr(since),
		Limit: 500,
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

// buildAttestation queries every supplied relay for cjp-mirrors events in
// the lookback window, aggregates per-pubkey first/last-seen and counts,
// and returns a snapshot. Also collects every OTHER mirror's attestation
// event ID into the SeenAttestations chain — passive endorsement that
// makes back-dating cryptographically expensive. The mirror's own pubkey
// is excluded so each attestation is purely about *other* observed peers.
func buildAttestation(relayURLs []string, ownPubkey string, lookback time.Duration) *Attestation {
	since := time.Now().Add(-lookback)
	peers := make(map[string]*PeerObservation)
	seenAtts := make(map[string]*SeenAttestation) // dedupe by event ID

	for _, u := range relayURLs {
		// 1. Heartbeats → peer observations.
		events, err := queryHeartbeatsFromRelay(u, since, 10*time.Second)
		if err != nil {
			log.Printf("attestation: heartbeats %s: %v", u, err)
		}
		for _, ev := range events {
			if ev.PubKey == ownPubkey {
				continue
			}
			var hb struct {
				URL     string `json:"url"`
				Country string `json:"country"`
			}
			_ = json.Unmarshal([]byte(ev.Content), &hb)
			ts := int64(ev.CreatedAt)
			p, ok := peers[ev.PubKey]
			if !ok {
				p = &PeerObservation{Pubkey: ev.PubKey, FirstSeen: ts, LastSeen: ts}
				peers[ev.PubKey] = p
			}
			if ts < p.FirstSeen {
				p.FirstSeen = ts
			}
			if ts > p.LastSeen {
				p.LastSeen = ts
			}
			p.Heartbeats++
			if uu := safePeerURL(hb.URL); uu != "" {
				p.URL = uu
			}
			if hb.Country != "" {
				p.Country = hb.Country
			}
		}

		// 2. Other mirrors' attestations → SeenAttestations chain.
		atts, err := queryAttestationsFromRelay(u, since, 10*time.Second)
		if err != nil {
			log.Printf("attestation: query atts %s: %v", u, err)
			continue
		}
		for _, ev := range atts {
			if ev.PubKey == ownPubkey || ev.ID == "" {
				continue
			}
			if _, dup := seenAtts[ev.ID]; dup {
				continue
			}
			seenAtts[ev.ID] = &SeenAttestation{
				EventID:        ev.ID,
				AttesterPubkey: ev.PubKey,
				CreatedAt:      int64(ev.CreatedAt),
			}
		}
	}

	list := make([]PeerObservation, 0, len(peers))
	for _, p := range peers {
		list = append(list, *p)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].FirstSeen != list[j].FirstSeen {
			return list[i].FirstSeen < list[j].FirstSeen
		}
		return list[i].Pubkey < list[j].Pubkey
	})

	seenList := make([]SeenAttestation, 0, len(seenAtts))
	for _, s := range seenAtts {
		seenList = append(seenList, *s)
	}
	sort.Slice(seenList, func(i, j int) bool { return seenList[i].CreatedAt < seenList[j].CreatedAt })

	return &Attestation{
		WindowStart:      since.Unix(),
		WindowEnd:        time.Now().Unix(),
		Peers:            list,
		SeenAttestations: seenList,
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

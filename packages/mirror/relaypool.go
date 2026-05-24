package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

// relayConn holds a single persistent connection to one relay. Reconnects
// lazily on failure so the heartbeat path doesn't pay a TLS+WSS handshake
// every tick — which was triggering Cloudflare's abuse detection on
// CF-fronted relays after a few hours.
type relayConn struct {
	url string

	mu    sync.Mutex
	relay *nostr.Relay // nil when disconnected
}

func (rc *relayConn) getOrConnect(parent context.Context) (*nostr.Relay, error) {
	rc.mu.Lock()
	if rc.relay != nil {
		r := rc.relay
		rc.mu.Unlock()
		return r, nil
	}
	rc.mu.Unlock()

	dialCtx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	newR, err := nostr.RelayConnect(dialCtx, rc.url)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	rc.mu.Lock()
	defer rc.mu.Unlock()
	if rc.relay != nil {
		// Lost the race — another goroutine connected first; drop ours.
		newR.Close()
		return rc.relay, nil
	}
	rc.relay = newR
	return newR, nil
}

func (rc *relayConn) markDead(r *nostr.Relay) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if rc.relay == r {
		rc.relay.Close()
		rc.relay = nil
	}
}

func (rc *relayConn) publish(parent context.Context, ev nostr.Event) error {
	r, err := rc.getOrConnect(parent)
	if err != nil {
		return err
	}
	pubCtx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	if err := r.Publish(pubCtx, ev); err != nil {
		// Drop the connection so next publish reconnects fresh.
		rc.markDead(r)
		return err
	}
	return nil
}

func (rc *relayConn) close() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if rc.relay != nil {
		rc.relay.Close()
		rc.relay = nil
	}
}

// RelayPool fans publishes to N relays in parallel over persistent
// connections. One handshake per relay lifetime — not one per heartbeat.
type RelayPool struct {
	conns []*relayConn
}

func NewRelayPool(urls []string) *RelayPool {
	conns := make([]*relayConn, len(urls))
	for i, u := range urls {
		conns[i] = &relayConn{url: u}
	}
	return &RelayPool{conns: conns}
}

// Publish broadcasts ev to every relay in parallel. Returns the number of
// relays that acknowledged the event.
func (p *RelayPool) Publish(ctx context.Context, ev nostr.Event) int {
	var wg sync.WaitGroup
	var n int32
	for _, rc := range p.conns {
		wg.Add(1)
		go func(rc *relayConn) {
			defer wg.Done()
			if err := rc.publish(ctx, ev); err != nil {
				log.Printf("publish %s: %v", rc.url, err)
				return
			}
			atomic.AddInt32(&n, 1)
		}(rc)
	}
	wg.Wait()
	return int(n)
}

func (p *RelayPool) Close() {
	for _, rc := range p.conns {
		rc.close()
	}
}

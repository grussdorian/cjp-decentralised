package main

import (
	"encoding/json"
	"os"
	"time"
)

type Config struct {
	IPFSApi      string        // IPFS HTTP API, e.g. http://localhost:5001
	PollInterval time.Duration // How often to check for updates
	IPNSName     string        // IPNS name to resolve for latest.json (optional)
	FallbackURLs []string      // Clearweb URLs to fetch latest.json from
	SignersFile  string        // Path to trusted-signers.json
	StateFile    string        // Path to persisted daemon state
	Country      string        // Reported country in heartbeat (optional)
}

func defaultConfig() Config {
	home, _ := os.UserHomeDir()
	return Config{
		IPFSApi:      envOr("IPFS_API", "http://localhost:5001"),
		PollInterval: parseDuration(envOr("POLL_INTERVAL", "900"), 15*60),
		IPNSName:     envOr("IPNS_NAME", ""),
		FallbackURLs: []string{
			"https://raw.githubusercontent.com/cjp-decentralized/cjp-decentralized/main/latest.json",
		},
		SignersFile: envOr("SIGNERS_FILE", "trusted-signers.json"),
		StateFile:   envOr("STATE_FILE", home+"/.cjp/mirror-state.json"),
		Country:     envOr("COUNTRY", ""),
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseDuration(s string, defaultSec int) time.Duration {
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}
	return time.Duration(defaultSec) * time.Second
}

// State persists across restarts.
type State struct {
	NostrSK      string `json:"nostr_sk"`
	PinnedCID    string `json:"pinned_cid"`
	PinnedVer    int64  `json:"pinned_version"`
}

func loadState(path string) (*State, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return &State{}, nil
	}
	var s State
	if err := json.Unmarshal(raw, &s); err != nil {
		return &State{}, nil
	}
	return &s, nil
}

func saveState(path string, s *State) error {
	b, _ := json.MarshalIndent(s, "", "  ")
	if err := os.MkdirAll(dirOf(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0600)
}

func dirOf(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[:i]
		}
	}
	return "."
}

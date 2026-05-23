package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	var (
		ipfsAPI    = flag.String("ipfs-api", "", "IPFS HTTP API URL (overrides IPFS_API env)")
		pollSec    = flag.Int("poll", 0, "poll interval in seconds (overrides POLL_INTERVAL env)")
		signersF   = flag.String("signers", "", "path to trusted-signers.json (overrides SIGNERS_FILE env)")
		stateF     = flag.String("state", "", "path to state file (overrides STATE_FILE env)")
		country    = flag.String("country", "", "your country, reported in heartbeat (overrides COUNTRY env)")
		ipnsName   = flag.String("ipns", "", "IPNS name to resolve for latest.json (overrides IPNS_NAME env)")
		once       = flag.Bool("once", false, "run one poll cycle then exit (useful for testing)")
	)
	flag.Parse()

	cfg := defaultConfig()
	if *ipfsAPI != "" {
		cfg.IPFSApi = *ipfsAPI
	}
	if *pollSec > 0 {
		cfg.PollInterval = time.Duration(*pollSec) * time.Second
	}
	if *signersF != "" {
		cfg.SignersFile = *signersF
	}
	if *stateF != "" {
		cfg.StateFile = *stateF
	}
	if *country != "" {
		cfg.Country = *country
	}
	if *ipnsName != "" {
		cfg.IPNSName = *ipnsName
	}

	daemon, err := newDaemon(cfg)
	if err != nil {
		log.Fatalf("init daemon: %v", err)
	}

	if *once {
		daemon.poll()
		if err := saveState(cfg.StateFile, daemon.state); err != nil {
			log.Printf("save state: %v", err)
		}
		os.Exit(0)
	}

	stop := make(chan struct{})
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		close(stop)
	}()

	daemon.Run(stop)
}

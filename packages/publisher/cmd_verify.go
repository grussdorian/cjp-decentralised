package main

import (
	"flag"
	"fmt"
	"net/http"
	"sync"
	"time"
)

var gateways = []string{
	"https://ipfs.io/ipfs/%s",
	"https://cloudflare-ipfs.com/ipfs/%s",
	"https://dweb.link/ipfs/%s",
	"https://gateway.pinata.cloud/ipfs/%s",
	"https://hardbin.com/ipfs/%s",
}

func cmdVerify(args []string) {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	cid := fs.String("cid", "", "CID to verify (default: reads from latest.json)")
	latestPath := fs.String("latest", "latest.json", "path to latest.json")
	fs.Parse(args)

	if *cid == "" {
		l, err := readLatest(*latestPath)
		if err != nil {
			die("read latest.json: %v", err)
		}
		*cid = l.CID
	}
	if *cid == "" {
		die("no CID found — pass --cid or ensure latest.json has a CID")
	}

	fmt.Printf("Checking CID %s across %d gateways...\n\n", *cid, len(gateways))

	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0

	for _, gw := range gateways {
		wg.Add(1)
		go func(gw string) {
			defer wg.Done()
			url := fmt.Sprintf(gw, *cid)
			status, err := headRequest(url)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				fmt.Printf("  ✗ %s — %v\n", url, err)
			} else if status == 200 || status == 301 || status == 302 {
				fmt.Printf("  ✓ %s — %d\n", url, status)
				ok++
			} else {
				fmt.Printf("  ? %s — HTTP %d\n", url, status)
			}
		}(gw)
	}
	wg.Wait()

	fmt.Printf("\nReachable on %d/%d gateways.\n", ok, len(gateways))
}

func headRequest(url string) (int, error) {
	c := &http.Client{Timeout: 10 * time.Second}
	resp, err := c.Head(url)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return resp.StatusCode, nil
}

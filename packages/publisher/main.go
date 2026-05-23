package main

import (
	"fmt"
	"os"
)

const usage = `publisher — CJP decentralized site publisher

Usage:
  publisher keygen   [--out <dir>]
  publisher sign     --cid <cid> [--key <path>] [--version <n>] [--note <text>]
  publisher publish  [--latest <path>] [--nostr-key <path>] [--ipfs-api <url>]
  publisher verify   [--cid <cid>] [--latest <path>]

Commands:
  keygen   Generate Ed25519 signing key + Nostr key pair. Output pubkey for trusted-signers.json.
  sign     Sign a new latest.json with your Ed25519 key. Run after CI reports the new CID.
  publish  Broadcast the signed latest.json to IPNS and Nostr relays.
  verify   Check that a CID is reachable on public IPFS gateways.

Workflow:
  1. Push to main → CI builds site and prints new IPFS CID
  2. publisher sign --cid <new-cid>
  3. git add latest.json && git commit -m "chore: publish vN" && git push
  4. publisher publish
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(0)
	}
	switch os.Args[1] {
	case "keygen":
		cmdKeygen(os.Args[2:])
	case "sign":
		cmdSign(os.Args[2:])
	case "publish":
		cmdPublish(os.Args[2:])
	case "verify":
		cmdVerify(os.Args[2:])
	case "help", "--help", "-h":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", os.Args[1], usage)
		os.Exit(1)
	}
}

func die(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", a...)
	os.Exit(1)
}

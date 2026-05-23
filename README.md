# CJP Decentralized

Censorship-resistant web presence for the Cockroach Janata Party. No single point of failure across hosting, naming, form backends, or identity.

## Repository layout

```
packages/site/       Static HTML/CSS/JS frontend (5 languages)
packages/mirror/     Go daemon — volunteers run this to pin and serve the site
packages/publisher/  Go CLI — sign and publish CID updates (run locally, key never leaves machine)
content/manifesto/   Manifesto source (English Markdown)
content/translations/ i18n JSON for en, hi, ta, te, bn
scripts/build.js     Renders templates × languages → dist/
trusted-signers.json Ed25519 pubkeys of authorized publishers
latest.json          Signed pointer to current IPFS CID
docker-compose.yml   One-command volunteer mirror stack
```

## Run a mirror (volunteers)

```bash
docker compose up -d
```

That's it. Your node will automatically pin the latest version and serve it on port 8081.

## Publish an update (developers)

1. Push to `main` — CI builds the site and uploads to IPFS, printing the new CID.
2. Locally:
   ```bash
   publisher sign --key ~/.cjp/signing.key --cid <new-cid> --version <n> --note "your note"
   publisher publish --latest latest.json
   git add latest.json && git commit -m "chore: publish v<n>"
   git push
   ```

## Access

| Method | Address |
|--------|---------|
| IPFS gateway | `https://ipfs.io/ipfs/<CID>` |
| IPNS | `https://ipfs.io/ipns/<key>` |
| ENS | `cockroachjanataparty.eth` (via MetaMask or Brave) |
| Tor | see `.onion` address printed by `docker compose up` |
| Clearweb mirrors | listed at `/mirror` on the site |

## Add a trusted signer

```bash
publisher keygen --out ~/.cjp/
# Outputs pubkey — add it to trusted-signers.json, commit, and push
```

## Tech stack

- **Hosting**: IPFS (content-addressed, anyone can pin)
- **Mutability**: IPNS + ENS content hash (Gnosis Safe multisig)
- **Form backend**: Nostr protocol (sign-up encrypted age DMs; petition public events)
- **Spam protection**: Browser-only SHA-256 proof-of-work (no server, no CDN, works on Tor)
- **Mirror sync**: Ed25519-signed `latest.json` polled every 15 min
- **Mirror registry**: Nostr heartbeat events tagged `#cjp-mirrors`

## License

[MIT](LICENSE) — copy, modify, redistribute freely. See [CONTRIBUTING.md](CONTRIBUTING.md) for fork guidance.

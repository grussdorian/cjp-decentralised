# CJP Decentralized

[![Build](https://github.com/grussdorian/cjp-decentralised/actions/workflows/publish.yml/badge.svg)](https://github.com/grussdorian/cjp-decentralised/actions/workflows/publish.yml)

Censorship-resistant web presence for the Cockroach Janata Party. No single point of failure across hosting, naming, form backends, or identity.

## Live mirrors

| Domain | Operator | Status |
|--------|----------|--------|
| [cjp.fheya.de](https://cjp.fheya.de) | official | clearweb |
| [todo.fheya.com](https://todo.fheya.com) | official | clearweb |

More mirrors are listed live at [cjp.fheya.de/mirror.html](https://cjp.fheya.de/mirror.html) — updated every 2 minutes from the Nostr mirror registry.

**Coming soon:** Tor `.onion` hidden service and ENS/Ethereum on-chain trust anchor (see [Trust model](#trust-model) below).

## Verify any mirror

Every mirror shows a badge at the bottom of each page. To verify independently:

1. Open [`latest.json`](latest.json) in this repo. Note the `version` number and `cid`.
2. Open any mirror — the badge must show the **same version number** and **same IPFS CID** (`bafybeicpdosradlm3y2n7kc7camuj2pbuyfnarlklanrwc47l6ky66aidq`).
3. The key fingerprint in the badge (`c1688ff0…b5c3`) must match [trusted-signers.json](trusted-signers.json).

If a mirror shows a different CID, a different version, or a different fingerprint — it is not serving authentic content.

You can also fetch the content directly from IPFS:
```
https://ipfs.io/ipfs/bafybeicpdosradlm3y2n7kc7camuj2pbuyfnarlklanrwc47l6ky66aidq
```

## Run a volunteer mirror

```bash
git clone https://github.com/grussdorian/cjp-decentralised
cd cjp-decentralised
docker compose up -d
```

Your node pins the latest signed content to IPFS and announces itself on the global Nostr mirror registry every 15 minutes — no domain or registration required.

**To appear as a clickable link on the site**, set `MIRROR_URL` to your public hostname:

```yaml
# docker-compose.override.yml
services:
  mirror:
    environment:
      MIRROR_URL: "https://cjp.yourdomain.example"
      COUNTRY: "IN"
```

Your domain will appear automatically in the [live mirror list](https://cjp.fheya.de/mirror.html) within two minutes — no PR or manual registration needed. Visitors can click your link and independently verify the badge on your mirror.

See [mirror.html](https://cjp.fheya.de/mirror.html) for full setup instructions including serving with your own nginx or Apache.

## Repository layout

```
packages/site/        Static HTML/CSS/JS frontend (5 languages)
packages/mirror/      Go daemon — volunteers run this to pin and serve the site
packages/publisher/   Go CLI — sign and publish CID updates (run locally, key never leaves machine)
content/manifesto/    Manifesto source (English Markdown)
content/translations/ i18n JSON for en, hi, ta, te, bn
scripts/build.js      Renders templates × languages → dist/
trusted-signers.json  Ed25519 pubkeys of authorized publishers
latest.json           Signed pointer to current IPFS CID
docker-compose.yml    One-command volunteer mirror stack
```

## Publish an update (developers)

1. Push to `main` — CI builds the site and uploads to IPFS, printing the new CID.
2. Locally:
   ```bash
   publisher sign --key ~/.cjp/signing.key --cid <new-cid> --version <n> --note "your note"
   publisher publish --latest latest.json
   git add latest.json README.md && git commit -m "chore: publish v<n>"
   git push
   ```

> When publishing, update the IPFS CID in the [Verify any mirror](#verify-any-mirror) section of this README so readers always have the current address.

## Trust model

Content authenticity rests on a chain of verifiable anchors:

```
Ed25519 signatures (M-of-N keys)
  └─ sign IPFS CID  →  content-addressed directory
        └─ contains integrity.json  →  SHA-256 of every page
              └─ verify.js compares against the page served by each mirror
```

**Current state:** 1-of-1 signing key. As more trusted party members join, the threshold will increase — a single compromised key will not be sufficient to publish a fraudulent update.

**In progress:**
- **Tor `.onion`** — hidden service so the site remains reachable if all clearweb domains are seized.
- **ENS / Ethereum** — on-chain content hash (`cockroachjanataparty.eth` via Gnosis Safe multisig). Once live, users can resolve the canonical CID without trusting GitHub, this repo, or any DNS provider. This is the highest-trust anchor in the system.

## Become a signer

Signing authority is intentionally restricted. Contact the repository owner directly — do not open a public issue. Signers are vetted individually; the M-of-N threshold and vetting process will become stricter as the mirror network grows.

## Access

| Method | Address |
|--------|---------|
| Clearweb mirrors | listed at [/mirror](https://cjp.fheya.de/mirror.html) on the site |
| IPFS gateway | [`ipfs.io/ipfs/bafybeicpdosradlm3y2n7kc7camuj2pbuyfnarlklanrwc47l6ky66aidq`](https://ipfs.io/ipfs/bafybeicpdosradlm3y2n7kc7camuj2pbuyfnarlklanrwc47l6ky66aidq) |
| IPNS | pending |
| ENS | `cockroachjanataparty.eth` — pending on-chain registration |
| Tor | pending hidden service setup |

## Form data persistence

Sign-up and petition submissions are stored on Nostr relays — a separate network from IPFS. This means:

- **Version changes do not affect submissions.** Publishing a new site version updates the IPFS CID and `latest.json`, but relay data is never touched. A submission made on v1 is still retrievable on v50.
- **No central server holds the data.** Submissions are broadcast to 12 independent relays across jurisdictions simultaneously. No single relay going down loses any data.
- **Sign-ups are end-to-end encrypted.** Each submission is age-encrypted to the party's public key before it leaves the browser. Only the key holder can read it — relays store opaque ciphertext.
- **Petitions are public and verifiable.** Demand form entries are signed Nostr events anyone can count on any relay — no need to trust the party's tally.

**Long-term retention note:** Public relays may expire events after 30–90 days to manage storage. Running a private relay ensures permanent retention. See issue [#7](https://github.com/grussdorian/cjp-decentralised/issues) for context.

## Tech stack

- **Hosting**: IPFS (content-addressed, anyone can pin)
- **Mutability**: IPNS + ENS content hash (Gnosis Safe multisig)
- **Form backend**: Nostr protocol (sign-up age-encrypted to party key; petition public events)
- **Spam protection**: Browser-only SHA-256 proof-of-work (no server, no CDN, works on Tor)
- **Mirror sync**: Ed25519-signed `latest.json` polled every 15 min
- **Mirror registry**: Nostr heartbeat events tagged `#cjp-mirrors`

## Contributing

If you are a developer, you may contribute to this repo — see the [open issues](https://github.com/grussdorian/cjp-decentralised/issues) for pending work. Good first areas: Tor hidden service (#7) and IPNS setup (#9).

## License

[MIT](LICENSE) — copy, modify, redistribute freely. See [CONTRIBUTING.md](CONTRIBUTING.md) for fork guidance.

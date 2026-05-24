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
2. Open any mirror — the badge must show the **same version number** and **same IPFS CID** (`bafybeigqenmqcvqupguyqr2dl4pb45dcipux62xc73u665p2iqzyb2sqle`).
3. The key fingerprint in the badge (`c1688ff0…b5c3`) must match [trusted-signers.json](trusted-signers.json).

If a mirror shows a different CID, a different version, or a different fingerprint — it is not serving authentic content.

You can also fetch the content directly from IPFS:
```
https://dweb.link/ipfs/bafybeigqenmqcvqupguyqr2dl4pb45dcipux62xc73u665p2iqzyb2sqle
```

## Run a volunteer mirror

Everything you need is bundled. A VPS with a public IP and a DNS record is enough — the stack builds itself, provisions a free TLS cert, and federates with the network.

```bash
git clone https://github.com/grussdorian/cjp-decentralised
cd cjp-decentralised
cp .env.example .env       # edit MIRROR_HOST, ACME_EMAIL, MIRROR_RELAY_URL
docker compose up -d
```

That's the whole setup. The stack brings up:

| Service | Role |
|---------|------|
| **caddy** | Auto-HTTPS via Let's Encrypt — provisions a cert for `MIRROR_HOST` on first run |
| **nginx** | Serves the static site + reverse-proxies `/ipfs/*` and `/relay` |
| **ipfs** (kubo) | Pins the latest signed CID; serves the self-hosted IPFS gateway |
| **relay** (strfry) | Bundled Nostr relay; daemon writes heartbeats here first |
| **mirror** | Builds from source; polls `latest.json`, verifies signatures, pins, broadcasts |
| **tor** | Optional `.onion` hidden service |

**For a public mirror**, set `MIRROR_HOST=cjp.example.com` in `.env`. The DNS A/AAAA record must already point at the VPS — Caddy uses the HTTP-01 challenge to get the cert, so the host must be reachable from the public internet on port 80 before the first `docker compose up`.

**For a local test stack**, leave `MIRROR_HOST` empty. Caddy serves HTTP-only on `:80`, no cert work happens, and the daemon still federates via public Nostr relays.

Your domain appears automatically in the [live mirror list](https://cjp.fheya.de/mirror.html) within ~2 minutes of the daemon broadcasting its first heartbeat. No PR, no manual registration.

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
| IPFS gateway | [`dweb.link/ipfs/bafybeigqenmqcvqupguyqr2dl4pb45dcipux62xc73u665p2iqzyb2sqle`](https://dweb.link/ipfs/bafybeigqenmqcvqupguyqr2dl4pb45dcipux62xc73u665p2iqzyb2sqle) |
| IPNS | pending |
| ENS | `cockroachjanataparty.eth` — pending on-chain registration |
| Tor | pending hidden service setup |

## Federated IPFS gateway (bundled)

Every mirror bundles a [kubo](https://github.com/ipfs/kubo) IPFS node with the current CID always pinned. The bundled `nginx.conf` reverse-proxies `/ipfs/<CID>` and `/ipns/<name>` straight to it. CID links on every page resolve same-origin: no DHT lookup, no third-party gateway flakiness, sub-100ms first-byte every time.

**To enable on your mirror:** set `MIRROR_HOST` in `.env` or `docker-compose.override.yml` to the hostname your reverse proxy serves on. The kubo init script configures the gateway to use path-mode (instead of the default subdomain mode, which would otherwise redirect `/ipfs/<CID>` to `<CID>.ipfs.<host>`).

```yaml
# docker-compose.override.yml
services:
  ipfs:
    environment:
      MIRROR_HOST: "cjp.mirror.example.com"
```

The mirror list on `/mirror.html` automatically links each mirror's advertised CID to that mirror's own gateway — federation works for cross-mirror browsing too.

## Federated relay (bundled)

Every mirror runs its own [strfry](https://github.com/hoytech/strfry) Nostr relay alongside IPFS. The mirror daemon writes heartbeats to its local relay first (guaranteed success), then to a small set of public relays for federation. Set `MIRROR_RELAY_URL` to your public WSS URL to advertise your relay to other mirrors — visiting browsers automatically discover it from your heartbeats and merge it into their query pool.

**Why it matters:**
- **No central point of failure.** As more volunteers join, the relay set grows automatically.
- **Daemon liveness doesn't depend on any public relay.** Local writes always succeed.
- **Resilient to relay policy changes.** When public relays add PoW requirements, disappear, or rate-limit, the federated set keeps working.

**Default config:** the relay is bundled but not exposed publicly. Volunteers opt-in to federation by setting `MIRROR_RELAY_URL` in `docker-compose.override.yml`:

```yaml
services:
  mirror:
    environment:
      MIRROR_RELAY_URL: "wss://mirror.example.com/relay"
```

The bundled `nginx.conf` reverse-proxies `/relay` to the strfry container with proper WebSocket upgrade headers, so no extra config is needed.

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

# Contributing to CJP Decentralized

Thank you for supporting this project. Every contribution — whether it is a translation, a bug fix, a mirror node, or a review — directly strengthens the censorship resistance of this platform.

---

## Table of contents

1. [Why this project is open source](#why-open-source)
2. [Community principles](#community-principles)
3. [Ways to contribute](#ways-to-contribute)
4. [Development setup](#development-setup)
5. [Contribution workflow](#contribution-workflow)
6. [Commit conventions](#commit-conventions)
7. [Translations](#translations)
8. [Running a mirror](#running-a-mirror)
9. [Becoming a trusted signer](#becoming-a-trusted-signer)
10. [Governance and decision making](#governance-and-decision-making)

---

## Why open source

This project is open source for reasons that go beyond the usual engineering arguments.

**Censorship resistance requires auditability.** A site that claims to be uncensorable cannot ask its users to trust a black box. Anyone must be able to read the code, verify that no data is leaked, confirm that forms work exactly as described, and check that the build artifact matches the published CID.

**Forks are a feature.** If this repository is taken down, any fork is an equally valid continuation. Open source ensures that no single GitHub account, developer, or jurisdiction can permanently stop the project. The permissive MIT license means any individual or group may copy, modify, and redistribute without restriction.

**Volunteer mirrors require trust.** People who run mirror nodes are taking on real-world risk. They deserve to read every line of code their node executes before they commit their infrastructure to it.

**Collective ownership.** The party does not own this software; the contributors do. Party leadership can direct content — what goes in the manifesto, which translations to accept — but the infrastructure and tooling belong to everyone who has contributed.

---

## Community principles

1. **Privacy by default.** Never add telemetry, analytics, or server-side logging that could identify visitors. The architecture is intentionally designed so that even the project maintainers cannot see who visits the site or submits a form.

2. **No single point of trust.** Avoid designs that require trusting one person, one server, or one service. If you find a dependency that creates a choke point, open an issue.

3. **Work offline.** Features that depend on external APIs, CDNs, or services that can be blocked (including Cloudflare, Google, and large cloud providers) should have offline fallbacks or be removed. The site must work fully on Tor Browser with JavaScript restricted to local scripts only.

4. **Minimal footprint.** No frameworks, no bundlers, no npm dependency trees in the browser. Plain HTML, CSS, and vanilla JS. The smaller the artifact, the harder it is to find an attack surface and the easier it is for volunteers to audit.

5. **Inclusive participation.** Contributions are evaluated on their technical and political merit, not on the identity of the contributor. The project explicitly welcomes contributors who are Dalit, Bahujan, Adivasi, Muslim, LGBTQ+, female, non-binary, or otherwise marginalized — because the people most affected by the conditions this project addresses should have the most voice in how it works.

---

## Ways to contribute

### Run a mirror (no technical skill required)

The easiest and most impactful contribution. See [Running a mirror](#running-a-mirror).

### Translate content

The manifesto, join page, and demand page need accurate human-reviewed translations. See [Translations](#translations).

### Report bugs

Open a GitHub issue. Include steps to reproduce, observed behavior, and expected behavior. For security issues, do **not** open a public issue — follow [SECURITY.md](SECURITY.md) instead.

### Fix bugs and implement features

Open a pull request against `main`. Read [Contribution workflow](#contribution-workflow) first.

### Review pull requests

Code review is valuable even if you cannot write code. Check translations for accuracy, read change descriptions for logical errors, and test the site in your browser.

### Improve documentation

README, setup guides, deployment instructions. Keep language direct and avoid assumptions about the reader's political or technical background.

### Audit dependencies

The project uses `nostr-tools` and the `age-encryption` library via CDN (currently `esm.sh`). Auditing those libraries, proposing local bundles, or checking subresource integrity hashes is a high-value contribution.

---

## Development setup

**Requirements**

- Node.js 20+
- Go 1.22+
- Git

**Clone and build**

```bash
git clone https://github.com/grussdorian/cjp-decentralised
cd cjp-decentralised
node scripts/build.js
```

The build script renders the site into `dist/`. Serve it locally:

```bash
python3 -m http.server 7070 --directory dist/
```

Open `http://localhost:7070` in your browser.

**Build the Go tools**

```bash
# Publisher CLI (signing and publishing)
cd packages/publisher && go build -o /usr/local/bin/publisher . && cd ../..

# Mirror daemon
cd packages/mirror && go build -o /usr/local/bin/cjp-mirror . && cd ../..
```

**Run tests**

```bash
# Go packages
cd packages/publisher && go test ./...
cd packages/mirror && go test ./...
```

There are currently no automated frontend tests. Manual testing in Firefox and Tor Browser is expected before submitting frontend PRs.

---

## Contribution workflow

1. **Fork** the repository to your own GitHub account.

2. **Create a branch** from `main` with a descriptive name:
   ```
   feat/bengali-translation
   fix/pow-worker-edge-case
   docs/mirror-setup-guide
   ```

3. **Make your changes.** Keep each PR focused on one logical change. If you are fixing a bug and notice an unrelated issue, open a separate PR for the second fix.

4. **Test locally.** Build the site with `node scripts/build.js` and verify your change works as expected in a browser. For Go changes, run `go test ./...`.

5. **Open a pull request** against `main`. Write a clear description of:
   - What changed and why
   - How you tested it
   - Any risks or trade-offs

6. **Address review comments.** All PRs require at least one approving review from a maintainer before merging.

7. **CI must pass.** The `build` check (site build + IPFS CID computation) must succeed. If CI is failing for reasons unrelated to your change, note that in the PR description.

**For translation PRs specifically:** Tag a native speaker in the PR description and wait for their sign-off before requesting maintainer review. Do not merge a translation PR without at least one native-speaker approval.

---

## Commit conventions

Follow the [Conventional Commits](https://www.conventionalcommits.org/) format:

```
<type>(<scope>): <short description>

[optional body]
```

**Types:**

| Type | Use for |
|------|---------|
| `feat` | New feature or page |
| `fix` | Bug fix |
| `docs` | Documentation only |
| `chore` | Build, CI, tooling, dependency updates |
| `i18n` | Translation changes |
| `refactor` | Code restructuring without behavior change |
| `security` | Security fix (use sparingly; coordinate with maintainers first) |

**Scopes** (optional but useful): `site`, `mirror`, `publisher`, `ci`, `docker`, `translations`.

**Examples:**

```
feat(site): add Bengali manifesto translation
fix(publisher): handle empty IPFS API response
chore(ci): pin kubo to v0.29.0
i18n(hi): correct Hindi translation of "political prisoner"
```

**Rules:**
- Use the imperative mood: "add", not "adds" or "added"
- Do not end the subject line with a period
- Keep the subject line under 72 characters
- The body (if present) should explain *why*, not *what*

---

## Translations

The manifesto and all page content are maintained in `content/`. The English source is authoritative. Translations live in `content/translations/`.

**Adding or updating a translation**

1. Read the English source in `content/manifesto/manifesto.md` and the English JSON in `content/translations/en.json`.

2. Edit or create the relevant language JSON file (e.g., `content/translations/ta.json` for Tamil).

3. Rebuild with `node scripts/build.js` and verify the rendered page looks correct.

4. Open a PR with the type `i18n`. In the PR description, tag at least one native speaker who can verify accuracy.

**Translation principles:**

- Translate the meaning, not just the words. Political vocabulary differs across languages; use terms that will be understood by the intended audience.
- Do not censor or soften content for presumed sensitivities. The translated text should carry the same force as the English original.
- If a term has no clean equivalent, transliterate and add a brief parenthetical explanation in the same language.
- Translations that have not been reviewed by a native speaker are marked as unreviewed in the build and are not linked from the main navigation until approved.

**Currently approved for publication:** English, Bengali  
**Pending human review:** Hindi, Tamil, Telugu

---

## Running a mirror

Mirror operators are volunteers who pin the current site to their IPFS node and optionally serve it via HTTP and/or Tor. Running a mirror is the most operationally valuable thing you can do for this project.

**Requirements**
- A machine with a public IP address (VPS or home server with port forwarding)
- Docker and Docker Compose
- Port 4001 TCP/UDP open for IPFS swarm
- Optional: ports 80/443 open for HTTP clearweb serving

**Setup**

```bash
git clone https://github.com/grussdorian/cjp-decentralised
cd cjp-decentralised
docker compose up -d
```

The mirror daemon will:
1. Fetch `latest.json` from Nostr relays, IPNS, and fallback URLs every 15 minutes.
2. Verify the Ed25519 signature against `trusted-signers.json`.
3. Run `ipfs pin add <CID>` if a new valid version is detected.
4. Post a signed heartbeat event to Nostr (tagged `#cjp-mirrors`) so the mirror registry on the site stays current.

**Security note for mirror operators:** You are not required to keep any secrets. The mirror daemon only reads public data (signed `latest.json`) and only needs write access to your local IPFS node. It does not handle form submissions, user data, or private keys.

**If your server's outbound port 4001 is firewalled:** IPFS can also route over WebSockets on port 443. Set `IPFS_SWARM_OPTS=--enable-pubsub-experiment` in your environment and configure kubo to use WebSocket bootstrappers. See the kubo documentation for details.

**Nginx / clearweb serving:** The included `nginx.conf` proxies to the local IPFS gateway and serves the current pinned version at port 8081. To serve on port 80, update the port mapping in `docker-compose.yml`.

---

## Becoming a trusted signer

Trusted signers are developers who can publish new versions of the site. The list of trusted signers is maintained in `trusted-signers.json` at the repo root. Only the current Ed25519 pubkeys listed in that file can produce `latest.json` updates that mirror daemons will accept.

**Applying to become a trusted signer**

1. Generate a keypair:
   ```bash
   publisher keygen --out ~/.cjp/
   ```
   This writes `~/.cjp/signing.key` (private, never share) and `~/.cjp/signing.pub` (safe to share).

2. Open a PR that adds your pubkey (the hex string from `signing.pub`) to `trusted-signers.json`. Include in the PR description:
   - Who you are (pseudonym is fine)
   - Why you want signing access
   - Confirmation that you understand the key security requirements

3. Two existing trusted signers must approve and merge the PR.

**Key security requirements:**
- The private key (`signing.key`) must never be committed to any repository, logged, or transmitted over a network.
- Store it with permissions `600` (`chmod 600 ~/.cjp/signing.key`).
- If your key is compromised, immediately notify the other trusted signers. They can merge a PR removing your pubkey from `trusted-signers.json`, after which your key can no longer update mirrors.
- Do not use the signing key on shared or cloud machines.

**Revoking a signer:** Open a PR removing the relevant pubkey from `trusted-signers.json`. One other trusted signer can merge this PR. Mirror daemons will stop accepting signatures from the revoked key immediately after the updated `trusted-signers.json` is deployed.

---

## Governance and decision making

This project has no formal hierarchy beyond a rough maintainer/contributor distinction.

**Maintainers** are people with merge access to `main`. They are responsible for reviewing PRs, running CI, managing trusted-signers.json, and making judgment calls on disputed contributions. Maintainers are trusted signers.

**Contributors** are anyone who opens a PR, files an issue, runs a mirror, or contributes a translation.

**Decision making**

- Routine changes (bug fixes, translation updates, documentation) are merged after one maintainer approval.
- Changes to `trusted-signers.json`, the signing or encryption protocol, or the mirror daemon's verification logic require two maintainer approvals and a 48-hour open comment window.
- Changes to the manifesto content require consensus among active party leadership and at least one maintainer review.
- No change that weakens privacy protections, removes encryption, or adds centralized dependencies may be merged without a public discussion period of at least 7 days and explicit justification.

**Forking policy:** You are free to fork this project under the terms of the MIT License without asking for permission. If your fork is intended to serve a different political party or movement, we encourage you to change the party name, age encryption keys, Nostr keys, and trusted signers before deploying — using this infrastructure with the original keys would allow the original parties to decrypt submissions intended for your fork.

**Inactive maintainers:** A maintainer who has not reviewed a PR or made a commit in 6 months is considered inactive. Inactive maintainers may be removed from the maintainer list by consensus of the remaining active maintainers. Their signing key should also be removed from `trusted-signers.json` at that time.

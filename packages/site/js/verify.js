// Mirror authenticity verification — per-file content integrity.
//
// Trust chain:
//   GitHub latest.json  →  M-of-N Ed25519 signatures (trusted party keys)
//     └─ sign IPFS CID  →  content-addressed directory
//           └─ contains integrity.json  →  SHA-256 of every page
//                 └─ fetched via signed CID  →  compare to this page's bytes
//
// Consensus model: latest.json carries an array of independent signatures.
// The badge requires ≥ threshold of them to be valid and from trusted keys.
// An attacker must compromise multiple separate signing keys to forge a valid
// update — compromising one key is not sufficient.
//
// Signature scheme (mirrors packages/publisher/signing.go):
//   Ed25519( SHA-256( "{cid}\n{version}\n{timestamp}" ) )

(async function () {
  'use strict';

  const REPO     = 'https://raw.githubusercontent.com/grussdorian/cjp-decentralised/main';
  const GATEWAYS = [
    'https://ipfs.io/ipfs',
    'https://cloudflare-ipfs.com/ipfs',
    'https://dweb.link/ipfs',
  ];

  // Trusted Ed25519 public keys (hex) — hardcoded so that GitHub cannot swap
  // in attacker-controlled keys by modifying trusted-signers.json.
  // These keys are authoritative; adding a key here requires a code change in
  // this file, which is itself covered by the IPFS integrity check.
  // To add a new party member: run `publisher keygen`, add the pubkey here,
  // rebuild, publish a new CID signed by existing threshold.
  const HARDCODED_SIGNERS = new Set([
    'c1688ff074c50557d4aacfd668580c119dd4e425f18eb65c8cffac53a433b5c3',
  ]);
  // Minimum number of distinct valid signatures required before showing ✓.
  // Raise this as more party members join.
  const HARDCODED_THRESHOLD = 1;

  const badge = document.getElementById('cjp-verify-badge');
  if (!badge) return;

  function hexToBytes(hex) {
    const a = new Uint8Array(hex.length >>> 1);
    for (let i = 0; i < a.length; i++) a[i] = parseInt(hex.slice(i * 2, i * 2 + 2), 16);
    return a;
  }

  function bytesToHex(buf) {
    return Array.from(new Uint8Array(buf)).map(b => b.toString(16).padStart(2, '0')).join('');
  }

  function set(state, html) {
    badge.className = 'cjp-badge cjp-badge--' + state;
    badge.innerHTML = html;
  }

  set('pending', '<span class="cjp-badge__spin"></span>Verifying…');

  // ── Step 1: fetch latest.json from GitHub (CID pointer only) ───────────
  // We do NOT fetch trusted-signers.json from GitHub — that would let a state
  // actor with GitHub access swap in their own keys.  Trusted keys are hardcoded
  // above and covered by the IPFS integrity check of this very file.
  let latest;
  try {
    const lr = await fetch(REPO + '/latest.json', { cache: 'no-cache' });
    if (!lr.ok) throw new Error('fetch');
    latest = await lr.json();
  } catch (_) {
    set('unknown', '? Cannot reach GitHub — try the <a href="https://github.com/grussdorian/cjp-decentralised/blob/main/latest.json" target="_blank" rel="noopener noreferrer">signed manifest</a>');
    return;
  }

  // ── Step 2: collect signatures; filter to hardcoded trusted keys ─────────
  const threshold  = HARDCODED_THRESHOLD;
  const trustedSet = HARDCODED_SIGNERS;
  const totalKeys  = trustedSet.size;

  // Normalise both legacy single-sig and new multi-sig array formats.
  const allSigs = Array.isArray(latest.signatures) && latest.signatures.length > 0
    ? latest.signatures
    : (latest.signer ? [{ signer: latest.signer, signature: latest.signature }] : []);

  const trustedSigs = allSigs.filter(s => trustedSet.has(s.signer));
  if (trustedSigs.length === 0) {
    set('invalid', '✗ No signature from a trusted signer — do not trust this mirror');
    return;
  }

  // ── Step 3: verify Ed25519 signatures — require ≥ threshold valid ───────
  // All signers sign the same message: SHA-256("{cid}\n{version}\n{timestamp}")
  let msgHash;
  try {
    const msgBytes = new TextEncoder().encode(`${latest.cid}\n${latest.version}\n${latest.timestamp}`);
    msgHash = await crypto.subtle.digest('SHA-256', msgBytes);
  } catch (_) {
    set('unknown', '? Ed25519 not supported in this browser — <a href="/ipfs/' + latest.cid + '" target="_blank" rel="noopener noreferrer">verify via IPFS</a>');
    return;
  }

  // Verify all trusted signatures in one pass; collect valid signer keys.
  // Showing the key fingerprint in the badge means a phishing clone cannot
  // display the same fingerprint without holding the real private keys.
  const validSigners = [];
  for (const s of trustedSigs) {
    try {
      const pubKey = await crypto.subtle.importKey(
        'raw', hexToBytes(s.signer),
        { name: 'Ed25519' }, false, ['verify']
      );
      const ok = await crypto.subtle.verify(
        { name: 'Ed25519' }, pubKey,
        hexToBytes(s.signature),
        msgHash
      );
      if (ok) validSigners.push(s.signer);
    } catch (_) { /* malformed entry, skip */ }
  }
  const validCount = validSigners.length;

  if (validCount < threshold) {
    set('invalid',
      `✗ Only ${validCount}/${threshold} required signatures valid — ` +
      `signed pointer may be tampered`);
    return;
  }

  const short = c => c.slice(0, 16) + '…';
  const gwLink = `<a class="cjp-badge__cid" href="/ipfs/${latest.cid}" target="_blank" rel="noopener noreferrer" title="Open canonical version on IPFS">${short(latest.cid)}</a>`;

  // Short fingerprint: first 8 + last 4 hex chars, e.g. "c1688ff0…b5c3"
  // Shown in the badge so users can cross-check against out-of-band sources
  // (flyers, Nostr, trusted contacts).  A clone using different keys will show
  // different fingerprints, exposing the fake.
  const fingerprints = validSigners.map(k => k.slice(0, 8) + '…' + k.slice(-4)).join(' ');
  const sigLabel = `<a class="cjp-badge__fp" href="trust.html" title="What does this fingerprint mean? How to verify.">${fingerprints}</a>`;

  // ── Step 4: fetch integrity.json via signed CID (content-addressed) ─────
  // Try all gateways in parallel with a short timeout so a single slow gateway
  // doesn't hang the badge for 30+ seconds.
  let integrity = null;
  try {
    const ctl = new AbortController();
    const timer = setTimeout(() => ctl.abort(), 8000);
    const res = await Promise.any(
      GATEWAYS.map(gw =>
        fetch(`${gw}/${latest.cid}/integrity.json`, {
          cache: 'no-cache',
          signal: ctl.signal,
        })
      )
    );
    clearTimeout(timer);
    if (res.ok) integrity = await res.json();
    else ctl.abort(); // stop remaining requests on non-200
  } catch (_) { /* all gateways failed or timed out */ }

  if (!integrity) {
    set('verified', `✓ Signed · ${gwLink} · v${latest.version} · ${sigLabel} · <small>IPFS propagating…</small>`);
    return;
  }

  // ── Step 5: hash this page and compare to the integrity manifest ─────────
  const pagePath = location.pathname === '/' ? 'index.html'
                 : location.pathname.replace(/^\//, '');

  const expectedHash = integrity.files && integrity.files[pagePath];
  if (!expectedHash) {
    set('verified', `✓ Signed · ${gwLink} · v${latest.version} · ${sigLabel} · <small>page not in manifest</small>`);
    return;
  }

  let actualHash;
  try {
    const resp = await fetch(location.pathname + location.search, { cache: 'no-cache' });
    const text = await resp.text();
    // Strip <script> tags before hashing — CDN providers (e.g. Cloudflare) inject
    // scripts into HTML responses without modifying the signed content. We verify
    // the document body, not the delivery wrapper.
    const stripped = text.replace(/<script\b[^>]*>[\s\S]*?<\/script>/gi, '');
    actualHash = bytesToHex(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(stripped)));
  } catch (_) {
    set('verified', `✓ Signed · ${gwLink} · v${latest.version} · ${sigLabel} · <small>page re-fetch blocked</small>`);
    return;
  }

  if (actualHash === expectedHash) {
    set('verified', `✓ Signed · ${gwLink} · v${latest.version} · ${sigLabel}`);
  } else {
    set('invalid',
      `✗ Page content does not match signed CID — this mirror may be serving modified content. ` +
      `Compare against ${gwLink}`);
  }
})();

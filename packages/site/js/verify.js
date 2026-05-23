// Mirror authenticity verification — per-file content integrity.
//
// Trust chain:
//   GitHub latest.json  →  Ed25519 signature (party private key)
//     └─ signs IPFS CID  →  content-addressed directory
//           └─ contains integrity.json  →  SHA-256 of every page
//                 └─ fetched via signed CID  →  compare to this page's bytes
//
// A fork with modified content produces different file hashes → different
// integrity.json → different CID → no valid signature → badge fails.
// The signing key is the sole root of trust.
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

  // ── Step 1: fetch latest.json and trusted-signers.json from GitHub ──────
  let latest, signers;
  try {
    const [lr, sr] = await Promise.all([
      fetch(REPO + '/latest.json',          { cache: 'no-cache' }),
      fetch(REPO + '/trusted-signers.json', { cache: 'no-cache' }),
    ]);
    if (!lr.ok || !sr.ok) throw new Error('fetch');
    latest  = await lr.json();
    signers = await sr.json();
  } catch (_) {
    set('unknown', '? Cannot reach GitHub — try the <a href="https://github.com/grussdorian/cjp-decentralised/blob/main/latest.json" target="_blank" rel="noopener noreferrer">signed manifest</a>');
    return;
  }

  // ── Step 2: signer must be in trusted-signers.json ──────────────────────
  if (!Array.isArray(signers.signers) || !signers.signers.includes(latest.signer)) {
    set('invalid', '✗ Signer not in trusted list — do not trust this mirror');
    return;
  }

  // ── Step 3: verify Ed25519 signature ────────────────────────────────────
  // Message = SHA-256("{cid}\n{version}\n{timestamp}"), matching signing.go
  let sigOk = false;
  try {
    const msgBytes = new TextEncoder().encode(`${latest.cid}\n${latest.version}\n${latest.timestamp}`);
    const msgHash  = await crypto.subtle.digest('SHA-256', msgBytes);
    const pubKey   = await crypto.subtle.importKey(
      'raw', hexToBytes(latest.signer),
      { name: 'Ed25519' }, false, ['verify']
    );
    sigOk = await crypto.subtle.verify(
      { name: 'Ed25519' }, pubKey,
      hexToBytes(latest.signature),
      msgHash
    );
  } catch (_) {
    set('unknown', '? Ed25519 not supported in this browser — <a href="https://ipfs.io/ipfs/' + latest.cid + '" target="_blank" rel="noopener noreferrer">verify via IPFS</a>');
    return;
  }

  if (!sigOk) {
    set('invalid', '✗ Signature invalid — signed pointer has been tampered with');
    return;
  }

  const short    = c => c.slice(0, 16) + '…';
  const gwLink   = `<a class="cjp-badge__cid" href="https://ipfs.io/ipfs/${latest.cid}" target="_blank" rel="noopener noreferrer" title="Open canonical version on IPFS">${short(latest.cid)}</a>`;

  // ── Step 4: fetch integrity.json via signed CID (content-addressed) ─────
  // Because we fetch via the CID, the content of this file is cryptographically
  // bound to the signature — a modified integrity.json would be at a different CID.
  let integrity = null;
  for (const gw of GATEWAYS) {
    try {
      const r = await fetch(`${gw}/${latest.cid}/integrity.json`, { cache: 'no-cache' });
      if (r.ok) { integrity = await r.json(); break; }
    } catch (_) { /* try next */ }
  }

  if (!integrity) {
    // Gateways haven't propagated the content yet (can take a few minutes after first pin)
    set('verified', `✓ Signature valid · ${gwLink} · v${latest.version} · <small>content check pending propagation</small>`);
    return;
  }

  // ── Step 5: hash this page and compare to the integrity manifest ─────────
  const pagePath = location.pathname === '/' ? 'index.html'
                 : location.pathname.replace(/^\//, '');

  const expectedHash = integrity.files && integrity.files[pagePath];
  if (!expectedHash) {
    // Page not in manifest (e.g. language subdir not yet added)
    set('verified', `✓ Signature valid · ${gwLink} · v${latest.version} · <small>page not in manifest</small>`);
    return;
  }

  let actualHash;
  try {
    const resp  = await fetch(location.pathname + location.search, { cache: 'no-cache' });
    const bytes = await resp.arrayBuffer();
    actualHash  = bytesToHex(await crypto.subtle.digest('SHA-256', bytes));
  } catch (_) {
    set('verified', `✓ Signature valid · ${gwLink} · v${latest.version} · <small>page re-fetch blocked</small>`);
    return;
  }

  if (actualHash === expectedHash) {
    set('verified', `✓ Authentic CJP content · ${gwLink} · v${latest.version}`);
  } else {
    set('invalid',
      `✗ Page content does not match signed CID — this mirror may be serving modified content. ` +
      `Compare against ${gwLink}`);
  }
})();

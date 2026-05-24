// Queries Nostr relays for mirror heartbeat events and displays live stats.
//
// Federation model: every volunteer mirror runs its own bundled strfry relay
// and advertises its public WSS URL in the heartbeat's `relay_url` field.
// This module bootstraps from the small RELAYS list, then merges every
// discovered relay_url into the query pool for the next refresh. Result:
// the relay set grows with volunteer count, no central registry needed.
import { RELAYS, MIRROR_TAG } from './relays.js';

// Heartbeat window: mirrors that haven't sent a heartbeat within this many
// seconds are not counted. Mirror daemons beat every 60s ±10s.
const HEARTBEAT_WINDOW_S = 300;

// Whitelist of CID character class — base32 + base58. Anything else is dropped.
const CID_PATTERN = /^[A-Za-z0-9]{20,80}$/;
// Whitelist for ISO country codes / short identifiers displayed in the list.
const COUNTRY_PATTERN = /^[A-Za-z0-9 _\-]{0,32}$/;

// Runtime-discovered relay URLs from past heartbeats. Persists across refreshes
// within the page session. Bounded to keep the query fan-out reasonable.
const MAX_DISCOVERED_RELAYS = 30;
const discoveredRelays = new Set();

function safeURL(s) {
  if (typeof s !== 'string') return null;
  try {
    const u = new URL(s);
    if (u.protocol !== 'https:' && u.protocol !== 'http:') return null;
    return u;
  } catch {
    return null;
  }
}

function safeRelayURL(s) {
  if (typeof s !== 'string') return null;
  try {
    const u = new URL(s);
    if (u.protocol !== 'wss:' && u.protocol !== 'ws:') return null;
    return u.toString();
  } catch {
    return null;
  }
}

function el(tag, attrs, text) {
  const e = document.createElement(tag);
  if (attrs) for (const [k, v] of Object.entries(attrs)) e.setAttribute(k, v);
  if (text !== undefined) e.textContent = text;
  return e;
}

export async function loadMirrorStats(countEl, listEl) {
  const since = Math.floor(Date.now() / 1000) - HEARTBEAT_WINDOW_S;
  const filter = { kinds: [1], '#t': [MIRROR_TAG], since, limit: 200 };

  const seen = new Map(); // nostr pubkey → latest event

  // Query bootstrap RELAYS plus everything discovered from previous beats.
  const urls = Array.from(new Set([...RELAYS, ...discoveredRelays]));
  await Promise.allSettled(urls.map(url => queryRelay(url, filter, seen)));

  // Harvest relay_url advertisements from this round for the next refresh.
  // Federation grows organically as more mirrors come online.
  for (const [, ev] of seen) {
    try {
      const data = JSON.parse(ev.content);
      const u = safeRelayURL(data.relay_url);
      if (u && !discoveredRelays.has(u) && discoveredRelays.size < MAX_DISCOVERED_RELAYS) {
        discoveredRelays.add(u);
      }
    } catch {}
  }

  if (countEl) {
    countEl.textContent = seen.size;
  }

  if (!listEl) return;

  // Reset list
  listEl.replaceChildren();

  if (seen.size === 0) {
    const p = el('p', { style: 'color:var(--muted);font-size:.875rem' });
    p.append('No active mirrors in the last hour. ');
    p.appendChild(el('a', { href: 'mirror.html' }, 'Be the first.'));
    listEl.appendChild(p);
    return;
  }

  // Heartbeat content is attacker-controlled (anyone can publish a Nostr event
  // with the cjp-mirrors tag). Treat every field as untrusted and build the DOM
  // via textContent / strict whitelists — never innerHTML.
  for (const [peer, ev] of seen) {
    let data;
    try { data = JSON.parse(ev.content); } catch { continue; }
    if (!data || typeof data !== 'object') continue;

    const div = el('div', { class: 'stat-box' });

    const peerCode = el('code', null, peer.slice(0, 16) + '…');
    div.appendChild(peerCode);
    div.appendChild(el('br'));

    const small = el('small', { style: 'color:var(--muted)' });
    const country = (typeof data.country === 'string' && COUNTRY_PATTERN.test(data.country))
      ? data.country : 'Unknown';
    small.append(country, ' · ');

    if (typeof data.cid === 'string' && CID_PATTERN.test(data.cid)) {
      // Federation: prefer the originating mirror's own gateway when it
      // advertises a URL — that mirror has the CID pinned by definition.
      // Fall back to dweb.link if no URL is advertised.
      const advertisedURL = safeURL(data.url);
      const cidHref = advertisedURL
        ? advertisedURL.origin + '/ipfs/' + data.cid
        : 'https://dweb.link/ipfs/' + data.cid;
      small.appendChild(el('a', {
        href: cidHref,
        target: '_blank',
        rel: 'noopener noreferrer',
        style: 'color:var(--muted)',
      }, 'CID ' + data.cid.slice(0, 12) + '…'));
    } else {
      small.append('CID unknown');
    }

    const url = safeURL(data.url);
    if (url) {
      small.append(' · ');
      small.appendChild(el('a', {
        href: url.href,
        target: '_blank',
        rel: 'noopener noreferrer',
        style: 'color:var(--accent)',
      }, url.hostname));
    }

    div.appendChild(small);
    listEl.appendChild(div);
  }
}

function queryRelay(url, filter, seen) {
  return new Promise((resolve) => {
    const ws = new WebSocket(url);
    const subId = Math.random().toString(36).slice(2);
    const timer = setTimeout(() => { ws.close(); resolve(); }, 5000);
    ws.onopen = () => ws.send(JSON.stringify(['REQ', subId, filter]));
    ws.onmessage = (msg) => {
      const data = JSON.parse(msg.data);
      if (data[0] === 'EOSE') { clearTimeout(timer); ws.close(); resolve(); return; }
      if (data[0] === 'EVENT') {
        const ev = data[2];
        const peer = ev.pubkey;
        if (!seen.has(peer) || seen.get(peer).created_at < ev.created_at) {
          seen.set(peer, ev);
        }
      }
    };
    ws.onerror = () => { clearTimeout(timer); resolve(); };
  });
}

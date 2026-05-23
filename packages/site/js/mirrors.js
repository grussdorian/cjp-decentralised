// Queries Nostr relays for mirror heartbeat events and displays live stats.
import { RELAYS, MIRROR_TAG } from './relays.js';

const ONE_HOUR = 3600;

export async function loadMirrorStats(countEl, listEl) {
  const since = Math.floor(Date.now() / 1000) - ONE_HOUR;
  const filter = { kinds: [1], '#t': [MIRROR_TAG], since, limit: 200 };

  const seen = new Map(); // peer_id → latest event

  await Promise.allSettled(RELAYS.slice(0, 5).map(url => queryRelay(url, filter, seen)));

  if (countEl) {
    countEl.textContent = seen.size;
  }

  if (listEl) {
    if (seen.size === 0) {
      listEl.innerHTML = '<p style="color:var(--muted);font-size:.875rem">No active mirrors in the last hour. <a href="mirror.html">Be the first.</a></p>';
    } else {
      listEl.innerHTML = '';
      for (const [peer, ev] of seen) {
        try {
          const data = JSON.parse(ev.content);
          const div = document.createElement('div');
          div.className = 'stat-box';
          const cidShort = (data.cid || '').slice(0, 12) + '…';
          const cidLink  = data.cid
            ? `<a href="https://ipfs.io/ipfs/${data.cid}" target="_blank" rel="noopener noreferrer" style="color:var(--muted)">CID ${cidShort}</a>`
            : 'CID unknown';
          div.innerHTML = `<code>${peer.slice(0, 16)}…</code><br><small style="color:var(--muted)">${data.country || 'Unknown'} · ${cidLink}</small>`;
          listEl.appendChild(div);
        } catch {}
      }
    }
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
        try {
          const d = JSON.parse(ev.content);
          const peer = d.peer_id || ev.pubkey;
          if (!seen.has(peer) || seen.get(peer).created_at < ev.created_at) {
            seen.set(peer, ev);
          }
        } catch {}
      }
    };
    ws.onerror = () => { clearTimeout(timer); resolve(); };
  });
}

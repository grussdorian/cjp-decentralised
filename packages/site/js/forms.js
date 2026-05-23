// Nostr form submission handler.
// TODO: Replace CDN import with local bundle: import { ... } from '/js/nostr-tools.bundle.js';
import {
  generateSecretKey,
  getPublicKey,
  finalizeEvent,
  nip04,
} from 'https://esm.sh/nostr-tools@2.10.4';

import { RELAYS, PARTY_PUBKEY, DEMAND_TAG } from './relays.js';

async function broadcast(event) {
  const results = await Promise.allSettled(
    RELAYS.map(url => publishToRelay(url, event))
  );
  const ok = results.filter(r => r.status === 'fulfilled').length;
  return { ok, total: RELAYS.length };
}

function publishToRelay(url, event) {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(url);
    const timer = setTimeout(() => { ws.close(); reject(new Error('timeout')); }, 6000);
    ws.onopen = () => ws.send(JSON.stringify(['EVENT', event]));
    ws.onmessage = (msg) => {
      clearTimeout(timer);
      ws.close();
      const data = JSON.parse(msg.data);
      if (data[0] === 'OK' && data[2] === true) resolve();
      else reject(new Error(data[3] || 'relay rejected'));
    };
    ws.onerror = () => { clearTimeout(timer); reject(new Error('ws error')); };
  });
}

function getCaptchaToken(form) {
  const el = form.querySelector('frc-captcha');
  return el ? el.dataset.solution || '' : '';
}

function setStatus(form, type, text) {
  let el = form.querySelector('.status');
  if (!el) return;
  el.className = 'status ' + type;
  el.textContent = text;
}

// ── Join form (encrypted NIP-04 DM to party pubkey) ──
export async function handleJoin(form) {
  const btn = form.querySelector('[type=submit]');
  btn.disabled = true;

  const name = form.elements.name.value.trim();
  const state = form.elements.state.value;
  const captcha = getCaptchaToken(form);

  if (!name || !state) {
    setStatus(form, 'err', 'Please fill in all fields.');
    btn.disabled = false;
    return;
  }
  if (!captcha) {
    setStatus(form, 'err', 'Please complete the verification.');
    btn.disabled = false;
    return;
  }
  if (PARTY_PUBKEY === 'REPLACE_WITH_PARTY_PUBKEY_HEX') {
    setStatus(form, 'err', 'Party key not configured yet. Try again soon.');
    btn.disabled = false;
    return;
  }

  try {
    const sk = generateSecretKey();
    const pk = getPublicKey(sk);
    const content = await nip04.encrypt(sk, PARTY_PUBKEY, JSON.stringify({ name, state }));
    const event = finalizeEvent({
      kind: 4,
      created_at: Math.floor(Date.now() / 1000),
      tags: [['p', PARTY_PUBKEY]],
      content,
    }, sk);

    const { ok, total } = await broadcast(event);
    if (ok > 0) {
      setStatus(form, 'ok', `Submitted to ${ok}/${total} relays. Welcome to the Cockroach Army!`);
      form.reset();
    } else {
      setStatus(form, 'err', 'Could not reach any relays. Check your connection and try again.');
    }
  } catch (e) {
    setStatus(form, 'err', 'Error: ' + e.message);
  } finally {
    btn.disabled = false;
  }
}

// ── Demand form (public NIP-01 event, tagged #cjp-demand) ──
export async function handleDemand(form) {
  const btn = form.querySelector('[type=submit]');
  btn.disabled = true;

  const name = form.elements.name.value.trim();
  const city = form.elements.city.value.trim();
  const country = form.elements.country.value.trim();
  const captcha = getCaptchaToken(form);

  if (!name || !city || !country) {
    setStatus(form, 'err', 'Please fill in all fields.');
    btn.disabled = false;
    return;
  }
  if (!captcha) {
    setStatus(form, 'err', 'Please complete the verification.');
    btn.disabled = false;
    return;
  }

  try {
    const sk = generateSecretKey();
    const pk = getPublicKey(sk);
    const event = finalizeEvent({
      kind: 1,
      created_at: Math.floor(Date.now() / 1000),
      tags: [['t', DEMAND_TAG]],
      content: JSON.stringify({ name, city, country }),
    }, sk);

    const { ok, total } = await broadcast(event);
    if (ok > 0) {
      setStatus(form, 'ok', `Signature recorded on ${ok}/${total} relays. Your voice is heard.`);
      form.reset();
    } else {
      setStatus(form, 'err', 'Could not reach any relays. Check your connection and try again.');
    }
  } catch (e) {
    setStatus(form, 'err', 'Error: ' + e.message);
  } finally {
    btn.disabled = false;
  }
}

// Nostr form submission handler.
// Sign-up submissions are age-encrypted to all keys listed in PARTY_AGE_KEYS.
// Any single key holder can independently decrypt — no coordination needed.
// TODO: Replace CDN imports with local bundles.
import {
  generateSecretKey,
  getPublicKey,
  finalizeEvent,
} from 'https://esm.sh/nostr-tools@2.10.4';

import { encrypt as ageEncrypt } from 'https://esm.sh/age-encryption@0.1.2';

import { RELAYS, PARTY_AGE_KEYS, DEMAND_TAG } from './relays.js';

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
  const el = form.querySelector('[name="pow-token"]');
  return el ? el.value : '';
}

function setStatus(form, type, text) {
  let el = form.querySelector('.status');
  if (!el) return;
  el.className = 'status ' + type;
  el.textContent = text;
}

// ── Join form (age-encrypted to all party member keys, posted as Nostr event) ──
// PARTY_AGE_KEYS is a list of age1... public keys from party-keys.txt.
// Any single key holder can independently decrypt the submission.
export async function handleJoin(form) {
  const btn = form.querySelector('[type=submit]');
  btn.disabled = true;

  const name = form.elements.name.value.trim();
  const location = form.elements.location.value.trim();
  const captcha = getCaptchaToken(form);

  if (!name || !location) {
    setStatus(form, 'err', 'Please fill in all fields.');
    btn.disabled = false;
    return;
  }
  if (!captcha) {
    setStatus(form, 'err', 'Proof-of-work not yet complete. Please wait a moment.');
    btn.disabled = false;
    return;
  }
  const activeKeys = PARTY_AGE_KEYS.filter(k => !k.startsWith('#') && k.trim());
  if (activeKeys.length === 0) {
    setStatus(form, 'err', 'Party keys not configured yet. Try again soon.');
    btn.disabled = false;
    return;
  }

  try {
    // age-encrypt the payload to ALL party member keys simultaneously.
    // Each member can independently decrypt with their own private key.
    const plaintext = new TextEncoder().encode(JSON.stringify({ name, location, ts: Date.now() }));
    const ciphertext = await ageEncrypt(plaintext, activeKeys);
    const content = btoa(String.fromCharCode(...ciphertext)); // base64

    const sk = generateSecretKey();
    const event = finalizeEvent({
      kind: 1337,  // custom kind: CJP encrypted signup
      created_at: Math.floor(Date.now() / 1000),
      tags: [['t', 'cjp-signup']],
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
    setStatus(form, 'err', 'Proof-of-work not yet complete. Please wait a moment.');
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

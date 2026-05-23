// Public Nostr relays. Spread across jurisdictions for censorship resistance.
// TODO: Replace esm.sh import with a local bundle at /js/nostr-tools.bundle.js
export const RELAYS = [
  'wss://relay.damus.io',
  'wss://nos.lol',
  'wss://nostr.wine',
  'wss://relay.nostr.band',
  'wss://relay.snort.social',
  'wss://offchain.pub',
  'wss://nostr.fmt.wiz.biz',
  'wss://relay.nostr.info',
  'wss://nostr-pub.wellorder.net',
  'wss://relay.current.fyi',
  'wss://nostr.oxtr.dev',
  'wss://nostr.bitcoiner.social',
];

// Party member age public keys for sign-up encryption.
// Add one age1... key per line for each trusted party member.
// Any single key holder can independently decrypt submissions.
// Generate a key pair with: age-keygen
// Add only the public key here (from stdout of age-keygen).
export const PARTY_AGE_KEYS = [
  // 'age1REPLACE_WITH_REAL_KEY',
];

// Tag used for public demand/petition events
export const DEMAND_TAG = 'cjp-demand';

// Tag used for mirror heartbeat events
export const MIRROR_TAG = 'cjp-mirrors';

// Tag used for update notifications
export const UPDATE_TAG = 'cjp-update';

// Captcha sitekeys. Turnstile is primary (free, unlimited).
// mCaptcha is self-hosted fallback shown when Turnstile fails to load (Tor, CDN blocks).
export const TURNSTILE_SITEKEY = 'REPLACE_WITH_TURNSTILE_SITEKEY';
export const MCAPTCHA_SITEKEY  = 'REPLACE_WITH_MCAPTCHA_SITEKEY';
export const MCAPTCHA_URL      = 'REPLACE_WITH_MCAPTCHA_URL';

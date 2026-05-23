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

// Party's Nostr public key (hex) — sign-up DMs are encrypted to this key.
// Replace with the actual party key after running: publisher keygen
export const PARTY_PUBKEY = 'REPLACE_WITH_PARTY_PUBKEY_HEX';

// Tag used for public demand/petition events
export const DEMAND_TAG = 'cjp-demand';

// Tag used for mirror heartbeat events
export const MIRROR_TAG = 'cjp-mirrors';

// Tag used for update notifications
export const UPDATE_TAG = 'cjp-update';

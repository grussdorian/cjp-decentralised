#!/usr/bin/env node
// Uploads dist/ to Pinata and prints the root IPFS CID to stdout.
// Usage: PINATA_JWT=<jwt> node scripts/pinata-upload.mjs [dist-dir]
import { readFileSync, readdirSync } from 'fs';
import { join, relative } from 'path';

const JWT = process.env.PINATA_JWT;
if (!JWT) { console.error('PINATA_JWT not set'); process.exit(1); }

const distDir = process.argv[2] ?? 'dist';

function walk(dir) {
  return readdirSync(dir, { withFileTypes: true }).flatMap(e => {
    const full = join(dir, e.name);
    return e.isDirectory() ? walk(full) : [full];
  });
}

const form = new FormData();
for (const file of walk(distDir)) {
  const rel = relative(distDir, file);
  form.append('file', new Blob([readFileSync(file)]), rel);
}
form.append('pinataMetadata', JSON.stringify({ name: 'cjp-site' }));
form.append('pinataOptions', JSON.stringify({ wrapWithDirectory: false }));

const res = await fetch('https://api.pinata.cloud/pinning/pinFileToIPFS', {
  method: 'POST',
  headers: { Authorization: `Bearer ${JWT}` },
  body: form,
});

const json = await res.json();
if (!json.IpfsHash) { console.error(JSON.stringify(json)); process.exit(1); }
console.log(json.IpfsHash);

#!/usr/bin/env node
// Copies packages/site/ → dist/, then generates dist/integrity.json with
// SHA-256 hashes of every HTML, JS and CSS file.
//
// The integrity.json becomes part of the IPFS directory — so the party's
// Ed25519 signature over the CID implicitly signs every file hash too.
// verify.js fetches integrity.json via the signed IPFS CID (content-addressed,
// tamper-proof) and checks the current page against it.

const fs     = require('fs');
const path   = require('path');
const crypto = require('crypto');

const ROOT = path.join(__dirname, '..');
const SRC  = path.join(ROOT, 'packages/site');
const DIST = path.join(ROOT, 'dist');

function copyTree(src, dst) {
  fs.mkdirSync(dst, { recursive: true });
  for (const entry of fs.readdirSync(src, { withFileTypes: true })) {
    if (entry.name === '.gitkeep') continue;
    const s = path.join(src, entry.name);
    const d = path.join(dst, entry.name);
    if (entry.isDirectory()) copyTree(s, d);
    else fs.copyFileSync(s, d);
  }
}

// Walk dist/, hash every .html/.js/.css file, return { "rel/path": "hexhash" }
function buildIntegrity(dir, base) {
  base = base || dir;
  const out = {};
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      Object.assign(out, buildIntegrity(full, base));
    } else if (/\.(html|js|css)$/.test(entry.name)) {
      const rel = path.relative(base, full).replace(/\\/g, '/');
      let content = fs.readFileSync(full);
      if (entry.name.endsWith('.html')) {
        // Strip <script> tags before hashing — mirrors verify.js behaviour.
        // CDN providers inject scripts into HTML; we hash only document content.
        content = Buffer.from(
          content.toString('utf8').replace(/<script\b[^>]*>[\s\S]*?<\/script>/gi, ''),
          'utf8'
        );
      }
      const hash = crypto.createHash('sha256').update(content).digest('hex');
      out[rel] = hash;
    }
  }
  return out;
}

if (fs.existsSync(DIST)) fs.rmSync(DIST, { recursive: true });
copyTree(SRC, DIST);

const files = buildIntegrity(DIST);
fs.writeFileSync(
  path.join(DIST, 'integrity.json'),
  JSON.stringify({ generated: new Date().toISOString(), files }, null, 2)
);
console.log(`integrity.json: ${Object.keys(files).length} files hashed`);
console.log(`Built → ${DIST}`);

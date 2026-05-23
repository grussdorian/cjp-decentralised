#!/usr/bin/env node
// Copies packages/site/ → dist/, excluding .gitkeep files.
// Run: node scripts/build.js

const fs = require('fs');
const path = require('path');

const SRC = path.join(__dirname, '../packages/site');
const DIST = path.join(__dirname, '../dist');

function copy(src, dst) {
  fs.mkdirSync(dst, { recursive: true });
  for (const entry of fs.readdirSync(src, { withFileTypes: true })) {
    if (entry.name === '.gitkeep') continue;
    const s = path.join(src, entry.name);
    const d = path.join(dst, entry.name);
    if (entry.isDirectory()) {
      copy(s, d);
    } else {
      fs.copyFileSync(s, d);
    }
  }
}

if (fs.existsSync(DIST)) fs.rmSync(DIST, { recursive: true });
copy(SRC, DIST);
console.log(`Built → ${DIST}`);

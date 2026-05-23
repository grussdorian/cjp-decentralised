#!/usr/bin/env node
// Renders site templates for each language into dist/
// Usage: node scripts/build.js

const fs = require('fs');
const path = require('path');

const SITE_DIR = path.join(__dirname, '../packages/site');
const CONTENT_DIR = path.join(__dirname, '../content');
const DIST_DIR = path.join(__dirname, '../dist');

const LANGUAGES = ['en', 'hi', 'ta', 'te', 'bn'];
const PAGES = ['index', 'join', 'demand', 'mirror'];

function render(template, strings) {
  return template.replace(/\{\{(\w+(?:\.\w+)*)\}\}/g, (match, key) => {
    const parts = key.split('.');
    let val = strings;
    for (const part of parts) {
      if (val == null) return match;
      val = val[part];
    }
    return val != null ? val : match;
  });
}

function build() {
  if (fs.existsSync(DIST_DIR)) fs.rmSync(DIST_DIR, { recursive: true });

  for (const lang of LANGUAGES) {
    const outDir = lang === 'en' ? DIST_DIR : path.join(DIST_DIR, lang);
    fs.mkdirSync(outDir, { recursive: true });

    const stringsPath = path.join(CONTENT_DIR, 'translations', `${lang}.json`);
    if (!fs.existsSync(stringsPath)) {
      console.warn(`Missing translation: ${lang}.json — skipping`);
      continue;
    }
    const strings = JSON.parse(fs.readFileSync(stringsPath, 'utf8'));
    strings.lang = lang;
    strings.languages = LANGUAGES;

    for (const page of PAGES) {
      const tplPath = path.join(SITE_DIR, 'templates', `${page}.html`);
      if (!fs.existsSync(tplPath)) {
        console.warn(`Missing template: ${page}.html — skipping`);
        continue;
      }
      const tpl = fs.readFileSync(tplPath, 'utf8');
      const out = render(tpl, strings);
      fs.writeFileSync(path.join(outDir, `${page === 'index' ? 'index' : page}.html`), out);
    }

    // Copy static assets on first pass only
    if (lang === 'en') {
      for (const dir of ['js', 'css']) {
        const src = path.join(SITE_DIR, dir);
        const dst = path.join(DIST_DIR, dir);
        if (fs.existsSync(src)) fs.cpSync(src, dst, { recursive: true });
      }
    }
  }

  console.log(`Built site for languages: ${LANGUAGES.join(', ')}`);
  console.log(`Output: ${DIST_DIR}`);
}

build();

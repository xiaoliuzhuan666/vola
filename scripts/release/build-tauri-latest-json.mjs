#!/usr/bin/env node

import fs from 'node:fs';
import path from 'node:path';

function parseArgs(argv) {
  const args = {};
  for (let i = 0; i < argv.length; i += 1) {
    const token = argv[i];
    if (!token.startsWith('--')) continue;
    const key = token.slice(2);
    const next = argv[i + 1];
    if (!next || next.startsWith('--')) {
      args[key] = 'true';
      continue;
    }
    args[key] = next;
    i += 1;
  }
  return args;
}

function required(args, key) {
  const value = args[key];
  if (!value) {
    throw new Error(`Missing --${key}`);
  }
  return value;
}

function walkFiles(dir) {
  const entries = fs.readdirSync(dir, { withFileTypes: true });
  const files = [];
  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      files.push(...walkFiles(fullPath));
    } else if (entry.isFile()) {
      files.push(fullPath);
    }
  }
  return files;
}

function normalizeVersion(raw) {
  return raw.trim().replace(/^v/, '');
}

function normalizePubDate(raw) {
  if (!raw) return new Date().toISOString();
  const parsed = Date.parse(raw);
  if (Number.isNaN(parsed)) {
    throw new Error(`Invalid --published-at: ${raw}`);
  }
  return new Date(parsed).toISOString();
}

function releaseUrl(repo, version, fileName) {
  return `https://github.com/${repo}/releases/download/v${version}/${encodeURIComponent(fileName)}`;
}

function readSignature(signatureByAsset, assetName) {
  const signaturePath = signatureByAsset.get(assetName);
  if (!signaturePath) return null;
  return fs.readFileSync(signaturePath, 'utf8').trim();
}

function addPlatform(platforms, key, assetPath, signatureByAsset, repo, version) {
  const assetName = path.basename(assetPath);
  const signature = readSignature(signatureByAsset, assetName);
  if (!signature) return false;
  platforms[key] = {
    signature,
    url: releaseUrl(repo, version, assetName),
  };
  return true;
}

function classifyAsset(fileName) {
  const lower = fileName.toLowerCase();

  if (lower.endsWith('.app.tar.gz')) {
    if (lower.includes('aarch64') || lower.includes('arm64')) return ['darwin-aarch64', 'darwin-aarch64-app'];
    if (lower.includes('x64') || lower.includes('x86_64') || lower.includes('amd64')) return ['darwin-x86_64', 'darwin-x86_64-app'];
    if (lower.includes('universal')) return ['darwin-aarch64', 'darwin-x86_64', 'darwin-universal'];
  }

  if (lower.endsWith('.msi')) {
    if (lower.includes('arm64') || lower.includes('aarch64')) return ['windows-aarch64-msi'];
    return ['windows-x86_64-msi'];
  }

  if (lower.endsWith('-setup.exe') || lower.endsWith('setup.exe')) {
    if (lower.includes('arm64') || lower.includes('aarch64')) return ['windows-aarch64', 'windows-aarch64-nsis'];
    return ['windows-x86_64', 'windows-x86_64-nsis'];
  }

  if (lower.endsWith('.appimage')) {
    if (lower.includes('aarch64') || lower.includes('arm64')) return ['linux-aarch64', 'linux-aarch64-appimage'];
    if (lower.includes('amd64') || lower.includes('x86_64') || lower.includes('x64')) return ['linux-x86_64', 'linux-x86_64-appimage'];
  }

  if (lower.endsWith('.deb')) {
    if (lower.includes('arm64') || lower.includes('aarch64')) return ['linux-aarch64-deb'];
    if (lower.includes('amd64') || lower.includes('x86_64') || lower.includes('x64')) return ['linux-x86_64-deb'];
  }

  if (lower.endsWith('.rpm')) {
    if (lower.includes('aarch64') || lower.includes('arm64')) return ['linux-aarch64-rpm'];
    if (lower.includes('x86_64') || lower.includes('amd64') || lower.includes('x64')) return ['linux-x86_64-rpm'];
  }

  return [];
}

function main() {
  const args = parseArgs(process.argv.slice(2));
  const version = normalizeVersion(required(args, 'version'));
  const repo = required(args, 'repo');
  const assetsDir = required(args, 'assets-dir');
  const output = args.output || 'latest.json';
  const notes = args.notes || `Vola v${version}`;
  const pubDate = normalizePubDate(args['published-at']);

  if (!fs.existsSync(assetsDir) || !fs.statSync(assetsDir).isDirectory()) {
    throw new Error(`Assets directory not found: ${assetsDir}`);
  }

  const files = walkFiles(assetsDir);
  const signatures = new Map();
  for (const filePath of files) {
    const name = path.basename(filePath);
    if (name.endsWith('.sig')) {
      signatures.set(name.slice(0, -4), filePath);
    }
  }

  const platforms = {};
  for (const filePath of files) {
    const name = path.basename(filePath);
    if (name.endsWith('.sig') || name === 'latest.json' || name === 'SHA256SUMS.txt') continue;
    const keys = classifyAsset(name);
    for (const key of keys) {
      addPlatform(platforms, key, filePath, signatures, repo, version);
    }
  }

  if (Object.keys(platforms).length === 0) {
    throw new Error('No signed updater artifacts were found. Check createUpdaterArtifacts and signing secrets.');
  }

  const latest = {
    version,
    notes,
    pub_date: pubDate,
    platforms,
  };

  fs.writeFileSync(output, `${JSON.stringify(latest, null, 2)}\n`);
  console.log(`Wrote ${output}`);
  console.log(Object.keys(platforms).sort().join('\n'));
}

try {
  main();
} catch (error) {
  console.error(`[build-tauri-latest-json] ${error.message}`);
  process.exit(1);
}

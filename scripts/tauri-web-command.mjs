#!/usr/bin/env node

import { spawnSync } from 'node:child_process'
import { existsSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const command = process.argv[2]
const allowedCommands = new Set(['build', 'dev'])

if (!allowedCommands.has(command)) {
  console.error('Usage: node scripts/tauri-web-command.mjs <build|dev>')
  process.exit(1)
}

const scriptDir = dirname(fileURLToPath(import.meta.url))
const candidates = [
  resolve(process.cwd(), 'web'),
  resolve(process.cwd(), '..', 'web'),
  resolve(scriptDir, '..', 'web'),
  resolve(scriptDir, '..', '..', 'web'),
]

const webDir = candidates.find((candidate) => existsSync(resolve(candidate, 'package.json')))

if (!webDir) {
  console.error('Could not find web/package.json from current working directory.')
  process.exit(1)
}

const result = spawnSync('npm', ['run', command], {
  cwd: webDir,
  shell: process.platform === 'win32',
  stdio: 'inherit',
})

if (result.error) {
  console.error(result.error.message)
  process.exit(1)
}

process.exit(result.status ?? 1)

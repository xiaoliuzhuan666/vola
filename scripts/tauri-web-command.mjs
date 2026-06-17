#!/usr/bin/env node

import { spawnSync } from 'node:child_process'
import { copyFileSync, existsSync, mkdirSync, rmSync } from 'node:fs'
import { execFileSync } from 'node:child_process'
import { basename, dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const command = process.argv[2]
const allowedCommands = new Set(['build', 'dev'])

if (!allowedCommands.has(command)) {
  console.error('Usage: node scripts/tauri-web-command.mjs <build|dev>')
  process.exit(1)
}

const scriptDir = dirname(fileURLToPath(import.meta.url))
const repoRoot = resolve(scriptDir, '..')
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

if (result.status !== 0) {
  process.exit(result.status ?? 1)
}

if (command !== 'build') {
  process.exit(0)
}

const rustTripleByPlatformArch = {
  'darwin:x64': 'x86_64-apple-darwin',
  'darwin:arm64': 'aarch64-apple-darwin',
  'win32:x64': 'x86_64-pc-windows-msvc',
  'win32:arm64': 'aarch64-pc-windows-msvc',
  'linux:x64': 'x86_64-unknown-linux-gnu',
  'linux:arm64': 'aarch64-unknown-linux-gnu',
}

function detectRustTriple() {
  const envTriple = process.env.TAURI_ENV_TARGET_TRIPLE || process.env.CARGO_BUILD_TARGET
  if (envTriple?.trim()) return envTriple.trim()

  try {
    const direct = execFileSync('rustc', ['--print', 'host-tuple'], {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'ignore'],
    }).trim()
    if (direct) return direct
  } catch {
    // Older rustc builds do not support --print host-tuple.
  }

  try {
    const verbose = execFileSync('rustc', ['-Vv'], {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'ignore'],
    })
    const hostLine = verbose.split(/\r?\n/).find((line) => line.startsWith('host:'))
    const parsed = hostLine?.replace('host:', '').trim()
    if (parsed) return parsed
  } catch {
    // Fall through to Node platform mapping.
  }

  return rustTripleByPlatformArch[`${process.platform}:${process.arch}`]
}

const goEnvByRustTriple = {
  'aarch64-apple-darwin': { GOOS: 'darwin', GOARCH: 'arm64' },
  'x86_64-apple-darwin': { GOOS: 'darwin', GOARCH: 'amd64' },
  'aarch64-pc-windows-msvc': { GOOS: 'windows', GOARCH: 'arm64' },
  'x86_64-pc-windows-msvc': { GOOS: 'windows', GOARCH: 'amd64' },
  'aarch64-pc-windows-gnu': { GOOS: 'windows', GOARCH: 'arm64' },
  'x86_64-pc-windows-gnu': { GOOS: 'windows', GOARCH: 'amd64' },
  'aarch64-unknown-linux-gnu': { GOOS: 'linux', GOARCH: 'arm64' },
  'x86_64-unknown-linux-gnu': { GOOS: 'linux', GOARCH: 'amd64' },
  'aarch64-unknown-linux-musl': { GOOS: 'linux', GOARCH: 'arm64' },
  'x86_64-unknown-linux-musl': { GOOS: 'linux', GOARCH: 'amd64' },
}

const rustTriple = detectRustTriple()
const goTargetEnv = goEnvByRustTriple[rustTriple]
if (!goTargetEnv) {
  console.error(`Unsupported Tauri desktop backend target: ${rustTriple || `${process.platform}/${process.arch}`}`)
  process.exit(1)
}

const outputDir = resolve(repoRoot, 'src-tauri', 'bin')
const outputPath = resolve(outputDir, 'vola')
const buildPath = resolve(outputDir, `.vola-${rustTriple}.build`)
mkdirSync(outputDir, { recursive: true })
rmSync(buildPath, { force: true })

const backendResult = spawnSync('go', ['build', '-o', buildPath, './cmd/vola/'], {
  cwd: repoRoot,
  env: {
    ...process.env,
    ...goTargetEnv,
  },
  shell: process.platform === 'win32',
  stdio: 'inherit',
})

if (backendResult.error) {
  console.error(backendResult.error.message)
  process.exit(1)
}

if (backendResult.status !== 0) {
  process.exit(backendResult.status ?? 1)
}

copyFileSync(buildPath, outputPath)
rmSync(buildPath, { force: true })
console.log(`Prepared Tauri desktop backend ${basename(outputPath)} for ${rustTriple}`)

process.exit(0)

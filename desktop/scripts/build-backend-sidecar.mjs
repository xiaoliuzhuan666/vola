import { copyFileSync, cpSync, existsSync, mkdirSync, rmSync } from 'node:fs'
import { basename, join, resolve } from 'node:path'
import { execFileSync, spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'

const here = fileURLToPath(new URL('.', import.meta.url))
const desktopDir = resolve(here, '..')
const repoRoot = resolve(desktopDir, '..')
const sidecarDir = join(desktopDir, 'sidecars')
mkdirSync(sidecarDir, { recursive: true })

const platform = process.platform
const arch = process.arch
const rustTripleByPlatformArch = {
  'darwin:x64': 'x86_64-apple-darwin',
  'darwin:arm64': 'aarch64-apple-darwin',
  'win32:x64': 'x86_64-pc-windows-msvc',
  'win32:arm64': 'aarch64-pc-windows-msvc',
  'linux:x64': 'x86_64-unknown-linux-gnu',
  'linux:arm64': 'aarch64-unknown-linux-gnu',
}

function detectRustTriple() {
  if (process.env.TAURI_ENV_TARGET_TRIPLE) {
    return process.env.TAURI_ENV_TARGET_TRIPLE.trim()
  }
  try {
    const direct = execFileSync('rustc', ['--print', 'host-tuple'], {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'ignore'],
    }).trim()
    if (direct) return direct
  } catch {
    // Rust < 1.84 does not support --print host-tuple.
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
  return rustTripleByPlatformArch[`${platform}:${arch}`]
}

const rustTriple = detectRustTriple()
if (!rustTriple) {
  console.error(`Unsupported desktop sidecar target: ${platform}/${arch}`)
  process.exit(1)
}

const goEnvByRustTriple = {
  'aarch64-apple-darwin': { GOOS: 'darwin', GOARCH: 'arm64' },
  'x86_64-apple-darwin': { GOOS: 'darwin', GOARCH: 'amd64' },
  'aarch64-pc-windows-msvc': { GOOS: 'windows', GOARCH: 'arm64' },
  'x86_64-pc-windows-msvc': { GOOS: 'windows', GOARCH: 'amd64' },
  'aarch64-unknown-linux-gnu': { GOOS: 'linux', GOARCH: 'arm64' },
  'x86_64-unknown-linux-gnu': { GOOS: 'linux', GOARCH: 'amd64' },
}

const goTargetEnv = goEnvByRustTriple[rustTriple]
if (!goTargetEnv) {
  console.error(`Unsupported desktop sidecar target triple: ${rustTriple}`)
  process.exit(1)
}

const sidecarName = platform === 'win32'
  ? `vola-${rustTriple}.exe`
  : `vola-${rustTriple}`

const sidecarPath = join(sidecarDir, sidecarName)
const buildPath = join(sidecarDir, `.${sidecarName}.build`)
const webDist = join(repoRoot, 'web', 'dist')
const embeddedWebDist = join(repoRoot, 'internal', 'web', 'dist')

rmSync(buildPath, { force: true })

if (existsSync(webDist)) {
  rmSync(embeddedWebDist, { recursive: true, force: true })
  cpSync(webDist, embeddedWebDist, { recursive: true })
}

const built = spawnSync('go', ['build', '-o', buildPath, './cmd/vola'], {
  cwd: repoRoot,
  env: {
    ...process.env,
    ...goTargetEnv,
  },
  stdio: 'inherit',
})
if (built.status !== 0) {
  process.exit(built.status || 1)
}

copyFileSync(buildPath, sidecarPath)
rmSync(buildPath, { force: true })
console.log(`Prepared desktop sidecar ${basename(sidecarName)}`)

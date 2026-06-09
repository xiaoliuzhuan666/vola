import { execFileSync } from 'node:child_process'

const args = new Set(process.argv.slice(2))
const requireNew = args.has('--require-new')

const repoRoot = new URL('../..', import.meta.url).pathname.replace(/\/$/, '')
const oldAppNeedle = `${repoRoot}/src-tauri/target`
const newAppNeedle = `${repoRoot}/desktop/target/release/bundle/macos/Vola.app`
const newAppExecutable = `${newAppNeedle}/Contents/MacOS/vola-desktop`
const newSidecarExecutable = `${newAppNeedle}/Contents/MacOS/vola`

let output = ''
try {
  output = execFileSync('ps', ['-ax', '-o', 'pid=,command='], { encoding: 'utf8' })
} catch (err) {
  console.error(`Failed to inspect desktop processes: ${err.message}`)
  process.exit(1)
}

const lines = output.split(/\r?\n/).filter(Boolean)
const processes = lines.map((line) => {
  const match = line.trim().match(/^(\d+)\s+(.+)$/)
  return match ? { pid: match[1], command: match[2], line: line.trim() } : null
}).filter(Boolean)

const oldVola = processes.filter(({ command }) => (
  command.startsWith(oldAppNeedle) &&
  (
    command.includes('/vola.app/') ||
    command.includes('/vola.app ') ||
    command.endsWith('/vola.app') ||
    command.includes('/Contents/MacOS/app') ||
    command.includes('/Contents/MacOS/vola')
  )
))
const newAppProcesses = processes.filter(({ command }) => (
  command === newAppExecutable ||
  command.startsWith(`${newAppExecutable} `)
))
const newSidecarProcesses = processes.filter(({ command }) => (
  command === newSidecarExecutable ||
  command.startsWith(`${newSidecarExecutable} server `)
))
const newVola = [...newAppProcesses, ...newSidecarProcesses]

if (oldVola.length > 0) {
  console.error('Old src-tauri Vola process is still running. Quit it before desktop verification.')
  for (const proc of oldVola) console.error(`  ${proc.line}`)
  process.exit(1)
}

if (requireNew) {
  if (newAppProcesses.length !== 1 || newSidecarProcesses.length !== 1) {
    console.error(`Expected exactly one new desktop app and one sidecar from: ${newAppNeedle}`)
    if (newVola.length > 0) {
      console.error('Detected new Vola processes:')
      for (const proc of newVola) console.error(`  ${proc.line}`)
    }
    process.exit(1)
  }
}

if (newVola.length > 0) {
  console.log('New desktop Vola process:')
  for (const proc of newVola) console.log(`  ${proc.line}`)
} else {
  console.log('No Vola desktop process is running.')
}

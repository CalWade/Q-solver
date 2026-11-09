import { spawn, execFile } from 'node:child_process'
import { promisify } from 'node:util'
import { ensureWails } from './toolchain.mjs'
import { prepareFramework } from './framework.mjs'
import { mkdir, access, readFile, writeFile, chmod } from 'node:fs/promises'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const frontend = resolve(root, 'frontend')
const npm = process.platform === 'win32' ? 'npm.cmd' : 'npm'
const cancellation = new AbortController()
const capture = async (bin, args) => (await promisify(execFile)(bin, args, { cwd: root, timeout: 60000, signal: cancellation.signal })).stdout
const wails = () => ensureWails({ directory: resolve(root, '.local/bin'), capture, run })
const command = process.argv[2] || 'dev'
const children = new Set()
let stopping = false
let stopRequested = false

function publishStatus(message) {
  if (process.connected) process.send(message, error => { if (error) requestStop() })
}

function launch(bin, args, cwd = root, env = {}) {
  if (stopping) throw new Error('Task is stopping')
  const child = spawn(bin, args, { cwd, env: { ...process.env, ...env }, stdio: 'inherit', detached: process.platform !== 'win32', shell: process.platform === 'win32' && bin.endsWith('.cmd') })
  children.add(child)
  child.once('exit', () => children.delete(child))
  return child
}
function run(bin, args, cwd = root, env) {
  return new Promise((resolve, reject) => {
    const child = launch(bin, args, cwd, env)
    child.once('error', reject)
    child.once('exit', (code, signal) => code === 0 ? resolve() : reject(new Error(`${bin} exited with ${code ?? signal}`)))
  })
}
async function shutdown() {
  if (stopping) return
  stopping = true
  for (const child of children) {
    if (!child.pid) continue
    try {
      if (process.platform === 'win32') spawn('taskkill', ['/pid', String(child.pid), '/t', '/f'], { stdio: 'ignore' })
      else process.kill(-child.pid, 'SIGTERM')
    } catch (error) { if (error.code !== 'ESRCH') console.error(error.message) }
  }
  if (process.connected) process.disconnect()
}
function requestStop() {
  stopRequested = true
  cancellation.abort()
  shutdown()
}
process.on('SIGINT', requestStop)
process.on('SIGTERM', requestStop)
process.on('message', message => {
  if (message?.type === 'stop') requestStop()
})
process.once('disconnect', () => {
  if (!stopping) requestStop()
})
publishStatus({ type: 'control-ready' })

async function normalizeBindings() {
  const models = resolve(frontend, 'wailsjs/go/models.ts')
  await writeFile(models, (await readFile(models, 'utf8')).replace(/[\t ]+$/gm, ''))
  for (const file of ['package.json', 'runtime.d.ts', 'runtime.js']) await chmod(resolve(frontend, 'wailsjs/runtime', file), 0o644)
}

async function dev() {
  await access(resolve(frontend, 'node_modules/vite'))
  await mkdir(resolve(root, '.local/bin'), { recursive: true })
  const executable = resolve(root, '.local/bin', process.platform === 'win32' ? 'harness.exe' : 'harness')
  publishStatus({ type: 'building' })
  await run('go', ['build', '-o', executable, './cmd/harness'])
  const backend = launch(executable, [])
  const ui = launch(npm, ['run', 'dev', '--', '--mode', 'harness'], frontend)
  for (const child of [backend, ui]) child.once('exit', () => {
    if (!stopping) {
      process.exitCode = 1
      shutdown()
    }
  })
  publishStatus({ type: 'waiting-ready', processes: [backend.pid, ui.pid] })
  for (let i = 0; i < 100 && !stopping; i++) {
    try {
      const responses = await Promise.all(['http://127.0.0.1:4317/health', 'http://127.0.0.1:4173'].map(url => fetch(url, { signal: AbortSignal.timeout(500) })))
      if (responses.every(response => response.ok)) {
        publishStatus({ type: 'ready', processes: [backend.pid, ui.pid] })
        console.log('Ready: http://127.0.0.1:4173 — isolated data, simulated OS ports. Ctrl+C stops both services.')
        return
      }
    } catch {}
    await new Promise(resolve => setTimeout(resolve, 200))
  }
  if (!stopRequested) throw new Error('Development services did not become ready; see the output above.')
}

try {
  await prepareFramework({ root, capture, run })
  switch (command) {
    case 'prepare': console.log('Pinned native framework is ready.'); break
    case 'harness': await run('go', ['run', './cmd/harness']); break
    case 'dev': await dev(); break
    case 'doctor':
      await run('go', ['version']); await run(npm, ['--version']);
      await access(resolve(frontend, 'node_modules/@playwright/test/package.json'))
      console.log('Toolchain and project test dependency found. Browser launch is verified by test:e2e.'); break
    case 'check':
      await run(npm, ['test'], frontend)
      await run(npm, ['run', 'build'], frontend)
      await run('go', ['test', '-race', './...'])
      await run(process.execPath, ['--test', 'scripts/framework.test.mjs', 'scripts/toolchain.test.mjs', 'scripts/compatibility.test.mjs', 'scripts/focus-evidence.test.mjs', 'scripts/keyboard-evidence.test.mjs', 'scripts/run.test.mjs'])
      await run(process.execPath, ['scripts/compatibility.mjs'])
      await run(npm, ['run', 'test:e2e'], frontend)
      break
    case 'bindings': await run(await wails(), ['generate', 'module']); await normalizeBindings(); break
    case 'build': await run(await wails(), ['build', '-tags', 'prod']); await normalizeBindings(); break
    case 'panel-probe':
    case 'host-probe':
    case 'capture-probe':
    case 'stream-capture-probe':
    case 'live-capture-probe':
    case 'display-capture-probe':
    case 'keyboard-probe':
    case 'focus-probe': {
      if (process.platform !== 'darwin') throw new Error('The native panel probe currently requires macOS.')
      await mkdir(resolve(root, '.local/bin'), { recursive: true })
      const target = ['panel-probe', 'capture-probe', 'stream-capture-probe', 'live-capture-probe', 'display-capture-probe'].includes(command) ? 'panelprobe' : 'hostprobe'
      const executable = resolve(root, '.local/bin', target)
      await run('go', ['build', '-tags', 'production', '-o', executable, `./cmd/${target}`])
      if (command === 'focus-probe') await run(npm, ['run', 'test:native'], frontend)
      else if (command === 'keyboard-probe') await run(npm, ['run', 'test:keyboard'], frontend)
      else if (command === 'stream-capture-probe') await run(executable, ['-capture-stream', '-output', '.local/reports/stream-capture-probe.json'])
      else if (command === 'live-capture-probe') await run(executable, ['-capture-live', '-output', '.local/reports/live-capture-probe.json'])
      else if (command === 'display-capture-probe') await run(executable, ['-capture-live', '-display-window', '-output', '.local/reports/display-capture-probe.json'])
      else await run(executable, command === 'capture-probe' ? ['-capture', '-output', '.local/reports/capture-probe.json'] : [])
      break
    }
    default: throw new Error('Usage: node scripts/run.mjs [prepare|dev|harness|doctor|check|bindings|build|panel-probe|host-probe|focus-probe|capture-probe|stream-capture-probe|live-capture-probe|display-capture-probe|keyboard-probe]')
  }
} catch (error) {
  if (!stopRequested) {
    console.error(error.message)
    process.exitCode = 1
  }
  await shutdown()
} finally {
  if (command !== 'dev' && process.connected) process.disconnect()
}

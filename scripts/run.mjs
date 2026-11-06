import { spawn } from 'node:child_process'
import { mkdir, access, readFile, writeFile, chmod } from 'node:fs/promises'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const frontend = resolve(root, 'frontend')
const npm = process.platform === 'win32' ? 'npm.cmd' : 'npm'
const wails = ['run', 'github.com/wailsapp/wails/v2/cmd/wails@v2.12.0']
const command = process.argv[2] || 'dev'
const children = new Set()
let stopping = false

function launch(bin, args, cwd = root, env = {}) {
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
}
process.on('SIGINT', () => { shutdown() })
process.on('SIGTERM', () => { shutdown() })

async function normalizeBindings() {
  const models = resolve(frontend, 'wailsjs/go/models.ts')
  await writeFile(models, (await readFile(models, 'utf8')).replace(/[\t ]+$/gm, ''))
  for (const file of ['package.json', 'runtime.d.ts', 'runtime.js']) await chmod(resolve(frontend, 'wailsjs/runtime', file), 0o644)
}

async function dev() {
  await access(resolve(frontend, 'node_modules/vite'))
  await mkdir(resolve(root, '.local/bin'), { recursive: true })
  const executable = resolve(root, '.local/bin', process.platform === 'win32' ? 'harness.exe' : 'harness')
  await run('go', ['build', '-o', executable, './cmd/harness'])
  const backend = launch(executable, [])
  const ui = launch(npm, ['run', 'dev', '--', '--mode', 'harness'], frontend)
  for (const child of [backend, ui]) child.once('exit', () => shutdown())
  for (let i = 0; i < 100 && !stopping; i++) {
    try {
      const responses = await Promise.all(['http://127.0.0.1:4317/health', 'http://127.0.0.1:4173'].map(url => fetch(url, { signal: AbortSignal.timeout(500) })))
      if (responses.every(response => response.ok)) {
        console.log('Ready: http://127.0.0.1:4173 — isolated data, simulated OS ports. Ctrl+C stops both services.')
        return
      }
    } catch {}
    await new Promise(resolve => setTimeout(resolve, 200))
  }
  throw new Error('Development services did not become ready; see the output above.')
}

try {
  switch (command) {
    case 'dev': await dev(); break
    case 'doctor':
      await run('go', ['version']); await run(npm, ['--version']);
      await access(resolve(frontend, 'node_modules/@playwright/test/package.json'))
      console.log('Toolchain and project test dependency found. Browser launch is verified by test:e2e.'); break
    case 'check':
      await run(npm, ['test'], frontend)
      await run(npm, ['run', 'build'], frontend)
      await run('go', ['test', '-race', './...'])
      await run(process.execPath, ['--test', 'scripts/run.test.mjs'])
      await run(npm, ['run', 'test:e2e'], frontend)
      break
    case 'bindings': await run('go', [...wails, 'generate', 'module']); await normalizeBindings(); break
    case 'build': await run('go', [...wails, 'build', '-tags', 'prod']); await normalizeBindings(); break
    default: throw new Error('Usage: node scripts/run.mjs [dev|doctor|check|bindings|build]')
  }
} catch (error) { console.error(error.message); await shutdown(); process.exitCode = 1 }

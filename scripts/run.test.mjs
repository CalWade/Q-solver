import test from 'node:test'
import assert from 'node:assert/strict'
import { spawn } from 'node:child_process'
import { once } from 'node:events'
import { setTimeout as delay } from 'node:timers/promises'

const urls = ['http://127.0.0.1:4317/health', 'http://127.0.0.1:4173']
async function reachable(url) { try { return (await fetch(url, { signal: AbortSignal.timeout(400) })).ok } catch { return false } }

test('autonomous startup becomes healthy and shutdown releases both services', { timeout: 45000 }, async t => {
  for (const url of urls) assert.equal(await reachable(url), false, `${url} is already occupied; leave that process untouched`)
  const child = spawn(process.execPath, ['scripts/run.mjs', 'dev'], { stdio: ['ignore', 'pipe', 'pipe'] })
  const exit = once(child, 'exit')
  let output = ''
  child.stdout.on('data', chunk => { output += chunk })
  child.stderr.on('data', chunk => { output += chunk })
  t.after(async () => { if (child.exitCode === null) { child.kill('SIGTERM'); await exit } })
  let ready = false
  for (let attempt = 0; attempt < 120; attempt++) {
    if (child.exitCode !== null) assert.fail(output)
    if ((await Promise.all(urls.map(reachable))).every(Boolean)) { ready = true; break }
    await delay(200)
  }
  assert.ok(ready, output)
  child.kill('SIGTERM')
  const [code] = await exit
  assert.equal(code, 0, output)
  for (const url of urls) assert.equal(await reachable(url), false, `service leaked: ${url}`)
})

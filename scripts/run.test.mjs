import test from 'node:test'
import assert from 'node:assert/strict'
import { fork } from 'node:child_process'
import { once } from 'node:events'
import { setTimeout as delay } from 'node:timers/promises'

const urls = ['http://127.0.0.1:4317/health', 'http://127.0.0.1:4173']
async function reachable(url) { try { return (await fetch(url, { signal: AbortSignal.timeout(400) })).ok } catch { return false } }

test('autonomous startup becomes healthy and shutdown releases both services', { timeout: 45000 }, async t => {
  for (const url of urls) assert.equal(await reachable(url), false, `${url} is already occupied; leave that process untouched`)
  const child = fork('scripts/run.mjs', ['dev'], { silent: true })
  const exit = once(child, 'exit')
  let output = ''
  child.stdout.on('data', chunk => { output += chunk })
  child.stderr.on('data', chunk => { output += chunk })
  t.after(async () => { if (child.exitCode === null && child.connected) { child.send({ type: 'stop' }); await exit } })
  let ready = false
  for (let attempt = 0; attempt < 120; attempt++) {
    if (child.exitCode !== null) assert.fail(output)
    if ((await Promise.all(urls.map(reachable))).every(Boolean)) { ready = true; break }
    await delay(200)
  }
  assert.ok(ready, output)
  child.send({ type: 'stop' })
  const [code] = await exit
  assert.equal(code, 0, output)
  for (const url of urls) assert.equal(await reachable(url), false, `service leaked: ${url}`)
})

test('a stop request during service readiness exits successfully and releases its listeners', { timeout: 45000 }, async t => {
  for (const url of urls) assert.equal(await reachable(url), false, `${url} is already occupied`)
  const child = fork('scripts/run.mjs', ['dev'], { silent: true })
  const exit = once(child, 'exit')
  let output = ''
  child.stdout.on('data', chunk => { output += chunk })
  child.stderr.on('data', chunk => { output += chunk })
  t.after(async () => {
    if (child.exitCode === null && child.connected) {
      child.send({ type: 'stop' })
      await exit
    }
  })
  for (let attempt = 0; attempt < 120 && !output.includes('isolated harness at'); attempt++) {
    if (child.exitCode !== null) assert.fail(output)
    await delay(20)
  }
  assert.ok(output.includes('isolated harness at'), output)
  child.send({ type: 'stop' })
  const [code] = await exit
  assert.equal(code, 0, output)
  for (const url of urls) assert.equal(await reachable(url), false, `service leaked: ${url}`)
})

for (const phase of ['control-ready', 'building']) {
  test(`a stop request during ${phase} cancels startup before services are launched`, { timeout: 45000 }, async t => {
    for (const url of urls) assert.equal(await reachable(url), false, `${url} is already occupied`)
    const child = fork('scripts/run.mjs', ['dev'], { silent: true })
    const exit = once(child, 'exit')
    let output = ''
    child.stdout.on('data', chunk => { output += chunk })
    child.stderr.on('data', chunk => { output += chunk })
    child.on('message', message => {
      if (message.type === phase && child.connected) child.send({ type: 'stop' })
    })
    t.after(async () => {
      if (child.exitCode === null && child.connected) {
        child.send({ type: 'stop' })
        await exit
      }
    })
    const [code] = await exit
    assert.equal(code, 0, output)
    assert.equal(output.includes('isolated harness at'), false, output)
    for (const url of urls) assert.equal(await reachable(url), false, `service leaked: ${url}`)
  })
}

test('a disconnected controller cancels preparation and exits cleanly', { timeout: 45000 }, async t => {
  for (const url of urls) assert.equal(await reachable(url), false, `${url} is already occupied`)
  const child = fork('scripts/run.mjs', ['dev'], { silent: true })
  const exit = once(child, 'exit')
  child.stdout.resume()
  child.stderr.resume()
  child.on('message', message => {
    if (message.type === 'control-ready' && child.connected) child.disconnect()
  })
  t.after(async () => {
    if (child.exitCode === null && child.connected) {
      child.send({ type: 'stop' })
      await exit
    }
  })
  const [code] = await exit
  assert.equal(code, 0)
  for (const url of urls) assert.equal(await reachable(url), false, `service leaked: ${url}`)
})

for (const failure of ['controller disconnect', 'backend exit']) {
  test(`running services are cleaned up after ${failure}`, { timeout: 45000 }, async t => {
    for (const url of urls) assert.equal(await reachable(url), false, `${url} is already occupied`)
    const child = fork('scripts/run.mjs', ['dev'], { silent: true })
    const exit = once(child, 'exit')
    let output = ''
    child.stdout.on('data', chunk => { output += chunk })
    child.stderr.on('data', chunk => { output += chunk })
    child.on('message', message => {
      if (message.type !== 'ready') return
      if (failure === 'controller disconnect') {
        child.disconnect()
      } else {
        const backendPID = message.processes[0]
        assert.ok(Number.isInteger(backendPID) && backendPID > 0)
        process.kill(backendPID, 'SIGTERM')
      }
    })
    t.after(async () => {
      if (child.exitCode === null && child.connected) {
        child.send({ type: 'stop' })
        await exit
      }
    })
    const [code] = await exit
    assert.equal(code, failure === 'backend exit' ? 1 : 0, output)
    for (const url of urls) assert.equal(await reachable(url), false, `service leaked: ${url}`)
  })
}

import test from 'node:test'
import assert from 'node:assert/strict'
import { ensureWails, wailsModule, wailsVersion } from './toolchain.mjs'

test('cached local tool works without network or rebuilding', async () => {
  const result = await ensureWails({ directory: '/tmp/q-solver-test', exists: async () => {}, capture: () => assert.fail('unexpected download'), run: () => assert.fail('unexpected build') })
  assert.match(result, /wails-v2\.12\.0/)
})
test('first build uses the pinned module own dependency graph and project cache', async () => {
  const builds = []
  const result = await ensureWails({ directory: '/tmp/q-solver-test', platform: 'win32', exists: async () => { throw Object.assign(new Error(), { code: 'ENOENT' }) }, makeDirectory: async () => {},
    capture: async (command, args) => { assert.equal(command, 'go'); assert.deepEqual(args, ['mod', 'download', '-json', `${wailsModule}@${wailsVersion}`]); return JSON.stringify({ Path: wailsModule, Version: wailsVersion, Dir: '/module-cache/pinned' }) },
    run: async (...args) => builds.push(args) })
  assert.match(result, /\.exe$/)
  assert.deepEqual(builds, [['go', ['build', '-mod=readonly', '-o', result, './cmd/wails'], '/module-cache/pinned', { GOWORK: 'off' }]])
})
test('an unverified module location is never executed', async () => {
  await assert.rejects(ensureWails({ directory: '/tmp/q-solver-test', exists: async () => { throw Object.assign(new Error(), { code: 'ENOENT' }) }, makeDirectory: async () => {}, capture: async () => JSON.stringify({ Error: 'module unavailable' }), run: () => assert.fail('unverified build') }), /module unavailable/)
})

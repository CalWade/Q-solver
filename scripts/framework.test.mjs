import test from 'node:test'
import assert from 'node:assert/strict'
import { mkdtemp, mkdir, writeFile, readFile, readdir, chmod, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { resolve } from 'node:path'
import { createHash } from 'node:crypto'
import { execFile } from 'node:child_process'
import { promisify } from 'node:util'
import { materializeFramework, validateFrameworkManifest, verifyFrameworkModule } from './framework.mjs'

const digest = value => createHash('sha256').update(value).digest('hex')
async function fixture(t) {
  const dir = await mkdtemp(resolve(tmpdir(), 'q-solver-framework-test-'))
  t.after(() => rm(dir, { recursive: true, force: true }))
  const source = resolve(dir, 'upstream'), target = resolve(dir, 'owned-copy'), path = 'internal/frontend/desktop/darwin/Fixture.h'
  await mkdir(resolve(source, 'internal/frontend/desktop/darwin'), { recursive: true })
  await writeFile(resolve(source, path), 'before\n')
  const patchPath = resolve(dir, 'fixture.patch')
  await writeFile(patchPath, `--- a/${path}\n+++ b/${path}\n@@ -1 +1 @@\n-before\n+after\n`)
  const manifest = { module: 'github.com/wailsapp/wails/v2', version: 'v2.12.0', files: { [path]: { before: digest('before\n'), after: digest('after\n') } } }
  return { source, target, patchPath, manifest, path, run: (bin, args) => promisify(execFile)(bin, args) }
}
test('patches only the owned copy, then reuses verified output', async t => {
  const f = await fixture(t)
  await materializeFramework(f)
  assert.equal(await readFile(resolve(f.source, f.path), 'utf8'), 'before\n')
  assert.equal(await readFile(resolve(f.target, f.path), 'utf8'), 'after\n')
  await materializeFramework({ ...f, run: () => assert.fail('verified cache should not be patched again') })
})
test('upstream drift and local changes are refused without deleting data', async t => {
  const f = await fixture(t)
  await writeFile(resolve(f.source, f.path), 'unexpected upstream\n')
  await assert.rejects(materializeFramework(f), /source does not match/)
  await writeFile(resolve(f.source, f.path), 'before\n')
  await materializeFramework(f)
  await writeFile(resolve(f.target, f.path), 'local edit\n')
  await assert.rejects(materializeFramework(f), /cache was modified/)
  assert.equal(await readFile(resolve(f.target, f.path), 'utf8'), 'local edit\n')
})
test('manifest cannot target paths outside the Darwin adapter', () => {
  assert.throws(() => validateFrameworkManifest({ module: 'github.com/wailsapp/wails/v2', version: 'v2.12.0', files: { '../escape': {} } }), /path\/hash/)
})
test('a failed patch cleans the owned read-only copy and keeps upstream intact', async t => {
  const f = await fixture(t)
  const readOnly = resolve(f.source, 'internal/frontend/desktop/darwin')
  await chmod(readOnly, 0o555)
  t.after(() => chmod(readOnly, 0o755).catch(() => {}))
  await assert.rejects(materializeFramework({ ...f, run: async () => { throw new Error('patch failed') } }), /patch failed/)
  assert.equal(await readFile(resolve(f.source, f.path), 'utf8'), 'before\n')
  assert.equal((await readdir(resolve(f.target, '..'))).some(name => name.startsWith('owned-copy')), false)
  await chmod(readOnly, 0o755)
})
test('the complete upstream module stays pinned despite a local Go replacement', async () => {
  const manifest = JSON.parse(await readFile(new URL('../patches/wails-v2.12.0-panel.json', import.meta.url), 'utf8'))
  const metadata = { Path: manifest.module, Version: manifest.version, Dir: '/module-cache', Sum: manifest.moduleSum, GoModSum: manifest.goModSum }
  verifyFrameworkModule(manifest, metadata)
  for (const change of [{ Sum: 'h1:wrong' }, { GoModSum: '' }, { Path: 'unexpected/module' }, { Version: 'v2.13.0' }, { Dir: '' }, { Error: 'unavailable' }]) {
    assert.throws(() => verifyFrameworkModule(manifest, { ...metadata, ...change }))
  }
  assert.throws(() => verifyFrameworkModule({ ...manifest, moduleSum: undefined }, metadata))
})

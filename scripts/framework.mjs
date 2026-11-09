import { cp, readFile, writeFile, mkdir, mkdtemp, readdir, chmod, rename, access, rm, realpath } from 'node:fs/promises'
import { createHash } from 'node:crypto'
import { dirname, resolve, isAbsolute } from 'node:path'

const digest = data => createHash('sha256').update(data).digest('hex')
export function validateFrameworkManifest(manifest) {
  if (manifest.module !== 'github.com/wailsapp/wails/v2' || manifest.version !== 'v2.12.0' || !Object.keys(manifest.files || {}).length) throw new Error('Unsupported framework patch manifest')
  for (const [path, hashes] of Object.entries(manifest.files)) {
    if (!path.startsWith('internal/frontend/desktop/darwin/') || path.split('/').includes('..') || isAbsolute(path) || !/^[a-f0-9]{64}$/.test(hashes.before) || !/^[a-f0-9]{64}$/.test(hashes.after)) throw new Error('Invalid framework patch path/hash')
  }
}

export function verifyFrameworkModule(manifest, metadata) {
  // A local Go replacement is omitted by `go mod tidy` from go.sum, so keep
  // the complete upstream module checksums in the tracked patch manifest.
  if (!manifest.moduleSum || !manifest.goModSum || metadata.Error || metadata.Path !== manifest.module || metadata.Version !== manifest.version || !metadata.Dir || metadata.Sum !== manifest.moduleSum || metadata.GoModSum !== manifest.goModSum) {
    throw new Error(metadata.Error || 'Pinned framework module checksum/metadata mismatch')
  }
}
async function matches(directory, files, side) {
  for (const [path, hashes] of Object.entries(files)) {
    try { if (digest(await readFile(resolve(directory, path))) !== hashes[side]) return false }
    catch (error) { if (error.code === 'ENOENT') return false; throw error }
  }
  return true
}

async function writableDirectories(directory) {
  await chmod(directory, 0o755)
  for (const item of await readdir(directory, { withFileTypes: true })) {
    if (item.isDirectory()) await writableDirectories(resolve(directory, item.name))
  }
}

// Only copies verified module sources into an owned project cache. The shared
// Go module cache remains untouched. A modified cache is preserved, never erased.
export async function materializeFramework({ source, target, manifest, patchPath, run }) {
  validateFrameworkManifest(manifest)
  const patchDigest = digest(await readFile(patchPath))
  const receipt = resolve(target, '.q-solver-patch.json')
  try {
    const saved = JSON.parse(await readFile(receipt, 'utf8'))
    if (saved.patchDigest === patchDigest && await matches(target, manifest.files, 'after')) return target
    throw new Error(`Framework cache was modified or uses another patch: preserve it before rebuilding ${target}`)
  } catch (error) { if (error.code !== 'ENOENT') throw error }
  try { await access(target); throw new Error(`Unmanaged framework directory exists: ${target}`) } catch (error) { if (error.code !== 'ENOENT') throw error }
  if (!await matches(source, manifest.files, 'before')) throw new Error('Framework source does not match the pinned upstream files')
  await mkdir(dirname(target), { recursive: true })
  const stagingRoot = await mkdtemp(`${target}.staging-`)
  const staging = resolve(stagingRoot, 'module')
  try {
    await cp(source, staging, { recursive: true, errorOnExist: true, force: false })
    // Go's module directories are read-only. The owned copy must be removable
    // even if applying the patch fails halfway through.
    await writableDirectories(staging)
    for (const path of Object.keys(manifest.files)) {
      await chmod(dirname(resolve(staging, path)), 0o755)
      await chmod(resolve(staging, path), 0o644)
    }
    // git apply runs on the isolated copy; manifest paths are restricted above.
    await run('git', ['apply', '--unsafe-paths', `--directory=${(await realpath(staging)).replaceAll('\\', '/')}`, await realpath(patchPath)])
    if (!await matches(staging, manifest.files, 'after')) throw new Error('Framework patch output failed verification')
    await writeFile(resolve(staging, '.q-solver-patch.json'), JSON.stringify({ module: manifest.module, version: manifest.version, patchDigest }) + '\n')
    await rename(staging, target)
    return target
  } finally {
    await writableDirectories(stagingRoot)
    await rm(stagingRoot, { recursive: true, force: true })
  }
}

export async function prepareFramework({ root, capture, run }) {
  const patchPath = resolve(root, 'patches/wails-v2.12.0-panel.patch')
  const manifest = JSON.parse(await readFile(resolve(root, 'patches/wails-v2.12.0-panel.json'), 'utf8'))
  validateFrameworkManifest(manifest)
  const target = resolve(root, '.local/framework/wails-v2.12.0')
  // Read the exact module version, not a moving branch or a package submodule.
  const metadata = JSON.parse(await capture('go', ['mod', 'download', '-json', `${manifest.module}@${manifest.version}`]))
  verifyFrameworkModule(manifest, metadata)
  return materializeFramework({ source: metadata.Dir, target, manifest, patchPath, run })
}

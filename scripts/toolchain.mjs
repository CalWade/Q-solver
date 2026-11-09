import { access, mkdir } from 'node:fs/promises'
import { resolve } from 'node:path'

export const wailsModule = 'github.com/wailsapp/wails/v2'
export const wailsVersion = 'v2.12.0'

// Build from the exact module's own locked dependency graph. `go run pkg@version`
// also performs package-submodule/deprecation lookups, even with cached sources.
export async function ensureWails({ directory, platform = process.platform, capture, run, exists = access, makeDirectory = mkdir }) {
  const executable = resolve(directory, `wails-${wailsVersion}${platform === 'win32' ? '.exe' : ''}`)
  try { await exists(executable); return executable } catch (error) { if (error.code !== 'ENOENT') throw error }
  await makeDirectory(directory, { recursive: true })
  const metadata = JSON.parse(await capture('go', ['mod', 'download', '-json', `${wailsModule}@${wailsVersion}`]))
  if (metadata.Error || metadata.Path !== wailsModule || metadata.Version !== wailsVersion || !metadata.Dir) {
    throw new Error(metadata.Error || 'Pinned Wails module metadata is incomplete')
  }
  await run('go', ['build', '-mod=readonly', '-o', executable, './cmd/wails'], metadata.Dir, { GOWORK: 'off' })
  return executable
}

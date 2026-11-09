import { expect } from '@playwright/test'
import { spawn } from 'node:child_process'
import { createInterface } from 'node:readline'
import { resolve, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'
import { setTimeout as delay } from 'node:timers/promises'

export const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..')

export async function receiverProcess(browser) {
  const cdp = await browser.newBrowserCDPSession()
  try {
    const { processInfo } = await cdp.send('SystemInfo.getProcessInfo')
    const receiver = processInfo.find(process => process.type === 'browser')
    if (!receiver || receiver.id <= 0) throw new Error('Owned browser process unavailable')
    return receiver.id
  } finally { await cdp.detach() }
}

export function launchHost(report, extraArgs = []) {
  const outputPath = resolve(root, '.local/reports', report)
  const child = spawn(resolve(root, '.local/bin/hostprobe'), ['-controlled', '-output', outputPath, ...extraArgs], { cwd: root, stdio: ['pipe', 'pipe', 'pipe'] })
  const exit = new Promise(resolve => {
    child.once('exit', (code, signal) => resolve({ code, signal }))
    child.once('error', error => resolve({ code: -1, error: error.message }))
  })
  let output = ''
  child.stderr.on('data', chunk => { output += chunk })
  child.stdout.on('data', chunk => { output += chunk })
  child.stdin.on('error', error => { output += error.message })
  const lines = createInterface({ input: child.stdout })
  const stages = []
  lines.on('line', line => {
    try { const value = JSON.parse(line); if (value.type === 'native-stage') stages.push(value.stage) } catch {}
  })
  return { child, exit, stages, output: () => output, async close() {
    child.stdin.end()
    if (child.exitCode === null && child.signalCode === null) {
      const timeout = new AbortController()
      try {
        const stopped = await Promise.race([exit.then(() => true), delay(12000, false, { signal: timeout.signal })])
        if (!stopped) { child.kill('SIGKILL'); await exit }
      } finally { timeout.abort() }
    }
    lines.close()
  } }
}

export async function prepareReceiver(page, context) {
  await context.route('**/*', route => route.abort())
  const cdp = await context.newCDPSession(page)
  await cdp.send('Emulation.setFocusEmulationEnabled', { enabled: false })
  await page.setContent(`<!doctype html><meta charset="utf-8"><title>Q-solver isolated receiver</title><h1>独立事件接收页面</h1><p>记录本地合成测试的键盘与焦点事件。</p><script>
  window.focusEvents=[]; window.nativeKeyEvents=[];
  for(const type of ['focus','blur']) window.addEventListener(type,e=>focusEvents.push({type,trusted:e.isTrusted,at:performance.now()}));
  for(const type of ['visibilitychange','fullscreenchange']) document.addEventListener(type,e=>focusEvents.push({type,trusted:e.isTrusted,at:performance.now(),visibility:document.visibilityState}));
  for(const type of ['keydown','keyup']) window.addEventListener(type,e=>{if(['MetaLeft','MetaRight','KeyA','KeyB'].includes(e.code)) nativeKeyEvents.push({type,code:e.code,repeat:e.repeat,meta:e.metaKey,trusted:e.isTrusted});});
  </script>`)
  await page.bringToFront()
  await expect.poll(() => page.evaluate(() => document.hasFocus())).toBe(true)
  await page.evaluate(() => { window.focusEvents = [] })
}

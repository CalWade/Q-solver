import { test, expect } from '@playwright/test'
import { spawn } from 'node:child_process'
import { createInterface } from 'node:readline'
import { readFile, writeFile, mkdir } from 'node:fs/promises'
import { resolve, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'
import { setTimeout as delay } from 'node:timers/promises'
import { evaluateFocusProbe } from '../../scripts/focus-evidence.mjs'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
function launchHost(report) {
  const child = spawn(resolve(root, '.local/bin/hostprobe'), ['-controlled', '-output', `.local/reports/${report}`], { cwd: root, stdio: ['pipe', 'pipe', 'pipe'] })
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

async function prepareReceiver(page, context) {
  await context.route('**/*', route => route.abort())
  // Playwright enables focus emulation by default. Keeping it enabled would
  // make document.hasFocus() unsuitable as evidence of operating-system focus.
  const cdp = await context.newCDPSession(page)
  await cdp.send('Emulation.setFocusEmulationEnabled', { enabled: false })
  await page.setContent(`<!doctype html><meta charset="utf-8"><title>Q-solver isolated focus receiver</title><h1>独立焦点接收页面</h1><p>仅记录本地测试的焦点和可见性事件，不连接会议或笔试平台。</p><script>
  window.focusEvents=[];
  for(const type of ['focus','blur']) window.addEventListener(type,e=>focusEvents.push({type, trusted:e.isTrusted, at:performance.now()}));
  for(const type of ['visibilitychange','fullscreenchange']) document.addEventListener(type,e=>focusEvents.push({type, trusted:e.isTrusted, at:performance.now(), visibility:document.visibilityState}));
  </script>`)
  await page.bringToFront()
  await expect.poll(() => page.evaluate(() => document.hasFocus())).toBe(true)
  await page.evaluate(() => { window.focusEvents = [] })
}

test('independent Chrome observes passive retention, explicit blur and return', async ({ page, context, browser }) => {
  await prepareReceiver(page, context)
  const host = launchHost('focus-host.json')
  const observations = []
  const errors = []
  let native
  const reportPath = resolve(root, '.local/reports/focus-browser.json')
  try {
    for (const [index, stage] of ['startup-passive', 'explicit-editor', 'returned-passive'].entries()) {
      await expect.poll(() => host.stages[index], { timeout: 15000, message: `native stage ${stage}` }).toBe(stage)
      await expect.poll(() => page.evaluate(() => document.hasFocus()), { timeout: 5000 }).toBe(stage !== 'explicit-editor')
      observations.push(await page.evaluate(stage => ({ stage, focused: document.hasFocus(), visibility: document.visibilityState, fullscreen: Boolean(document.fullscreenElement), events: [...window.focusEvents], focusEmulationDisabled: true }), stage))
      host.child.stdin.write(stage + '\n')
    }
    const result = await host.exit
    expect(result, host.output()).toEqual({ code: 0, signal: null })
    native = JSON.parse(await readFile(resolve(root, '.local/reports/focus-host.json'), 'utf8'))
    expect(evaluateFocusProbe({ observations, native }).passed).toBe(true)
  } catch (error) {
    errors.push(error.message)
    throw error
  } finally {
    // Closing the controller makes the native host restore focus and quit.
    // Bound cleanup as well, in case the native process failed before its gate.
    await host.close()
    await mkdir(dirname(reportPath), { recursive: true })
    await writeFile(reportPath, JSON.stringify({ recordedAt: new Date().toISOString(), browser: browser.version(), native, observations, errors, ...evaluateFocusProbe({ observations, native, errors }), limitations: ['独立临时 Chrome 页面，关闭 Playwright 焦点模拟；不是第三方平台结果。', '没有注入物理鼠标/键盘、验证输入法或采集屏幕共享。显式编辑预期产生外部页面 blur。'] }, null, 2) + '\n')
  }
})

test('a disconnected observer restores browser focus and exits with failure', async ({ page, context }) => {
  await prepareReceiver(page, context)
  const host = launchHost('focus-disconnected.json')
  try {
    await expect.poll(() => host.stages[0], { timeout: 15000 }).toBe('startup-passive')
    host.child.stdin.write('startup-passive\n')
    await expect.poll(() => host.stages[1], { timeout: 15000 }).toBe('explicit-editor')
    await expect.poll(() => page.evaluate(() => document.hasFocus())).toBe(false)
    host.child.stdin.end()
    expect(await host.exit, host.output()).toEqual({ code: 1, signal: null })
    await expect.poll(() => page.evaluate(() => document.hasFocus())).toBe(true)
    const report = JSON.parse(await readFile(resolve(root, '.local/reports/focus-disconnected.json'), 'utf8'))
    expect(report.errors.some(error => error.includes('observer disconnected'))).toBe(true)
    expect(report.checks.some(check => !check.passed)).toBe(true)
  } finally { await host.close() }
})

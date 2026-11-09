import { test, expect } from '@playwright/test'
import { readFile, writeFile, mkdir } from 'node:fs/promises'
import { resolve, dirname } from 'node:path'
import { launchHost, prepareReceiver, root } from './receiver.js'
import { evaluateFocusProbe } from '../../scripts/focus-evidence.mjs'

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

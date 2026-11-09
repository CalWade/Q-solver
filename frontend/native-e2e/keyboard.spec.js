import { test, expect } from '@playwright/test'
import { readFile, writeFile, mkdir, copyFile } from 'node:fs/promises'
import { setTimeout as delay } from 'node:timers/promises'
import { resolve, dirname } from 'node:path'
import { launchHost, prepareReceiver, receiverProcess, root } from './receiver.js'
import { passiveCaseIds, evaluateKeyboardEvidence } from '../../scripts/keyboard-evidence.mjs'

test('native event tap protects complete chords, preserves the other side and pauses in its own editor', async ({ page, context, browser }, testInfo) => {
  await prepareReceiver(page, context)
  const pid = await receiverProcess(browser)
  const nativePath = testInfo.outputPath('native-host.json')
  const host = launchHost(nativePath, ['-keyboard', '-receiver-pid', String(pid)])
  const observations = []
  const errors = []
  let native
  let stageIndex = 0
  async function waitStage(stage) {
    const deadline = Date.now() + 15000
    while (host.stages[stageIndex] === undefined) {
      if (host.child.exitCode !== null || Date.now() > deadline) {
        throw new Error(`Native stage ${stage} was not reached.\n${host.output()}`)
      }
      await delay(20)
    }
    expect(host.stages[stageIndex]).toBe(stage)
    stageIndex++
  }
  async function acknowledge(stage) {
    await waitStage(stage)
    host.child.stdin.write(stage + '\n')
  }
  async function observeCase(id) {
    await waitStage('keys-ready/' + id)
    await page.evaluate(() => { window.nativeKeyEvents = [] })
    host.child.stdin.write('keys-ready/' + id + '\n')
    await waitStage('keys-done/' + id)
    try {
      if (id !== 'own-editor') {
        await page.waitForFunction(() => nativeKeyEvents.some(event => event.code === 'KeyB' && event.type === 'keyup'), null, { timeout: 5000 })
      }
    } finally {
      observations.push(await page.evaluate(id => ({ id, focused: document.hasFocus(), focusEmulationDisabled: true, events: [...nativeKeyEvents] }), id))
    }
    host.child.stdin.write('keys-done/' + id + '\n')
  }
  try {
    await acknowledge('startup-passive')
    for (const id of passiveCaseIds) await observeCase(id)
    await observeCase('own-editor')
    await acknowledge('explicit-editor')
    await acknowledge('returned-passive')
    await observeCase('stale-editor-intent')
    await observeCase('restored-protection')
    const result = await host.exit
    native = JSON.parse(await readFile(nativePath, 'utf8'))
    expect(result, host.output()).toEqual({ code: 0, signal: null })
    expect(evaluateKeyboardEvidence({ observations, native }).passed).toBe(true)
  } catch (error) {
    errors.push(error.message)
    throw error
  } finally {
    await host.close()
    if (!native) {
      try { native = JSON.parse(await readFile(nativePath, 'utf8')) } catch {}
    }
    const path = resolve(root, '.local/reports/keyboard-browser.json')
    await mkdir(dirname(path), { recursive: true })
    const report = JSON.stringify({ recordedAt: new Date().toISOString(), browser: browser.version(), observations, native, errors, ...evaluateKeyboardEvidence({ observations, native, errors }), limitations: ['由带标记的 CGEvent 软件事件驱动，经过真实 session event tap、应用服务和独立 Chrome/WKWebView。', '诊断驱动仅处理带匹配标记的事件。物理设备、输入法和第三方平台另行验证。'] }, null, 2) + '\n'
    await writeFile(path, report)
    if (native) await copyFile(nativePath, resolve(root, '.local/reports/keyboard-host.json'))
    await testInfo.attach('keyboard-evidence', { body: Buffer.from(report), contentType: 'application/json' })
  }
})

test('observer disconnection removes the active native keyboard listener', async ({ page, context, browser }) => {
  await prepareReceiver(page, context)
  const pid = await receiverProcess(browser)
  const host = launchHost('keyboard-disconnected.json', ['-keyboard', '-receiver-pid', String(pid)])
  try {
    await expect.poll(() => host.stages[0], { timeout: 15000 }).toBe('startup-passive')
    host.child.stdin.write('startup-passive\n')
    await expect.poll(() => host.stages[1], { timeout: 15000 }).toBe('keys-ready/baseline-left')
    host.child.stdin.write('keys-ready/baseline-left\n')
    await expect.poll(() => host.stages[2], { timeout: 15000 }).toBe('keys-done/baseline-left')
    host.child.stdin.write('keys-done/baseline-left\n')
    await expect.poll(() => host.stages[3], { timeout: 15000 }).toBe('keys-ready/protected-left')
    host.child.stdin.end()
    expect(await host.exit, host.output()).toEqual({ code: 1, signal: null })
    const report = JSON.parse(await readFile(resolve(root, '.local/reports/keyboard-disconnected.json'), 'utf8'))
    expect(report.keyboard.stopped).toBe(true)
    expect(report.keyboard.listenerReleased).toBe(true)
    expect(report.keyboard.nativeTapsAfter).toBe(report.keyboard.nativeTapsBefore)
    expect(report.errors.some(error => error.includes('observer disconnected'))).toBe(true)
    await expect.poll(() => page.evaluate(() => document.hasFocus())).toBe(true)
  } finally { await host.close() }
})

test('a foreground change stops injection before keys reach another owned browser', async ({ page, context, browser }, testInfo) => {
  await prepareReceiver(page, context)
  const pid = await receiverProcess(browser)
  const nativePath = testInfo.outputPath('guard-native.json')
  const host = launchHost(nativePath, ['-keyboard', '-receiver-pid', String(pid)])
  let alternate
  try {
    await expect.poll(() => host.stages[0], { timeout: 15000 }).toBe('startup-passive')
    host.child.stdin.write('startup-passive\n')
    await expect.poll(() => host.stages[1], { timeout: 15000 }).toBe('keys-ready/baseline-left')
    host.child.stdin.write('keys-ready/baseline-left\n')
    await expect.poll(() => host.stages[2], { timeout: 15000 }).toBe('keys-done/baseline-left')
    host.child.stdin.write('keys-done/baseline-left\n')
    await expect.poll(() => host.stages[3], { timeout: 15000 }).toBe('keys-ready/protected-left')

    alternate = await browser.browserType().launch({ channel: process.env.PLAYWRIGHT_CHANNEL || 'chrome', headless: false })
    const alternateContext = await alternate.newContext()
    const alternatePage = await alternateContext.newPage()
    await prepareReceiver(alternatePage, alternateContext)
    expect(await receiverProcess(alternate)).not.toBe(pid)
    host.child.stdin.write('keys-ready/protected-left\n')

    expect(await host.exit, host.output()).toEqual({ code: 1, signal: null })
    const report = JSON.parse(await readFile(nativePath, 'utf8'))
    expect(report.errors.some(error => error.includes('foreground changed'))).toBe(true)
    expect(report.keyboard.listenerReleased).toBe(true)
    expect(await alternatePage.evaluate(() => nativeKeyEvents)).toEqual([])
    await testInfo.attach('foreground-guard', { path: nativePath, contentType: 'application/json' })
  } finally {
    await host.close()
    if (alternate) await alternate.close()
  }
})

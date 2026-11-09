import { test, expect } from '@playwright/test'

async function rpc(request, method, ...args) {
  const response = await request.post(`/rpc/${method}`, { data: args })
  expect(response.ok(), await response.text()).toBeTruthy()
  return response.json()
}
async function openSettings(page) {
  await page.getByTitle('设置', { exact: true }).click()
  await expect(page.locator('#settings-modal')).toBeVisible()
  await page.locator('#settings-modal .tab').getByText('常规', { exact: true }).click()
}
async function record(page, label, modifier, key) {
  const button = page.getByRole('button', { name: `录制${label}快捷键`, exact: true })
  await button.click()
  await expect(button).toContainText('请按组合键')
  await page.keyboard.down(`${modifier}Left`)
  await page.keyboard.down(key)
  await page.keyboard.up(key)
  await page.keyboard.up(`${modifier}Left`)
  await expect(button).not.toContainText('请按组合键')
}
function chord(modifier, key, side = 'Left') {
  return [{ key: modifier + side, down: true }, { key, down: true }, { key: modifier + side, down: false }, { key, down: false }]
}
test.beforeEach(async ({ page, request }) => {
  await rpc(request, 'HarnessReset')
  await page.goto('/')
  await expect(page.getByTitle('设置', { exact: true })).toBeVisible()
  await expect.poll(async () => page.evaluate(() => document.querySelector('#app').textContent.length)).toBeGreaterThan(30)
})

test('save left-side protection, reload configuration, suppress the complete sequence and dispatch once', async ({ page, request }) => {
  const platform = await page.evaluate(() => /mac/i.test(navigator.platform))
  const modifier = platform ? 'Meta' : 'Control'
  await openSettings(page)
  await page.getByLabel('启用键盘防护', { exact: true }).check()
  await record(page, '截图', modifier, 'KeyQ')
  // Recording changes only the draft.
  expect((await rpc(request, 'GetSettings')).keyboardProtection.enabled).toBe(false)
  await page.getByRole('button', { name: '保存', exact: true }).click()
  await expect(page.locator('#settings-modal')).toBeHidden()
  await rpc(request, 'HarnessReload')
  const saved = await rpc(request, 'GetSettings')
  expect(saved.keyboardProtection).toEqual({ enabled: true, blockLeft: true, blockRight: false })
  expect(saved.shortcuts.screenshot.keys).toEqual(expect.arrayContaining([modifier, 'KeyQ']))
  const decisions = await rpc(request, 'HarnessInput', chord(modifier, 'KeyQ'))
  expect(decisions.every(decision => decision.suppress)).toBe(true)
  await expect.poll(async () => (await rpc(request, 'HarnessSnapshot')).commands).toEqual(['screenshot'])
  const right = await rpc(request, 'HarnessInput', chord(modifier, 'KeyQ', 'Right'))
  expect(right.every(decision => !decision.suppress && !decision.action)).toBe(true)
})

test('editing pauses commands, cancellation preserves saved bindings, closing restores protection', async ({ page, request }) => {
  const modifier = await page.evaluate(() => /mac/i.test(navigator.platform) ? 'Meta' : 'Control')
  const cfg = await rpc(request, 'GetSettings')
  cfg.keyboardProtection.enabled = true
  cfg.shortcuts.screenshot = { keys: [modifier, 'KeyQ'] }
  expect(await rpc(request, 'UpdateSettings', JSON.stringify(cfg))).toBe('')
  await page.reload()
  await openSettings(page)
  await expect(page.getByTestId('keyboard-status')).toContainText('已暂停')
  const editing = await rpc(request, 'HarnessInput', chord(modifier, 'KeyQ').map(e => ({ ...e, ownWindow: true })))
  expect(editing.every(e => !e.suppress && !e.action)).toBe(true)
  await record(page, '截图', modifier, 'KeyW')
  await page.getByRole('button', { name: '关闭设置', exact: true }).click()
  expect((await rpc(request, 'GetSettings')).shortcuts.screenshot.keys).toContain('KeyQ')
  await expect.poll(async () => (await rpc(request, 'GetKeyboardProtectionStatus')).state).toBe('active')
  expect((await rpc(request, 'HarnessInput', chord(modifier, 'KeyQ'))).every(e => e.suppress)).toBe(true)
})

test('invalid sides and conflicting shortcuts cannot overwrite the last valid configuration', async ({ page, request }) => {
  const modifier = await page.evaluate(() => /mac/i.test(navigator.platform) ? 'Meta' : 'Control')
  await openSettings(page)
  await page.getByLabel('启用键盘防护', { exact: true }).check()
  await page.getByLabel(/屏蔽左/).uncheck()
  await page.getByRole('button', { name: '保存', exact: true }).click()
  await expect(page.locator('#settings-modal')).toBeVisible()
  expect((await rpc(request, 'GetSettings')).keyboardProtection.enabled).toBe(false)
  await page.getByLabel(/屏蔽左/).check()
  await record(page, '截图', modifier, 'KeyQ')
  await record(page, '发送解题', modifier, 'KeyQ')
  await page.getByRole('button', { name: '保存', exact: true }).click()
  await expect(page.getByText(/快捷键冲突/)).toBeVisible()
  expect((await rpc(request, 'GetSettings')).keyboardProtection.enabled).toBe(false)
})

test('listener failure is visible and saving cannot report it repaired', async ({ page, request }) => {
  await rpc(request, 'HarnessFailDriver')
  await openSettings(page)
  await page.getByLabel('启用键盘防护', { exact: true }).check()
  await page.getByRole('button', { name: '保存', exact: true }).click()
  await openSettings(page)
  await expect(page.getByTestId('keyboard-status')).toContainText('尚未生效')
  await page.getByRole('button', { name: '重新检查', exact: true }).click()
  await expect(page.getByTestId('keyboard-status')).not.toContainText('尚未生效')
})

test('failed native focus keeps settings closed and a retry reconciles passive state', async ({ page, request }) => {
  await rpc(request, 'HarnessFailWindow', 'input')
  await page.getByTitle('设置', { exact: true }).click()
  await expect(page.getByText(/simulated input failure/).first()).toBeVisible()
  await expect(page.locator('#settings-modal')).toBeHidden()
  await expect.poll(async () => (await rpc(request, 'GetWindowStatus')).desired.inputMode).toBe('passive')
  await rpc(request, 'HarnessFailWindow', '')
  expect(await rpc(request, 'RetryWindowPolicy')).toBe('')
  await openSettings(page)
  await expect.poll(async () => (await rpc(request, 'GetWindowStatus')).desired.inputMode).toBe('editing')
  await page.getByRole('button', { name: '关闭设置', exact: true }).click()
  await expect.poll(async () => (await rpc(request, 'GetWindowStatus')).windows[0].applied.inputMode).toBe('passive')
})

test('closing settings returns native focus even if keyboard cleanup fails', async ({ page, request }) => {
  await openSettings(page)
  await rpc(request, 'HarnessFailDriver')
  await page.getByRole('button', { name: '关闭设置', exact: true }).click()
  await expect(page.locator('#settings-modal')).toBeHidden()
  await expect.poll(async () => (await rpc(request, 'GetWindowStatus')).windows[0].applied.inputMode).toBe('passive')
})

test('repeated settings clicks share one pending native focus acquisition', async ({ page }) => {
  let restoreCalls = 0
  let releaseFocus
  const gate = new Promise(resolve => { releaseFocus = resolve })
  await page.route('**/rpc/RestoreFocus', async route => {
    restoreCalls++
    await gate
    await route.continue()
  })
  try {
    await page.getByTitle('设置', { exact: true }).evaluate(button => { button.click(); button.click() })
    await expect.poll(() => restoreCalls).toBeGreaterThan(0)
    releaseFocus()
    await expect(page.locator('#settings-modal')).toBeVisible()
    expect(restoreCalls).toBe(1)
  } finally {
    releaseFocus()
  }
})

test('capture policy is independent of local visibility and a native failure remains visible', async ({ page, request }) => {
  await openSettings(page)
  const protection = page.getByLabel('启用屏幕捕获保护', { exact: true })
  await expect(protection).toBeChecked()
  await protection.uncheck()
  await expect(page.getByTestId('window-status')).toHaveText('捕获保护已关闭')
  let status = await rpc(request, 'GetWindowStatus')
  expect(status.windows[0].applied.visible).toBe(true)
  expect(status.windows[0].applied.captureProtection).toBe(false)
  await rpc(request, 'HarnessFailWindow', 'capture')
  await protection.check()
  await expect(page.getByTestId('window-status')).toHaveText('窗口策略尚未完全生效')
  status = await rpc(request, 'GetWindowStatus')
  expect(status.windows[0].capture.applied).toBe(false)
  expect(status.windows[0].applied.visible).toBe(true)
  await rpc(request, 'HarnessFailWindow', '')
  await page.getByRole('button', { name: '重试窗口设置', exact: true }).click()
  await expect(page.getByTestId('window-status')).toContainText('系统保护属性已设置')
})

test('preview uses the capture pipeline and restores the latest disabled protection', async ({ page, request }) => {
  await openSettings(page)
  await page.getByLabel('启用屏幕捕获保护', { exact: true }).uncheck()
  await expect(page.getByTestId('window-status')).toHaveText('捕获保护已关闭')
  const preview = await rpc(request, 'GetScreenshotPreview', 80, 0, false, true, 'fullscreen')
  expect(preview.base64).toMatch(/^data:image\/png;base64,/)
  const status = await rpc(request, 'GetWindowStatus')
  expect(status.captures).toBe(0)
  expect(status.windows[0].captureSuppressed).toBe(false)
  expect(status.windows[0].applied.captureProtection).toBe(false)
  expect(status.windows[0].applied.visible).toBe(true)
})

import test from 'node:test'
import assert from 'node:assert/strict'
import { captureProtectionStatus, capturePolicyApplied } from './window-model.js'

test('window policy reports acknowledgement separately from visibility and capture guarantees', () => {
  const status = { desired: { captureProtection: true, visible: false }, captures: 0, windows: [{ ready: true, capture: { applied: true, mechanism: 'nswindow-sharing-none' } }] }
  assert.equal(capturePolicyApplied(status), true)
  assert.match(captureProtectionStatus(status), /实际效果需验证/)
  status.windows[0].capture.mechanism = 'blank-content'
  assert.match(captureProtectionStatus(status), /仍占位/)
  status.windows[0].error = 'native failure'
  status.windows[0].ready = false
  assert.equal(capturePolicyApplied(status), false)
  assert.match(captureProtectionStatus(status), /尚未完全生效/)
  assert.equal(capturePolicyApplied({}), false)
  assert.match(captureProtectionStatus({ windows: [] }), /尚未就绪/)
})

test('temporary capture protection does not overwrite the disabled user setting', () => {
  const status = { desired: { captureProtection: false }, captures: 1, windows: [{ ready: true, capture: { applied: true } }] }
  assert.match(captureProtectionStatus(status), /截图中/)
  assert.equal(capturePolicyApplied(status), false)
  status.captures = 0
  assert.equal(captureProtectionStatus(status), '捕获保护已关闭')
})

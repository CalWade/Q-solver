import test from 'node:test'
import assert from 'node:assert/strict'
import { createRecorder, createEditingCoordinator, createEditingIntents, beginDialogEditing, endDialogEditing, isTextEditor, modifierFamily, protectionStatus } from './input-model.js'

test('recording preserves the largest chord and deduplicates repeats', () => {
  let result
  const recorder = createRecorder({ complete: value => { result = value }, cancel: () => assert.fail('unexpected cancel') })
  recorder.down('ControlLeft'); recorder.down('KeyB'); recorder.down('KeyB')
  recorder.up('ControlLeft'); assert.equal(result, undefined)
  recorder.up('KeyB')
  assert.deepEqual(result.keys, ['Control', 'KeyB'])
  assert.equal(result.keyName, 'Ctrl+B')
  recorder.up('KeyB'); assert.equal(result.vkCode, 'Control+KeyB')
})

test('dialog, composition and recording intents cannot release one another', async () => {
  const calls = []
  const mode = createEditingIntents(async value => { calls.push(value); return '' })
  await mode.reason('dialog', true)
  await mode.reason('recording', true)
  await mode.reason('recording', false)
  assert.deepEqual(calls, [true])
  await mode.focus(false)
  await mode.focus(true)
  await mode.reason('text', true)
  await mode.reason('dialog', false)
  await mode.reason('text', false)
  assert.deepEqual(calls, [true, false, true, false])
})

test('mouse side buttons can be recorded and a cancelled recorder remains inert', () => {
  let binding
  const recorder = createRecorder({ complete: value => { binding = value }, cancel() {} })
  recorder.down('ControlRight'); recorder.down('MouseBack'); recorder.up('MouseBack'); recorder.up('ControlRight')
  assert.deepEqual(binding.keys, ['Control', 'MouseBack'])
  const cancelled = createRecorder({ complete: () => assert.fail('completed cancelled recorder'), cancel() {} })
  cancelled.down('KeyA'); cancelled.cancel(); cancelled.up('KeyA')
})

test('macOS recording normalizes either Command side', () => {
  for (const side of ['Left', 'Right']) {
    let result
    const recorder = createRecorder({ complete: value => { result = value }, cancel: () => {} })
    recorder.down(`Meta${side}`); recorder.down('KeyZ'); recorder.up('KeyZ'); recorder.up(`Meta${side}`)
    assert.deepEqual(result.keys, ['KeyZ', 'Meta'])
  }
})

test('Escape alone and modifier-only recordings cancel without applying', () => {
  for (const code of ['Escape', 'ControlLeft']) {
    let canceled = 0
    const recorder = createRecorder({ complete: () => assert.fail('invalid chord applied'), cancel: () => canceled++ })
    recorder.down(code); recorder.up(code); recorder.cancel()
    assert.equal(canceled, 1)
  }
})

test('text editing detection includes contenteditable and excludes controls', () => {
  assert.equal(isTextEditor({ tagName: 'INPUT', type: 'text' }), true)
  assert.equal(isTextEditor({ tagName: 'TEXTAREA' }), true)
  assert.equal(isTextEditor({ isContentEditable: true }), true)
  for (const type of ['checkbox', 'range', 'button', 'file']) assert.equal(isTextEditor({ tagName: 'INPUT', type }), false)
  assert.equal(isTextEditor({ tagName: 'TEXTAREA', readOnly: true }), false)
  assert.equal(isTextEditor(null), false)
})

test('editing changes serialize while a native response is pending', async () => {
  const calls = []
  let release
  const coordinator = createEditingCoordinator(async value => { calls.push(value); if (value) await new Promise(resolve => { release = resolve }); return '' })
  const first = coordinator.set(true)
  await new Promise(resolve => setImmediate(resolve))
  const second = coordinator.set(false)
  release()
  await Promise.all([first, second])
  assert.deepEqual(calls, [true, false])
})

test('same-turn changes do not lose the final editing intent', async () => {
  const calls = []
  const coordinator = createEditingCoordinator(async value => { calls.push(value); return '' })
  coordinator.set(false)
  await coordinator.set(true)
  assert.deepEqual(calls, [true])
})

test('failed editing update is reported and a later request can retry', async () => {
  let count = 0; let failures = 0
  const coordinator = createEditingCoordinator(async () => ++count === 1 ? 'failed' : '', () => failures++)
  await assert.rejects(coordinator.set(true), /failed/)
  await coordinator.set(true)
  assert.equal(count, 2); assert.equal(failures, 1)
})

test('cancelling an uncertain editing update explicitly reconciles the native state', async () => {
  const calls = []
  const coordinator = createEditingCoordinator(async value => {
    calls.push(value)
    return value ? 'partial transition' : ''
  })
  await assert.rejects(coordinator.set(true), /partial transition/)
  await coordinator.set(false)
  assert.deepEqual(calls, [true, false])
})

test('dialog editing acquires focus and releases all dialog reasons on close', async () => {
  const calls = []
  const editing = {
    async focus(value) { calls.push(['focus', value]) },
    async reason(name, value) { calls.push([name, value]) },
  }
  const windowApi = {
    async restoreFocus() { calls.push(['acquire']) },
    async removeFocus() { calls.push(['release']) },
  }
  await beginDialogEditing(editing, windowApi)
  await endDialogEditing(editing, windowApi)
  assert.deepEqual(calls, [['focus', true], ['dialog', true], ['acquire'], ['release'], ['recording', false], ['text', false], ['dialog', false]])
})

test('failed focus acquisition rolls back dialog intent and native focus', async () => {
  const calls = []
  const editing = {
    async focus() {},
    async reason(name, active) { calls.push([name, active]) },
  }
  const windowApi = {
    async restoreFocus() { throw new Error('foreground denied') },
    async removeFocus() { calls.push(['released']) },
  }
  await assert.rejects(beginDialogEditing(editing, windowApi), /foreground denied/)
  assert.deepEqual(calls, [['dialog', true], ['released'], ['recording', false], ['text', false], ['dialog', false]])
})

test('focus cleanup runs even when keyboard cleanup throws synchronously', async () => {
  const calls = []
  const editing = { reason(name) { calls.push(name); throw new Error(`failed ${name}`) } }
  await assert.rejects(endDialogEditing(editing, { async removeFocus() { calls.push('release') } }), /failed recording/)
  assert.deepEqual(calls, ['release', 'recording', 'text', 'dialog'])
})

test('keyboard repair settings still acquire native focus when keyboard pause is unavailable', async () => {
  let focused = false
  const editing = {
    async focus() { throw new Error('driver unavailable') },
    async reason() { throw new Error('driver unavailable') },
  }
  await beginDialogEditing(editing, { async restoreFocus() { focused = true } })
  assert.equal(focused, true)
})

test('status labels distinguish desired state from successful activation', () => {
  assert.match(protectionStatus({ state: 'unavailable' }), /尚未生效/)
  assert.match(protectionStatus({ state: 'waiting-release' }), /等待/)
  assert.equal(modifierFamily('MetaRight'), 'Meta')
  assert.equal(modifierFamily('KeyA'), 'KeyA')
})

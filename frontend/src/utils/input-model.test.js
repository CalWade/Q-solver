import test from 'node:test'
import assert from 'node:assert/strict'
import { createRecorder, createEditingCoordinator, createEditingIntents, isTextEditor, modifierFamily, protectionStatus } from './input-model.js'

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

test('status labels distinguish desired state from successful activation', () => {
  assert.match(protectionStatus({ state: 'unavailable' }), /尚未生效/)
  assert.match(protectionStatus({ state: 'waiting-release' }), /等待/)
  assert.equal(modifierFamily('MetaRight'), 'Meta')
  assert.equal(modifierFamily('KeyA'), 'KeyA')
})

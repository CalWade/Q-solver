import test from 'node:test'
import assert from 'node:assert/strict'
import { evaluateFocusEvidence, evaluateFocusProbe } from './focus-evidence.mjs'

const fixture = () => [
  { stage: 'startup-passive', focusEmulationDisabled: true, focused: true, events: [] },
  { stage: 'explicit-editor', focusEmulationDisabled: true, focused: false, events: [{ type: 'blur', trusted: true }] },
  { stage: 'returned-passive', focusEmulationDisabled: true, focused: true, events: [{ type: 'blur', trusted: true }, { type: 'focus', trusted: true }] },
]
test('passive focus is retained while explicit editing is truthfully observable', () => {
  assert.equal(evaluateFocusEvidence(fixture()).passed, true)
})
test('missing stages, emulated focus and synthetic or transient focus changes cannot pass', () => {
  for (const change of [
    list => list.pop(), list => list.reverse(),
    list => { list[0].focusEmulationDisabled = false },
    list => { list[0].events.push({ type: 'blur', trusted: true }, { type: 'focus', trusted: true }) },
    list => { list[1].focused = true },
    list => { list[1].events[0].trusted = false },
    list => { list[2].events.pop() },
  ]) {
    const observations = fixture()
    change(observations)
    assert.equal(evaluateFocusEvidence(observations).passed, false)
  }
})
test('a passing browser cannot hide a failed native host or observer', () => {
  const native = { pageReady: true, textRoundTrip: true, checks: [{ name: 'all host transitions observed', passed: true }] }
  assert.equal(evaluateFocusProbe({ observations: fixture(), native }).passed, true)
  for (const change of [{ native: undefined }, { native: { ...native, checks: [] } }, { native: { ...native, errors: ['native failure'] } }, { native: { ...native, textRoundTrip: false } }, { errors: ['runner failed after the last stage'] }]) {
    assert.equal(evaluateFocusProbe({ observations: fixture(), native, ...change }).passed, false)
  }
})

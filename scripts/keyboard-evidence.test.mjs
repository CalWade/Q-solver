import test from 'node:test'
import assert from 'node:assert/strict'
import { keyboardCaseIds, evaluateKeyboardEvidence } from './keyboard-evidence.mjs'

const event = (code, type, meta = false) => ({ code, type, meta, repeat: false, trusted: true })
const barrier = () => [event('KeyB', 'keydown'), event('KeyB', 'keyup')]
const chord = side => [event(side, 'keydown', true), event('KeyA', 'keydown', true), event('KeyA', 'keyup', true), event(side, 'keyup'), ...barrier()]

function fixture() {
  const observations = keyboardCaseIds.map(id => {
    let events = barrier()
    if (['baseline-left', 'untagged-control', 'protection-off'].includes(id)) events = chord('MetaLeft')
    if (['baseline-right', 'unselected-right'].includes(id)) events = chord('MetaRight')
    if (id === 'own-editor') events = []
    return { id, events, focused: id !== 'own-editor', focusEmulationDisabled: true }
  })
  const native = {
    pageReady: true,
    textRoundTrip: true,
    checks: [{ name: 'all host transitions observed', passed: true }],
    keyboard: { filtering: true, stopped: true, listenerReleased: true, checks: [{ name: 'native scenarios', passed: true }] },
  }
  return { observations, native }
}

test('native decisions and independent event baselines agree', () => {
  assert.equal(evaluateKeyboardEvidence(fixture()).passed, true)
})

test('modifier leaks, orphan releases, repeats and changed unselected keys fail the evidence', () => {
  for (const change of [
    data => data.observations[1].events.unshift(event('MetaLeft', 'keydown', true)),
    data => data.observations[2].events.unshift(event('KeyA', 'keyup')),
    data => { data.observations[1].events[0].meta = true },
    data => { data.observations[4].events[1].meta = false },
    data => { data.observations[0].events[1].trusted = false },
    data => { data.observations[8].events.push(event('KeyA', 'keydown')) },
  ]) {
    const data = fixture()
    change(data)
    assert.equal(evaluateKeyboardEvidence(data).passed, false)
  }
})

test('missing native cleanup, emulation or an interrupted observer cannot claim success', () => {
  for (const change of [
    data => { data.native.keyboard.listenerReleased = false },
    data => { data.native.keyboard.filtering = false },
    data => { data.native.keyboard.checks[0].passed = false },
    data => { data.native.keyboard.errors = ['listener failed'] },
    data => { data.native = undefined },
    data => data.observations.pop(),
    data => { data.observations[0].focusEmulationDisabled = false },
    data => { data.errors = ['runner failed after receiving all cases'] },
  ]) {
    const data = fixture()
    change(data)
    assert.equal(evaluateKeyboardEvidence(data).passed, false)
  }
})

import test from 'node:test'
import assert from 'node:assert/strict'
import { initialWorkflow, reduceWorkflow, requestKey, running, terminal } from './workflow-model.js'

const event = (kind, requestId, sequence, extra = {}) => ({ kind, requestId, sequence, sessionId: 'fixture', ...extra })
test('late chunks, duplicate delivery and stale cancellation cannot affect a newer request', () => {
  let state = reduceWorkflow(initialWorkflow(), event('started', 1, 1))
  state = reduceWorkflow(state, event('chunk', 1, 2, { text: 'old' }))
  state = reduceWorkflow(state, event('started', 2, 3))
  for (const stale of [event('chunk', 1, 4), event('cancelled', 1, 5), event('chunk', 2, 2)]) assert.equal(reduceWorkflow(state, stale), state)
  state = reduceWorkflow(state, event('chunk', 2, 6, { text: 'draft' }))
  state = reduceWorkflow(state, event('completed', 2, 7, { text: 'corrected final' }))
  assert.equal(state.content, 'corrected final')
  assert.equal(reduceWorkflow(state, event('chunk', 2, 8, { text: 'late' })), state)
  assert.equal(requestKey(state), 'fixture:2')
})
test('snapshot reconnect restores partial output without appending it twice', () => {
  let state = reduceWorkflow(initialWorkflow(), { ...event('snapshot', 4, 12), phase: 'generating', content: 'partial', thinking: 'reason', requestImages: [{ id: 2, dataURL: 'fixture' }] })
  assert.equal(reduceWorkflow(state, event('chunk', 4, 11, { text: 'duplicate' })), state)
  state = reduceWorkflow(state, event('chunk', 4, 13, { text: ' tail' }))
  assert.equal(state.content, 'partial tail')
  assert.equal(state.requestImages.length, 1)
  assert.equal(reduceWorkflow(state, { ...event('snapshot', 4, 10), phase: 'generating' }), state)
})
test('queue changes and terminal phases are independent of request output', () => {
  let state = reduceWorkflow(initialWorkflow(), event('screenshots', 0, 1, { images: [{ id: 1 }] }))
  state = reduceWorkflow(state, event('started', 1, 2, { images: state.images, keepContext: true }))
  state = reduceWorkflow(state, event('screenshots', 0, 3))
  assert.equal(state.images.length, 0)
  assert.equal(state.requestImages.length, 1)
  state = reduceWorkflow(state, event('ready', 1, 4, { images: state.requestImages, keepContext: true }))
  assert.ok(running(state.phase))
  state = reduceWorkflow(state, event('failed', 1, 5, { text: 'offline' }))
  assert.ok(terminal(state.phase))
  assert.equal(state.error, 'offline')
  assert.equal(reduceWorkflow(state, { ...event('started', 8, 9), sessionId: 'old-process' }), state)
})

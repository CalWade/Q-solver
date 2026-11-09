export const terminal = phase => ['completed', 'failed', 'cancelled'].includes(phase)
export const running = phase => phase === 'capturing' || phase === 'generating'
export const requestKey = state => `${state.sessionId}:${state.requestId}`
export const initialWorkflow = () => ({ sessionId: '', requestId: 0, sequence: 0, phase: 'idle', images: [], requestImages: [], content: '', thinking: '', error: '', keepContext: false })

// Return the same object for stale/invalid events so consumers perform no effects.
export function reduceWorkflow(state, event) {
  if (!event || !Number.isSafeInteger(event.sequence) || event.sequence < 0) return state
  if (event.kind === 'snapshot') {
    if (state.sessionId === event.sessionId && event.sequence < state.sequence) return state
    return { ...initialWorkflow(), ...event, images: [...(event.images || [])], requestImages: [...(event.requestImages || [])] }
  }
  if (state.sessionId && event.sessionId !== state.sessionId) return state
  if (event.sequence <= state.sequence) return state
  const next = { ...state, sessionId: event.sessionId, sequence: event.sequence }
  if (event.kind === 'screenshots') return { ...next, images: [...(event.images || [])] }
  if (event.kind === 'started') {
    if (event.requestId <= state.requestId) return state
    return { ...initialWorkflow(), ...next, requestId: event.requestId, phase: 'capturing', content: '', thinking: '', error: '', requestImages: [...(event.images || [])], keepContext: event.keepContext }
  }
  if (event.requestId !== state.requestId || terminal(state.phase)) return state
  switch (event.kind) {
    case 'ready': return { ...next, phase: 'generating', requestImages: [...(event.images || [])], keepContext: event.keepContext }
    case 'chunk': return { ...next, content: state.content + (event.text || '') }
    case 'thinking': return { ...next, thinking: state.thinking + (event.text || '') }
    case 'completed': return { ...next, phase: 'completed', content: event.text || '', thinking: event.thinking || state.thinking }
    case 'failed': return { ...next, phase: 'failed', error: event.text || '请求失败' }
    case 'cancelled': return { ...next, phase: 'cancelled' }
    default: return state
  }
}

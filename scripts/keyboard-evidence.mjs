export const passiveCaseIds = ['baseline-left', 'protected-left', 'release-order-repeat', 'baseline-right', 'unselected-right', 'both-sides', 'untagged-control', 'protection-off']
export const keyboardCaseIds = [...passiveCaseIds, 'own-editor', 'stale-editor-intent', 'restored-protection']

const eventSignature = events => JSON.stringify(events.map(({ code, type, repeat, meta, trusted }) => ({ code, type, repeat, meta, trusted })))
const barrier = [
  { code: 'KeyB', type: 'keydown', repeat: false, meta: false, trusted: true },
  { code: 'KeyB', type: 'keyup', repeat: false, meta: false, trusted: true },
]

export function evaluateKeyboardEvidence({ observations, native, errors = [] }) {
  const checks = []
  const nativeKeyboard = native?.keyboard
  const hostReady = Boolean(native?.pageReady && native?.textRoundTrip && !native.errors?.length)
  const hostChecksPassed = Boolean(native?.checks?.some(check => check.name === 'all host transitions observed' && check.passed) && native.checks.every(check => check.passed))
  const inputReady = Boolean(nativeKeyboard?.filtering && nativeKeyboard.stopped && nativeKeyboard.listenerReleased && !nativeKeyboard.errors?.length)
  const inputChecksPassed = Boolean(nativeKeyboard?.checks?.length && nativeKeyboard.checks.every(check => check.passed))
  const nativeValid = hostReady && hostChecksPassed && inputReady && inputChecksPassed
  checks.push({ name: 'native host, input cases and listener cleanup passed', passed: nativeValid })
  checks.push({ name: 'all independent receiver cases observed in order', passed: observations.length === keyboardCaseIds.length && observations.every((o, i) => o.id === keyboardCaseIds[i]) })
  const byId = Object.fromEntries(observations.map(o => [o.id, o]))
  for (const observation of observations) {
    let passed = observation.focusEmulationDisabled === true && observation.focused === (observation.id !== 'own-editor') && observation.events.every(event => event.trusted)
    switch (observation.id) {
      case 'protected-left':
      case 'release-order-repeat':
      case 'both-sides':
      case 'stale-editor-intent':
      case 'restored-protection':
        passed = passed && eventSignature(observation.events) === eventSignature(barrier)
        break
      case 'baseline-left':
      case 'baseline-right': {
        const side = observation.id === 'baseline-left' ? 'MetaLeft' : 'MetaRight'
        const modifierReceived = observation.events.some(event => event.code === side && event.type === 'keydown')
        const chordReceived = observation.events.some(event => event.code === 'KeyA' && event.type === 'keydown' && event.meta)
        const barrierReceived = observation.events.some(event => event.code === 'KeyB' && event.type === 'keyup' && !event.meta)
        passed = passed && modifierReceived && chordReceived && barrierReceived
        break
      }
      case 'unselected-right':
        passed = passed && byId['baseline-right']?.events.length > 0 && eventSignature(observation.events) === eventSignature(byId['baseline-right'].events)
        break
      case 'untagged-control':
      case 'protection-off':
        passed = passed && byId['baseline-left']?.events.length > 0 && eventSignature(observation.events) === eventSignature(byId['baseline-left'].events)
        break
      case 'own-editor':
        passed = passed && observation.events.length === 0
        break
      default:
        passed = false
    }
    checks.push({ name: observation.id, passed })
  }
  checks.push({ name: 'observer completed', passed: errors.length === 0 })
  return { passed: checks.every(check => check.passed), checks }
}

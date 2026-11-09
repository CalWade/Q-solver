export function evaluateFocusEvidence(observations) {
  const stages = ['startup-passive', 'explicit-editor', 'returned-passive']
  const checks = [{ name: 'all browser stages observed once and in order', passed: observations.length === 3 && observations.every((o, i) => o.stage === stages[i]) }]
  for (const [i, o] of observations.entries()) {
    const expectedFocus = i !== 1
    const blurIndex = o.events.findIndex(event => event.type === 'blur' && event.trusted === true)
    const restored = o.events.some((event, j) => j > blurIndex && event.type === 'focus' && event.trusted === true)
    const validEvents = i === 0 ? o.events.every(event => event.type !== 'blur') : blurIndex >= 0 && (i !== 2 || restored)
    checks.push({ name: `${o.stage}: real page focus and event evidence`, passed: o.focusEmulationDisabled === true && o.focused === expectedFocus && validEvents })
  }
  return { passed: checks.every(check => check.passed), checks }
}

export function evaluateFocusProbe({ observations, native, errors = [] }) {
  const browser = evaluateFocusEvidence(observations)
  const nativePassed = Boolean(native?.pageReady && native?.textRoundTrip && !native.errors?.length && native.checks?.some(check => check.name === 'all host transitions observed' && check.passed) && native.checks.every(check => check.passed))
  const checks = [...browser.checks, { name: 'native host completed all checks', passed: nativePassed }, { name: 'observer runner completed', passed: errors.length === 0 }]
  return { passed: checks.every(check => check.passed), checks }
}

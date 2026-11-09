import { isTextEditor, createEditingIntents } from '../utils/input-model'
import { api } from './api'

let reportError = () => {}
export const inputEditing = createEditingIntents(api.setInputEditing, error => reportError(error))

export function installInputEditing({ onError }) {
  reportError = onError
  let composing = false
  const ignore = promise => promise.catch(() => {})
  const update = () => ignore(inputEditing.reason('text', composing || isTextEditor(document.activeElement)))
  const focusOut = () => queueMicrotask(update)
  const compositionStart = () => { composing = true; update() }
  const compositionEnd = () => { composing = false; update() }
  const blur = () => { composing = false; ignore(inputEditing.focus(false)) }
  const focus = () => { update(); ignore(inputEditing.focus(true)) }
  document.addEventListener('focusin', update)
  document.addEventListener('focusout', focusOut)
  document.addEventListener('compositionstart', compositionStart)
  document.addEventListener('compositionend', compositionEnd)
  window.addEventListener('blur', blur)
  window.addEventListener('focus', focus)
  ignore(inputEditing.focus(document.hasFocus()))
  return () => {
    document.removeEventListener('focusin', update)
    document.removeEventListener('focusout', focusOut)
    document.removeEventListener('compositionstart', compositionStart)
    document.removeEventListener('compositionend', compositionEnd)
    window.removeEventListener('blur', blur)
    window.removeEventListener('focus', focus)
    ignore(inputEditing.clear())
    reportError = () => {}
  }
}

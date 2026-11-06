const families = ['Control', 'Meta', 'Alt', 'Shift']

export function modifierFamily(code) {
  return families.find(family => code === family || code === `${family}Left` || code === `${family}Right`) || code
}

export function displayKeys(keys) {
  return [...keys].sort((a, b) => Number(!families.includes(a)) - Number(!families.includes(b)) || a.localeCompare(b))
    .map(key => key.replace(/^Key|^Digit/g, '').replace('Control', 'Ctrl').replace('Meta', 'Cmd')).join('+')
}

export function isTextEditor(element) {
  if (!element || element.disabled || element.readOnly) return false
  if (element.isContentEditable) return true
  if (element.tagName === 'TEXTAREA' || element.tagName === 'SELECT') return true
  return element.tagName === 'INPUT' && !['button', 'submit', 'reset', 'checkbox', 'radio', 'range', 'file', 'color', 'hidden'].includes(element.type)
}

export function protectionStatus(status) {
  const labels = { active: '键盘防护已生效', off: '键盘防护已关闭', editing: '编辑中，键盘防护已暂停', 'waiting-release': '等待松开按键后应用', unavailable: '键盘防护尚未生效', stopped: '键盘服务尚未启动' }
  return labels[status?.state] || '正在读取键盘状态'
}

// A draft recorder has no backend side effects. Only the settings save commits
// it; repeats do not add keys and Escape alone cancels the recording.
export function createRecorder({ complete, cancel }) {
  const held = new Set()
  let largest = []
  let finished = false
  return {
    down(code) {
      if (finished || held.has(code)) return
      held.add(code)
      if (held.size >= largest.length) largest = [...held]
    },
    up(code) {
      if (finished || !held.has(code)) return
      held.delete(code)
      if (held.size) return
      finished = true
      if (largest.length === 1 && largest[0] === 'Escape') { cancel(); return }
      const keys = [...new Set(largest.map(modifierFamily))].sort()
      if (!keys.some(key => !families.includes(key))) { cancel(); return }
      complete({ keys, vkCode: keys.join('+'), keyName: displayKeys(keys) })
    },
    label: () => displayKeys([...new Set(largest.map(modifierFamily))]),
    cancel() { if (!finished) { finished = true; cancel() } },
  }
}

// Serialize native editing updates. A late response must never restore an old
// intent after the user has moved to another field or application.
export function createEditingCoordinator(setEditing, onError = () => {}) {
  let desired = false
  let applied = false
  let queue = Promise.resolve()
  return {
    set(value) {
      desired = Boolean(value)
      queue = queue.catch(() => {}).then(async () => {
        const next = desired
        if (next === applied) return
        const error = await setEditing(next)
        if (error) throw new Error(error)
        applied = next
      }).catch(error => { onError(error); throw error })
      return queue
    },
  }
}

export function createEditingIntents(setEditing, onError) {
  const reasons = new Set()
  const coordinator = createEditingCoordinator(setEditing, onError)
  let focused = true
  const update = () => coordinator.set(focused && reasons.size > 0)
  return {
    reason(name, active) { active ? reasons.add(name) : reasons.delete(name); return update() },
    focus(active) { focused = active; return update() },
    clear() { reasons.clear(); return update() },
  }
}

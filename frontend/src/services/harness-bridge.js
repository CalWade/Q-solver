// Loaded only by the explicit development mode. Production builds omit it.
export function installHarnessBridge() {
  const handlers = new Map()
  const rpc = async (method, args) => {
    const response = await fetch(`/rpc/${method}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(args) })
    const value = await response.json()
    if (!response.ok) throw new Error(value.error || `Harness HTTP ${response.status}`)
    return value
  }
  window.go = { app: { App: new Proxy({}, { get: (_, method) => (...args) => rpc(method, args) }) } }
  const emit = (name, ...args) => {
    for (const listener of handlers.get(name) || []) {
      listener.callback(...args)
      if (listener.remaining > 0 && --listener.remaining === 0) handlers.get(name).delete(listener)
    }
  }
  window.runtime = {
    EventsOnMultiple(name, callback, remaining) {
      if (!handlers.has(name)) handlers.set(name, new Set())
      const listener = { callback, remaining }
      handlers.get(name).add(listener)
      return () => handlers.get(name)?.delete(listener)
    },
    EventsOff: (...names) => names.forEach(name => handlers.delete(name)),
    EventsOffAll: () => handlers.clear(),
    EventsEmit: emit,
    Quit: () => {},
  }
  let stopped = false
  const poll = async () => {
    try { for (const event of await rpc('HarnessEvents', [])) emit(event.name, ...event.args) }
    catch (error) { if (!stopped) console.warn('Harness connection interrupted', error) }
    if (!stopped) setTimeout(poll, 80)
  }
  poll()
  window.addEventListener('beforeunload', () => { stopped = true }, { once: true })
}

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { initialWorkflow, reduceWorkflow, requestKey, running, terminal } from '../utils/workflow-model'
import { useSolutionStore } from './solution'
import { useSettingsStore } from './settings'
import { useUIStore } from './ui'
import { api } from '../services/api'

export const useWorkflowStore = defineStore('workflow', () => {
  const state = ref(initialWorkflow())
  const solution = useSolutionStore()
  const settings = useSettingsStore()
  const ui = useUIStore()
  const isRunning = computed(() => running(state.value.phase))

  function apply(event) {
    const next = reduceWorkflow(state.value, event)
    if (next === state.value) return
    state.value = next
    if (event.kind === 'screenshots' || next.phase === 'idle') return
    ui.hasStarted = true
    ui.mainVisible = true
    if (event.kind === 'started') {
      solution.handleStopped()
      solution.errorState.show = false
      solution.isLoading = true
      ui.flash('solve')
    }
    if (event.kind === 'ready' || event.kind === 'snapshot' && next.requestImages.length) {
      solution.setUserScreenshot(next.requestImages[0]?.dataURL || '')
      solution.handleStreamStart(next.keepContext, requestKey(next), next.requestImages)
      solution.isLoading = isRunning.value
    }
    if (event.kind === 'chunk') solution.handleStreamChunk(event.text || '')
    if (event.kind === 'thinking') solution.handleThinkingChunk(event.text || '')
    if (event.kind === 'snapshot' && next.requestImages.length) {
      if (next.thinking) solution.handleThinkingChunk(next.thinking)
      if (next.content) solution.handleStreamChunk(next.content)
    }
    if (next.phase === 'completed') solution.handleSolution(next.content, next.thinking)
    if (next.phase === 'cancelled') solution.handleStopped(next.requestImages.length ? '已停止' : '')
    if (next.phase === 'failed') {
      solution.handleStopped()
      const error = { title: '请求出错', desc: next.error, icon: 'x-circle' }
      if (!next.requestImages.length || !solution.handleInlineError(error)) Object.assign(solution.errorState, { ...error, show: true })
    }
    settings.statusText = ({ capturing: '正在截图...', generating: '正在生成...', completed: '解题完成', failed: '请求出错', cancelled: '已停止' })[next.phase] || ''
    settings.statusIcon = terminal(next.phase) ? (next.phase === 'completed' ? '✓' : '·') : '...'
  }
  async function cancel() {
    const id = state.value.requestId
    try { await api.cancelRequest(id) } catch (error) { ui.showToast(error.message, 'error') }
  }
  return { state, isRunning, apply, cancel }
})

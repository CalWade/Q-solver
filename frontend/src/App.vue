<template>
  <TopBar @openSettings="settingsStore.openSettings" />

  <WelcomeView v-if="!ui.hasStarted && solution.history.length === 0" />
  <SolveView v-else />

  <ScreenshotDock />
  <SettingsModal />

  <div id="toast-container">
    <div v-for="(t, i) in ui.toasts" :key="t.id || i" class="toast" :class="[t.type, { show: t.show }]">
      <Icon :name="toastIcon(t.type)" :size="14" class="toast-icon" />
      <span>{{ t.text }}</span>
    </div>
  </div>

  <div class="disclaimer">AI 助手的回答仅供参考。</div>

  <ResizeHandle />
</template>

<script setup>
import { onMounted, onUnmounted } from 'vue'
import TopBar from './components/TopBar.vue'
import WelcomeView from './components/WelcomeView.vue'
import SolveView from './components/SolveView.vue'
import ScreenshotDock from './components/ScreenshotDock.vue'
import SettingsModal from './components/SettingsModal.vue'
import ResizeHandle from './components/ResizeHandle.vue'
import Icon from './components/Icon.vue'

import { useUIStore } from './stores/ui'
import { useSettingsStore } from './stores/settings'
import { useSolutionStore } from './stores/solution'
import { useWorkflowStore } from './stores/workflow'
import { on } from './services/events'
import { api } from './services/api'
import { installInputEditing } from './services/input-editing'
import { capturePolicyApplied } from './utils/window-model'
import { initCodeBlockInteractions } from './utils/markdown-latex'

const ui = useUIStore()
const settingsStore = useSettingsStore()
const solution = useSolutionStore()
const workflow = useWorkflowStore()

let disposeInputEditing = () => {}
onUnmounted(() => disposeInputEditing())

function toastIcon(type) {
  if (type === 'error') return 'x-circle'
  if (type === 'success') return 'check-circle'
  if (type === 'warning') return 'alert-triangle'
  return 'info'
}

onMounted(() => {
  const applyWindowStatus = status => {
    ui.windowStatus = status
    ui.isStealthMode = capturePolicyApplied(status)
    const main = status.windows?.find(window => window.id === 'main')
    if (main) ui.isClickThrough = main.applied.clickThrough
  }
  api.getWindowStatus().then(applyWindowStatus).catch(error => ui.showToast(error.message, 'warning'))
  on('window-state', applyWindowStatus)
  disposeInputEditing = installInputEditing({ api, recording: () => Boolean(settingsStore.recordingAction), onError: error => ui.showToast(error.message, 'warning') })
  api.getKeyboardProtectionStatus().then(status => { settingsStore.keyboardStatus = status }).catch(() => {})
  on('keyboard-protection-status', status => { settingsStore.keyboardStatus = status })
  initCodeBlockInteractions()

  // 历史记录持久化：onMounted 时从磁盘加载 → 之后的 history 变更自动 debounce 写盘。
  // 失败/空文件视为新会话，不阻断 UI。
  let hydrated = false
  const queued = []
  on('workflow-event', event => { if (hydrated) workflow.apply(event); else queued.push(event) })
  solution.hydrateHistory().then(async () => {
    try { workflow.apply({ kind: 'snapshot', ...await api.getWorkflowStatus() }) }
    catch (error) { ui.showToast(error.message, 'warning') }
    hydrated = true
    queued.forEach(event => workflow.apply(event))
  })

  // Panic Button 触发时后端会 EventsEmit('panic-flush')，这里同步把 history 写到磁盘
  // 后再让进程退出。给后端在 panic 里留了 250ms 等这个 flush 完成。
  on('panic-flush', () => {
    solution.forceFlushHistory()
  })

  // 关窗 / 刷新时也尽力 flush 一次（Wails 窗口关闭时浏览器 beforeunload 不一定触发，
  // 但加上更稳。OnShutdown 走的是 Go 进程，前端没机会主动 flush，这条是 dev 模式
  // hot reload 时的兜底）。
  window.addEventListener('beforeunload', () => {
    solution.forceFlushHistory()
  })

  api.getInitStatus().then(s => { ui.initStatus = s })
  on('init-status', (s) => { ui.initStatus = s })

  settingsStore.loadSettings().then(() => {
    settingsStore.resetStatus()
  })

  on('key-recorded', (data) => {
    if (data && data.action) {
      if (settingsStore.tempShortcuts[data.action]) {
        settingsStore.tempShortcuts[data.action].keyName = data.keyName
        settingsStore.tempShortcuts[data.action].vkCode = data.comboID
      } else {
        settingsStore.tempShortcuts[data.action] = { keyName: data.keyName, vkCode: data.comboID }
      }
      if (settingsStore.recordingAction === data.action) {
        settingsStore.recordingText = data.keyName
      }
    }
  })

  on('shortcut-error', async (msg) => {
    ui.showToast(msg, 'error', 2000)
    const targetAction = settingsStore.recordingAction
    settingsStore.recordingAction = null
    settingsStore.recordingText = ''
    api.stopRecordingKey()
    if (!targetAction) return
    try {
      if (settingsStore.shortcuts[targetAction]?.keyName) {
        settingsStore.tempShortcuts[targetAction] = JSON.parse(JSON.stringify(settingsStore.shortcuts[targetAction]))
      } else {
        delete settingsStore.tempShortcuts[targetAction]
      }
    } catch (e) {
      console.error('回滚快捷键配置失败', e)
    }
  })

  on('shortcut-saved', (action) => {
    if (settingsStore.recordingAction === action) {
      settingsStore.recordingAction = null
      ui.showToast('快捷键已保存', 'success')
    }
  })

  on('capture-protection-state', (enabled) => {
    ui.flash('toggle')
    ui.showToast(enabled ? '已请求系统捕获保护' : '捕获保护已关闭', 'info')
  })

  on('copy-code', () => {
    const old = settingsStore.statusText
    settingsStore.statusText = '已复制'
    setTimeout(() => (settingsStore.statusText = old), 2000)
  })

  on('click-through-state', (enabled) => {
    ui.isClickThrough = enabled
  })

  on('scroll-content', (direction) => {
    const el = document.getElementById('content')
    if (!el) return
    el.scrollBy({ top: direction === 'up' ? -50 : 50, behavior: 'smooth' })
  })

  on('require-api-key', () => {
    ui.activeTab = 'api'
    ui.showToast('请打开设置配置 API Key', 'warning')
  })

  on('open-settings', (tab) => {
    if (tab) ui.activeTab = tab
    ui.showToast('请打开设置完成配置后重试', 'warning')
  })

  on('toast', (msg) => {
    ui.showToast(msg)
  })

  document.addEventListener('keydown', event => {
    if (
      event.key === 'F12' ||
      (event.ctrlKey && event.shiftKey && event.key === 'I') ||
      (event.ctrlKey && event.shiftKey && event.key === 'J') ||
      (event.ctrlKey && event.key === 'U')
    ) {
      event.preventDefault()
    }
  })
})
</script>

<style scoped>
.disclaimer {
  text-align: center;
  font-size: 10px;
  color: var(--text-muted);
  padding: 2px 0 0;
  pointer-events: none;
  user-select: none;
  flex-shrink: 0;
  letter-spacing: 0.3px;
  opacity: 0.7;
}
</style>

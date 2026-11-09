<template>
  <section class="window-settings" aria-label="窗口与共享">
    <strong>窗口与共享</strong>
    <label class="window-option">
      <input type="checkbox" :checked="ui.windowStatus.desired?.captureProtection" :disabled="busy" @change="changeProtection" aria-label="启用屏幕捕获保护" />
      启用屏幕捕获保护
    </label>
    <p data-testid="window-status" role="status">{{ captureProtectionStatus(ui.windowStatus) }}</p>
    <p>本地显示与共享画面保护是两个独立设置。保护效果需按会议平台、系统版本和共享方式实测。</p>
    <p>输入模式：{{ ui.windowStatus.desired?.inputMode === 'editing' ? '编辑' : '快捷键操作' }}。关闭设置后恢复原来的鼠标穿透偏好。</p>
    <p v-for="window in failures" :key="window.id" class="window-error">{{ window.error }}</p>
    <button class="btn btn-ghost" type="button" @click="retry" :disabled="busy">重试窗口设置</button>
  </section>
</template>

<script setup>
import { computed, ref } from 'vue'
import { api } from '../services/api'
import { useUIStore } from '../stores/ui'
import { captureProtectionStatus } from '../utils/window-model'
const ui = useUIStore()
const busy = ref(false)
const failures = computed(() => (ui.windowStatus.windows || []).filter(window => window.error))
async function apply(operation) {
  busy.value = true
  try {
    const error = await operation()
    ui.windowStatus = await api.getWindowStatus()
    if (error) ui.showToast(error, 'error')
  } catch (error) { ui.showToast(error.message, 'error') }
  finally { busy.value = false }
}
const changeProtection = event => apply(() => api.setCaptureProtection(event.target.checked))
const retry = () => apply(api.retryWindowPolicy)
</script>

<style scoped>
.window-settings { border: 1px solid var(--border-default); padding: 16px; border-radius: 12px; margin-bottom: 20px; }
.window-settings p { font-size: 12px; line-height: 1.6; color: var(--text-secondary); margin: 8px 0; }
.window-option { display: flex; align-items: center; gap: 8px; margin: 12px 0; font-size: 13px; }
.window-option input { width: auto; }
.window-settings .window-error { color: #ef4444; }
</style>

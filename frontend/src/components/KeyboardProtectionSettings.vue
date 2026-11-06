<template>
  <section class="keyboard-settings" aria-label="键盘防护">
    <div class="keyboard-heading">键盘防护</div>
    <p class="keyboard-description">保护选中的 {{ modifier }} 及其组合键。另一侧可保留正常使用。</p>
    <label class="keyboard-option">
      <input type="checkbox" v-model="store.tempSettings.keyboardProtection.enabled" aria-label="启用键盘防护" />
      启用键盘防护
    </label>
    <div class="keyboard-sides">
      <label class="keyboard-option">
        <input type="checkbox" v-model="store.tempSettings.keyboardProtection.blockLeft" :disabled="!store.tempSettings.keyboardProtection.enabled" :aria-label="`屏蔽左 ${modifier}`" />
        屏蔽左 {{ modifier }}
      </label>
      <label class="keyboard-option">
        <input type="checkbox" v-model="store.tempSettings.keyboardProtection.blockRight" :disabled="!store.tempSettings.keyboardProtection.enabled" :aria-label="`屏蔽右 ${modifier}`" />
        屏蔽右 {{ modifier }}
      </label>
    </div>
    <p class="keyboard-description">点击保存后生效。在设置页或助手输入框编辑时自动暂停。</p>
    <p class="keyboard-status" role="status" data-testid="keyboard-status">{{ protectionStatus(store.keyboardStatus) }}</p>
    <p v-if="store.keyboardStatus.error" class="keyboard-error">{{ store.keyboardStatus.error }}</p>
    <div class="keyboard-actions">
      <button type="button" class="btn btn-ghost" @click="applyPreset">使用防护快捷键预设</button>
      <button v-if="store.isMacOS && !store.keyboardStatus.capabilities?.filtering" type="button" class="btn btn-ghost" @click="grantPermission">授权键盘防护</button>
      <button type="button" class="btn btn-ghost" @click="retry">重新检查</button>
    </div>
    <p v-if="uncovered.length" class="keyboard-description">当前防护未覆盖这些快捷键：{{ uncovered.join('、') }}。可修改组合或使用预设。</p>
  </section>
</template>

<script setup>
import { computed } from 'vue'
import { useSettingsStore } from '../stores/settings'
import { useUIStore } from '../stores/ui'
import { api } from '../services/api'
import { modifierFamily, protectionStatus } from '../utils/input-model'
const store = useSettingsStore()
const ui = useUIStore()
const modifier = computed(() => store.isMacOS ? 'Command' : 'Ctrl')
const uncovered = computed(() => store.shortcutActions.filter(action => {
  const keys = store.tempShortcuts[action.action]?.keys || []
  return !keys.some(key => modifierFamily(key) === (store.isMacOS ? 'Meta' : 'Control'))
}).map(action => action.label))
async function applyPreset() {
  try {
    const preset = await api.getProtectionShortcutPreset()
    for (const key of Object.keys(store.tempShortcuts)) delete store.tempShortcuts[key]
    Object.assign(store.tempShortcuts, preset)
    ui.showToast('预设已填入，保存后生效', 'info')
  } catch (error) { ui.showToast(error.message || '读取预设失败') }
}
async function grantPermission() {
  try { await api.requestKeyboardPermission(); await retry() } catch (error) { ui.showToast(error.message || '授权失败') }
}
async function retry() {
  try {
    const error = await api.retryKeyboardService()
    store.keyboardStatus = await api.getKeyboardProtectionStatus()
    if (error) ui.showToast(error)
  } catch (error) { ui.showToast(error.message || '检查失败') }
}
</script>

<style scoped>
.keyboard-settings { padding: 16px; border: 1px solid var(--border-default); border-radius: 12px; margin-bottom: 20px; }
.keyboard-heading { font-weight: 600; }
.keyboard-description { color: var(--text-secondary); font-size: 12px; line-height: 1.6; margin: 8px 0; }
.keyboard-option { display: flex; align-items: center; gap: 8px; font-size: 13px; }
.keyboard-option input { width: auto; accent-color: var(--accent); }
.keyboard-sides, .keyboard-actions { display: flex; flex-wrap: wrap; gap: 16px; margin-top: 12px; }
.keyboard-status { font-size: 12px; margin: 12px 0; }
.keyboard-error { color: var(--color-danger, #ef4444); font-size: 12px; }
</style>

import {
  CancelRunningTask,
  CheckScreenCapturePermission,
  ClearResume,
  GetDomainCategories,
  GetInitStatus,
  GetModels,
  GetScreenshotPreview,
  GetSettings,
  MoveWindow,
  OpenScreenCaptureSettings,
  ParseResume,
  RemoveFocus,
  RequestScreenCapturePermission,
  RestoreFocus,
  SaveImageToFile,
  ScrollContent,
  SelectResume,
  SetWindowAlwaysOnTop,
  StartRecordingKey,
  StopRecordingKey,
  TestConnection,
  ToggleClickThrough,
  ToggleVisibility,
  TriggerSolve,
  TriggerScreenshot,
  TriggerSend,
  RemoveScreenshot,
  ClearScreenshots,
  UpdateSettings,
} from '../../wailsjs/go/app/App'

import { Quit } from '../../wailsjs/runtime/runtime'

export const api = {

  getWindowStatus: () => window.go.app.App.GetWindowStatus(),
  setCaptureProtection: enabled => window.go.app.App.SetCaptureProtection(enabled),
  retryWindowPolicy: () => window.go.app.App.RetryWindowPolicy(),

  getKeyboardProtectionStatus: () => window.go.app.App.GetKeyboardProtectionStatus(),
  setInputEditing: (editing) => window.go.app.App.SetInputEditing(editing),
  requestKeyboardPermission: () => window.go.app.App.RequestKeyboardPermission(),
  retryKeyboardService: () => window.go.app.App.RetryKeyboardService(),
  getProtectionShortcutPreset: () => window.go.app.App.GetProtectionShortcutPreset(),
  getSettings: () => GetSettings(),
  syncSettings: (json) => UpdateSettings(json),
  updateSettings: (json) => UpdateSettings(json),

  getModels: (apiKey) => GetModels(apiKey),
  testConnection: (apiKey, model) => TestConnection(apiKey, model),

  triggerSolve: () => TriggerSolve(),
  triggerScreenshot: () => TriggerScreenshot(),
  triggerSend: () => TriggerSend(),
  removeScreenshot: (index) => RemoveScreenshot(index),
  clearScreenshots: () => ClearScreenshots(),
  cancelTask: () => CancelRunningTask(),
  cancelRequest: id => window.go.app.App.CancelRequest(id),
  getWorkflowStatus: () => window.go.app.App.GetWorkflowStatus(),

  startRecordingKey: (action) => StartRecordingKey(action),
  stopRecordingKey: () => StopRecordingKey(),

  selectResume: () => SelectResume(),
  clearResume: () => ClearResume(),
  parseResume: () => ParseResume(),

  restoreFocus: async () => { const error = await RestoreFocus(); if (error) throw new Error(error) },
  removeFocus: async () => { const error = await RemoveFocus(); if (error) throw new Error(error) },
  moveWindow: (x, y) => MoveWindow(x, y),
  scrollContent: (dir) => ScrollContent(dir),
  setAlwaysOnTop: async (v) => { const error = await SetWindowAlwaysOnTop(v); if (error) throw new Error(error) },
  toggleVisibility: () => ToggleVisibility(),
  toggleClickThrough: () => ToggleClickThrough(),
  quit: () => Quit(),

  getInitStatus: () => GetInitStatus(),
  getDomainCategories: () => GetDomainCategories(),

  getScreenshotPreview: (q, s, g, n, m) => GetScreenshotPreview(q, s, g, n, m),
  checkScreenCapturePermission: () => CheckScreenCapturePermission(),
  requestScreenCapturePermission: () => RequestScreenCapturePermission(),
  openScreenCaptureSettings: () => OpenScreenCaptureSettings(),

  saveImageToFile: (b64) => SaveImageToFile(b64),

  // ---- 历史记录持久化 ----
  // 通过 window.go.app.App.* 动态访问 Wails 绑定，避免 Go 后端尚未 rebuild 完成时
  // 静态 import fail 而炸掉整个 api 模块。
  // 后端方法：app/history.go::SaveHistorySnapshot / LoadHistorySnapshot / GetHistoryFilePath
  saveHistorySnapshot: (jsonStr) => {
    const fn = window?.go?.app?.App?.SaveHistorySnapshot
    if (typeof fn !== 'function') return Promise.resolve('binding not ready')
    return fn(jsonStr)
  },
  loadHistorySnapshot: () => {
    const fn = window?.go?.app?.App?.LoadHistorySnapshot
    if (typeof fn !== 'function') return Promise.resolve('')
    return fn()
  },
  getHistoryFilePath: () => {
    const fn = window?.go?.app?.App?.GetHistoryFilePath
    if (typeof fn !== 'function') return Promise.resolve('')
    return fn()
  },
}

export function captureProtectionStatus(status) {
  if (!status?.windows?.length) return '窗口策略尚未就绪'
  if (status.windows.some(window => window.error || !window.ready)) return '窗口策略尚未完全生效'
  if (status.captures > 0) return '截图中，临时保护窗口'
  if (!status.desired.captureProtection) return '捕获保护已关闭'
  if (status.windows.some(window => window.capture.mechanism === 'blank-content')) return '仅支持内容留空，窗口可能仍占位'
  return '系统保护属性已设置，实际效果需验证'
}
export function capturePolicyApplied(status) {
  return Boolean(status?.desired?.captureProtection && status.windows?.length && status.windows.every(window => window.ready && window.capture.applied))
}

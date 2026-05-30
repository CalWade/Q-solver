package app

import (
	"Q-Solver/pkg/logger"
)

// SaveHistorySnapshot 由前端 watcher 在 history 变更时调用，
// 把整个 history 数组的 JSON 串加密落盘。
//
// 返回值约定：
//   - "" 表示成功
//   - 非空字符串是错误描述，前端可记录但不应阻断 UI
//
// 后端不解析 JSON 结构——前端 history 形态演化时无需改后端。
// 空串视为"清空历史"，会删磁盘文件。
func (a *App) SaveHistorySnapshot(jsonStr string) string {
	if a.historyStore == nil {
		return "history store not initialized"
	}
	if err := a.historyStore.Save(jsonStr); err != nil {
		logger.Printf("[history] 保存失败: %v", err)
		return err.Error()
	}
	return ""
}

// LoadHistorySnapshot 启动时由前端 onMounted 调用一次。
// 文件不存在或解密失败时返回空串（前端把它当作"无历史"处理即可）。
func (a *App) LoadHistorySnapshot() string {
	if a.historyStore == nil {
		return ""
	}
	js, err := a.historyStore.Load()
	if err != nil {
		// 错误已在 Store 内 logger 输出+做了 corrupted 隔离，这里只返回空让前端开新会话。
		return ""
	}
	return js
}

// GetHistoryFilePath 暴露磁盘路径给前端展示（设置页/关于页可显示，便于用户备份）。
func (a *App) GetHistoryFilePath() string {
	if a.historyStore == nil {
		return ""
	}
	return a.historyStore.FilePath()
}

package app

import (
	"Q-Solver/pkg/broadcast"
	"Q-Solver/pkg/config"
	"Q-Solver/pkg/history"
	"Q-Solver/pkg/llm"
	"Q-Solver/pkg/logger"
	"Q-Solver/pkg/resume"
	"Q-Solver/pkg/screen"
	"Q-Solver/pkg/shortcut"
	"Q-Solver/pkg/solution"
	"Q-Solver/pkg/state"
	"Q-Solver/pkg/task"
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx context.Context

	configManager *config.ConfigManager
	stateManager  *state.StateManager
	taskManager   *task.TaskCoordinator

	llmService      *llm.Service
	resumeService   *resume.Service
	shortcutService *shortcut.Service
	screenService   *screen.Service
	solver          *solution.Solver
	broadcaster     *broadcast.Server
	historyStore    *history.Store
}

func NewApp() *App {
	configManager := config.NewConfigManager()

	return &App{
		configManager: configManager,
		stateManager:  state.NewStateManager(),
		taskManager:   task.NewTaskCoordinator(),
		screenService: screen.NewService(),
		broadcaster:   broadcast.NewServer(0),                // 使用默认端口 47821
		historyStore:  history.NewStore(config.SharedCipher), // 加密历史落盘
	}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	if err := a.configManager.Load(); err != nil {
		logger.Printf("加载配置失败: %v", err)
	}

	cfg := a.configManager.Get()
	if cfg.WindowWidth > 0 && cfg.WindowHeight > 0 {
		runtime.WindowSetSize(ctx, cfg.WindowWidth, cfg.WindowHeight)
		logger.Printf("应用保存的窗口尺寸: %dx%d", cfg.WindowWidth, cfg.WindowHeight)
	}

	a.stateManager.Startup(ctx, a.EmitEvent)
	a.screenService.Startup(ctx)
	a.screenService.SetCaptureGuard(a.stateManager)

	a.llmService = llm.NewService(a.configManager.Get(), a.configManager)
	a.solver = solution.NewSolver(a.llmService.GetProvider())
	a.resumeService = resume.NewService(a.configManager.Get(), a.configManager)

	a.shortcutService = shortcut.NewService(a)
	if err := a.shortcutService.Start(cfg.Shortcuts, cfg.KeyboardProtection); err != nil {
		logger.Printf("键盘服务未完全就绪: %v", err)
	}

	a.configManager.Subscribe(a.onConfigChanged)

	// 启动局域网答案广播 HTTP server（供 iPad 等副机实时查看答案）
	if a.broadcaster != nil {
		if err := a.broadcaster.Start(); err != nil {
			logger.Printf("启动广播服务失败: %v", err)
		} else {
			for _, addr := range a.broadcaster.LocalAddresses() {
				logger.Printf("答案广播地址: %s", addr)
			}
		}
	}

	ready := len(a.stateManager.GetWindowState().Windows) > 0
	for _, window := range a.stateManager.GetWindowState().Windows {
		ready = ready && window.Ready
	}
	if ready {
		a.stateManager.UpdateInitStatus(state.StatusReady)
	} else {
		a.stateManager.UpdateInitStatus(state.StatusError)
	}
}

func (a *App) onConfigChanged(newConfig config.Config, oldConfig config.Config) {
	if a.solver != nil {
		a.solver.SetProvider(a.llmService.GetProvider())
	}

	if !newConfig.KeepContext && a.solver != nil {
		a.solver.ClearHistory()
	}

	logger.Println("配置已更新并应用")
}

func (a *App) OnShutdown(ctx context.Context) {
	a.taskManager.CancelCurrentTask()
	if a.shortcutService != nil {
		a.shortcutService.Stop()
	}
	if a.broadcaster != nil {
		a.broadcaster.Stop()
	}
	a.stateManager.UnregisterWindow("main")
	if err := a.configManager.Save(); err != nil {
		logger.Printf("保存配置失败: %v", err)
	}
}

func (a *App) EmitEvent(eventName string, data ...interface{}) {
	runtime.EventsEmit(a.ctx, eventName, data...)
}

func (a *App) Show() {
	if err := a.stateManager.SetVisible(true); err != nil {
		logger.Printf("显示窗口失败: %v", err)
	}
}

// GetBroadcastAddresses 返回当前机器上所有可用于 iPad/Mac 副机访问的广播地址。
// 前端可以在"设置 / 关于"里展示这些 URL，让用户在 iPad Safari 打开其中一个。
func (a *App) GetBroadcastAddresses() []string {
	if a.broadcaster == nil {
		return []string{}
	}
	return a.broadcaster.LocalAddresses()
}

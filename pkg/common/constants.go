package common

const (
	// AppName 仅用于"存储身份"：配置文件目录、磁盘路径等内部用途。
	// 保留原值以避免老用户升级后配置丢失（配置位于 ~/Library/Application Support/Q-Solver/ 或 %AppData%\Q-Solver\）。
	// !! 不要改这个 !!
	AppName    = "Q-Solver"
	AppVersion = "1.0.0"

	// DisplayName 是"显示身份"：关于对话框、窗口标题、可执行文件名、bundle id 等用户/系统可见位置。
	// 用于降低进程扫描识别风险，可以和 AppName 不同。
	DisplayName = "Text Input Helper"
)

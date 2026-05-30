// Package history 提供前端答题历史的本地加密持久化存储。
//
// 设计原则：
//   - **零侵入业务流程**：仅暴露 Save/Load/Path 三个 API，前端在 history 变更时主动调 Save，
//     启动时主动调 Load。后端不感知 history 内部结构。
//   - **磁盘原子写**：先写临时文件再 rename，防止断电写到一半留下半截损坏的文件。
//   - **加密**：复用 config 包的 AES-256-GCM 密钥，避免明文截图 base64 + 答案散落在用户机器上。
//   - **失败恢复**：解密/解析失败时不删源文件，备份为 history.dat.corrupted.<ts>，
//     前端从空开始；用户能事后手动救。
//   - **大小上限**：前端落盘前会裁到最近 N 条，本包不约束，但日志里给提醒。
//   - **路径与现有 config 同位**：~/Library/Application Support/Q-Solver/ (macOS) 或
//     %AppData%\Q-Solver\ (Win)。这样升级/卸载/导出行为和 config 一致。
package history

import (
	"Q-Solver/pkg/common"
	"Q-Solver/pkg/logger"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// FileName 落盘文件名（区别于 config）
	FileName = "history.dat"
	// 写入提醒阈值：超过这个大小提示用户
	logSizeWarnBytes = 5 * 1024 * 1024 // 5MB
)

// Encryptor 抽象加解密——本包不直接依赖 config 包，
// 由 NewStore 调用方注入加解密实现，便于测试与解耦。
type Encryptor interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
}

// Store 是历史记录的本地加密存储。线程安全。
type Store struct {
	mu       sync.Mutex
	filePath string
	enc      Encryptor
}

// NewStore 创建 Store，路径与 config 同位。
// enc 必须非 nil；建议传入 config 包暴露的加解密对象。
func NewStore(enc Encryptor) *Store {
	return &Store{
		filePath: resolvePath(),
		enc:      enc,
	}
}

func resolvePath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	appDir := filepath.Join(dir, common.AppName)
	if err := os.MkdirAll(appDir, 0755); err != nil {
		logger.Printf("[history] 创建目录失败: %v", err)
	}
	return filepath.Join(appDir, FileName)
}

// FilePath 暴露磁盘路径，便于前端在"设置/关于"里展示，或导出。
func (s *Store) FilePath() string {
	return s.filePath
}

// Save 把任意 JSON 字符串加密落盘。
//
// 写入策略：
//   - 空串 → 删文件（视为"清空历史"）
//   - 非空 → 加密 → 写入临时文件 → rename 替换（原子写）
//
// 调用方应自行做 debounce / 大小裁剪。
func (s *Store) Save(jsonStr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if jsonStr == "" {
		if err := os.Remove(s.filePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("删除历史文件失败: %w", err)
		}
		logger.Println("[history] 已清空（文件已删除）")
		return nil
	}

	if s.enc == nil {
		return fmt.Errorf("encryptor 未初始化")
	}

	cipher, err := s.enc.Encrypt([]byte(jsonStr))
	if err != nil {
		return fmt.Errorf("加密失败: %w", err)
	}

	// 原子写：tmp + rename
	tmpPath := s.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, cipher, 0644); err != nil {
		return fmt.Errorf("写临时文件失败: %w", err)
	}
	if err := os.Rename(tmpPath, s.filePath); err != nil {
		// rename 失败时清理临时文件，避免污染
		_ = os.Remove(tmpPath)
		return fmt.Errorf("替换历史文件失败: %w", err)
	}

	if size := len(cipher); size > logSizeWarnBytes {
		logger.Printf("[history] 历史文件偏大 (%d 字节)，建议在前端裁剪", size)
	}
	return nil
}

// Load 读取磁盘上加密的历史 JSON 字符串。
//
// 返回值：
//   - 文件不存在 → ("", nil)（首次启动场景，不是错误）
//   - 解密/读取失败 → 备份损坏文件后返回 ("", err)；前端可视错误为空
//
// 损坏的文件会被改名为 history.dat.corrupted.<timestamp>，
// 用户事后可以手动 attempt 解密或丢弃。
func (s *Store) Load() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("读取历史文件失败: %w", err)
	}

	if s.enc == nil {
		return "", fmt.Errorf("encryptor 未初始化")
	}

	plain, err := s.enc.Decrypt(data)
	if err != nil {
		s.quarantineCorrupt()
		return "", fmt.Errorf("解密历史文件失败: %w", err)
	}

	logger.Printf("[history] 已从磁盘加载 %d 字节", len(plain))
	return string(plain), nil
}

// quarantineCorrupt 把损坏的 history.dat 改名隔离，避免再次启动时反复尝试解密同一个坏文件。
func (s *Store) quarantineCorrupt() {
	ts := time.Now().Format("20060102-150405")
	bak := fmt.Sprintf("%s.corrupted.%s", s.filePath, ts)
	if err := os.Rename(s.filePath, bak); err != nil {
		logger.Printf("[history] 隔离损坏文件失败: %v", err)
		return
	}
	logger.Printf("[history] 损坏文件已隔离到 %s", bak)
}

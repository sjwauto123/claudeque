package ssh

import (
	"fmt"
	"sync"
	"time"

	"cloudque/pkg/server"

	"go.uber.org/zap"
)

// Config SSH会话管理器配置
type Config struct {
	ServerHost     string        // 服务器地址，如 "192.168.1.100:22"
	RootUsername   string        // 管理员用户名（默认root）
	Timeout        time.Duration // 连接超时
	SessionTimeout time.Duration // 会话超时时间，0表示永不超时
}

// UserSession 用户SSH会话
type UserSession struct {
	Client     *server.Client // SSH客户端
	UserID     uint           // 用户ID
	Username   string         // SSH用户名
	CreatedAt  time.Time      // 创建时间
	LastUsedAt time.Time      // 最后使用时间
	IsRootUser bool           // 是否为管理员连接
	mu         sync.RWMutex   // 会话锁
}

// GetUsername 获取SSH用户名
func (s *UserSession) GetUsername() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Username
}

// IsRoot 是否为管理员连接
func (s *UserSession) IsRoot() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.IsRootUser
}

// UpdateLastUsed 更新最后使用时间
func (s *UserSession) UpdateLastUsed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastUsedAt = time.Now()
}

// Close 关闭SSH会话
func (s *UserSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Client != nil {
		return s.Client.Close()
	}
	return nil
}

// SessionManager SSH会话管理器
type SessionManager struct {
	cfg      *Config
	sessions map[uint]*UserSession // 用户ID到会话的映射
	mu       sync.RWMutex          // 会话映射锁
	logger   *zap.Logger
	stopChan chan struct{} // 停止通道
}

// GetRootUsername 获取Root用户名
func (sm *SessionManager) GetRootUsername() string {
	return sm.cfg.RootUsername
}

// NewSessionManager 创建SSH会话管理器
func NewSessionManager(cfg *Config, logger *zap.Logger) *SessionManager {
	return &SessionManager{
		cfg:      cfg,
		sessions: make(map[uint]*UserSession),
		logger:   logger,
		stopChan: make(chan struct{}),
	}
}

// CreateSession 为用户创建SSH会话
// userID: 用户ID
// username: 用户名（普通用户使用用户自己的用户名，管理员使用root）
// password: SSH密码
// isRoot: 是否使用管理员账户连接
func (sm *SessionManager) CreateSession(userID uint, username, password string, isRoot bool) (*UserSession, error) {
	// 构建SSH用户名
	sshUsername := username
	if isRoot {
		sshUsername = sm.cfg.RootUsername
	}

	// 创建SSH配置
	sshConfig := &server.Config{
		Host:     sm.cfg.ServerHost,
		Username: sshUsername,
		Password: password,
		Timeout:  sm.cfg.Timeout,
	}

	// 创建SSH客户端
	client, err := server.NewClient(sshConfig)
	if err != nil {
		return nil, fmt.Errorf("SSH连接失败: %w", err)
	}

	// 创建会话
	session := &UserSession{
		Client:     client,
		UserID:     userID,
		Username:   sshUsername,
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
		IsRootUser: isRoot,
	}

	// 保存会话
	sm.mu.Lock()
	sm.sessions[userID] = session
	sm.mu.Unlock()

	sm.logger.Info("创建SSH会话成功",
		zap.Uint("user_id", userID),
		zap.String("ssh_username", sshUsername),
		zap.Bool("is_root", isRoot),
	)

	return session, nil
}

// AddSession 添加已存在的SSH会话（用于复用已验证的连接）
func (sm *SessionManager) AddSession(userID uint, session *UserSession) {
	sm.mu.Lock()
	sm.sessions[userID] = session
	sm.mu.Unlock()

	sm.logger.Info("添加SSH会话成功",
		zap.Uint("user_id", userID),
		zap.String("ssh_username", session.Username),
		zap.Bool("is_root", session.IsRootUser),
	)
}

// GetSession 获取用户的SSH会话
func (sm *SessionManager) GetSession(userID uint) (*UserSession, error) {
	sm.mu.RLock()
	session, exists := sm.sessions[userID]
	sm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("SSH会话不存在")
	}

	// 更新最后使用时间
	session.UpdateLastUsed()

	return session, nil
}

// DeleteSession 删除用户的SSH会话
func (sm *SessionManager) DeleteSession(userID uint) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[userID]
	if !exists {
		return nil
	}

	// 关闭SSH连接
	if err := session.Close(); err != nil {
		sm.logger.Warn("关闭SSH会话失败",
			zap.Uint("user_id", userID),
			zap.Error(err),
		)
	}

	delete(sm.sessions, userID)

	sm.logger.Info("删除SSH会话成功", zap.Uint("user_id", userID))
	return nil
}

// HasSession 检查用户是否有SSH会话
func (sm *SessionManager) HasSession(userID uint) bool {
	sm.mu.RLock()
	_, exists := sm.sessions[userID]
	sm.mu.RUnlock()
	return exists
}

// GetActiveSessionCount 获取活跃会话数量
func (sm *SessionManager) GetActiveSessionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessions)
}

// GetAllSessions 获取所有会话信息（用于监控）
func (sm *SessionManager) GetAllSessions() []map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessions := make([]map[string]interface{}, 0, len(sm.sessions))
	for _, session := range sm.sessions {
		session.mu.RLock()
		sessions = append(sessions, map[string]interface{}{
			"user_id":      session.UserID,
			"ssh_username": session.Username,
			"is_root":      session.IsRootUser,
			"created_at":   session.CreatedAt,
			"last_used_at": session.LastUsedAt,
		})
		session.mu.RUnlock()
	}
	return sessions
}

// StartCleanupTimer 启动会话超时清理定时器
func (sm *SessionManager) StartCleanupTimer() {
	if sm.cfg.SessionTimeout <= 0 {
		return
	}

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sm.cleanupExpiredSessions()
		case <-sm.stopChan:
			return
		}
	}
}

// cleanupExpiredSessions 清理过期会话
func (sm *SessionManager) cleanupExpiredSessions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	for userID, session := range sm.sessions {
		session.mu.RLock()
		if now.Sub(session.LastUsedAt) > sm.cfg.SessionTimeout {
			sm.logger.Info("清理过期SSH会话",
				zap.Uint("user_id", userID),
				zap.Duration("idle_time", now.Sub(session.LastUsedAt)),
			)
			session.mu.RUnlock()
			_ = session.Close()
			delete(sm.sessions, userID)
		} else {
			session.mu.RUnlock()
		}
	}
}

// CloseAll 关闭所有SSH会话
func (sm *SessionManager) CloseAll() {
	// 停止清理定时器
	close(sm.stopChan)

	sm.mu.Lock()
	defer sm.mu.Unlock()

	for userID, session := range sm.sessions {
		if err := session.Close(); err != nil {
			sm.logger.Warn("关闭SSH会话失败",
				zap.Uint("user_id", userID),
				zap.Error(err),
			)
		}
	}

	sm.sessions = make(map[uint]*UserSession)
	sm.logger.Info("所有SSH会话已关闭")
}

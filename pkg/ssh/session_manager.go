package ssh

import (
	"fmt"
	"log"
	"sync"
	"time"

	"cloudque/pkg/server"

	"go.uber.org/zap"
)

// Config SSH会话管理器配置
type Config struct {
	ServerHost           string        // 服务器地址
	RootUsername         string        // 管理员用户名（默认root）
	RootPassword         string        // 管理员密码
	PrivateKeyPath       string        // 私钥文件路径
	PrivateKeyPassphrase string        // 私钥密码
	Timeout              time.Duration // 连接超时
	SessionTimeout       time.Duration // 会话超时时间，0表示永不超时
}

// UserSession 用户SSH会话
type UserSession struct {
	Client     *server.Client // SSH客户端
	UserID     int            // 用户ID
	Username   string         // SSH用户名
	Password   string         // 密码（用于自动重连）
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
	Cfg      *Config
	sessions map[string]*UserSession // 用户ID+身份到会话的映射，key格式: "userID:isRoot"
	mu       sync.RWMutex            // 会话映射锁
	logger   *zap.Logger
	stopChan chan struct{} // 停止通道
}

// getSessionKey 生成会话唯一标识
func (sm *SessionManager) getSessionKey(userID int, isRoot bool) string {
	return fmt.Sprintf("%d:%v", userID, isRoot)
}

// GetRootUsername 获取Root用户名
func (sm *SessionManager) GetRootUsername() string {
	if sm.Cfg != nil && sm.Cfg.RootUsername != "" {
		return sm.Cfg.RootUsername
	}
	return "root" // 默认值
}

// SetTimeout 设置SSH连接超时
func (sm *SessionManager) SetTimeout(timeout time.Duration) {
	if sm.Cfg != nil {
		sm.Cfg.Timeout = timeout
	}
}

// GetTimeout 获取SSH连接超时
func (sm *SessionManager) GetTimeout() time.Duration {
	if sm.Cfg != nil && sm.Cfg.Timeout > 0 {
		return sm.Cfg.Timeout
	}
	return 30 * time.Second // 默认值
}

// NewSessionManager 创建SSH会话管理器
func NewSessionManager(cfg *Config, logger *zap.Logger) *SessionManager {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &SessionManager{
		Cfg:      cfg,
		sessions: make(map[string]*UserSession),
		logger:   logger,
		stopChan: make(chan struct{}),
	}
}

// CreateSession 为用户创建SSH会话
// userID: 用户ID
// username: 用户名（普通用户使用用户自己的用户名，管理员使用root）
// password: SSH密码
// isRoot: 是否使用管理员账户连接
func (sm *SessionManager) CreateSession(userID int, username, password string, isRoot bool) (*UserSession, error) {
	// 构建SSH用户名
	sshUsername := username
	if isRoot {
		sshUsername = sm.GetRootUsername()
	}

	// 创建SSH配置
	sshConfig := &server.Config{
		Host:     sm.Cfg.ServerHost,
		Username: sshUsername,
		Password: password,
		Timeout:  sm.GetTimeout(),
	}

	// 只有当是root用户时，才添加私钥路径和私钥密码
	if isRoot {
		sshConfig.PrivateKeyPath = sm.Cfg.PrivateKeyPath
		sshConfig.PrivateKeyPassphrase = sm.Cfg.PrivateKeyPassphrase
	}

	// 如果是root用户且配置了root密码，则优先使用配置的root密码
	if isRoot && sm.Cfg.RootPassword != "" {
		sshConfig.Password = sm.Cfg.RootPassword
	}

	// 创建SSH客户端
	client, err := server.NewClient(sshConfig)
	if err != nil {
		sm.logger.Warn("创建SSH会话失败",
			zap.Int("user_id", userID),
			zap.String("ssh_username", sshUsername),
			zap.Bool("is_root", isRoot),
			zap.Error(err),
		)
		return nil, fmt.Errorf("SSH连接失败: %w", err)
	}

	// 创建会话
	session := &UserSession{
		Client:     client,
		UserID:     userID,
		Username:   sshUsername,
		Password:   password, // 保存密码用于自动重连
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
		IsRootUser: isRoot,
	}

	// 保存会话
	sm.mu.Lock()
	key := sm.getSessionKey(userID, isRoot)
	// 如果旧会话存在，先关闭
	if old, ok := sm.sessions[key]; ok {
		_ = old.Close()
	}
	sm.sessions[key] = session
	sm.mu.Unlock()

	sm.logger.Info("创建SSH会话成功",
		zap.Int("user_id", userID),
		zap.String("ssh_username", sshUsername),
		zap.Bool("is_root", isRoot),
	)

	return session, nil
}

// AddSession 添加已存在的SSH会话（用于复用已验证的连接）
func (sm *SessionManager) AddSession(userID int, session *UserSession) {
	sm.mu.Lock()
	key := sm.getSessionKey(userID, session.IsRootUser)
	// 如果存在旧会话，先关闭
	if old, ok := sm.sessions[key]; ok {
		go func() {
			err := old.Close()
			if err != nil {
				log.Printf("无法关闭旧的ssh会话")
			}
		}()
	}
	sm.sessions[key] = session
	sm.mu.Unlock()

	sm.logger.Info("添加SSH会话成功",
		zap.Int("user_id", userID),
		zap.String("ssh_username", session.Username),
		zap.Bool("is_root", session.IsRootUser),
	)
}

// GetSession 获取用户的SSH会话
func (sm *SessionManager) GetSession(userID int, isRoot bool) (*UserSession, error) {
	sm.mu.RLock()
	key := sm.getSessionKey(userID, isRoot)
	session, exists := sm.sessions[key]
	sm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("SSH会话不存在")
	}

	// 更新最后使用时间
	session.UpdateLastUsed()

	return session, nil
}

// GetOrCreateSession 获取会话，如果不存在则使用提供的凭据创建
func (sm *SessionManager) GetOrCreateSession(userID int, username, password string, isRoot bool) (*UserSession, error) {
	// 先尝试获取现有会话
	session, err := sm.GetSession(userID, isRoot)
	if err == nil {
		// 验证连接是否仍然有效
		if sm.isSessionValid(session) {
			return session, nil
		}
		// 无效则删除
		err := sm.DeleteSession(userID, isRoot)
		if err != nil {
			return nil, err
		}
	}

	// 创建新会话
	return sm.CreateSession(userID, username, password, isRoot)
}

// isSessionValid 检查会话是否有效
func (sm *SessionManager) isSessionValid(s *UserSession) bool {
	s.mu.RLock()
	client := s.Client
	s.mu.RUnlock()

	if client == nil {
		return false
	}

	// 通过执行简单命令验证连接
	_, err := client.ExecuteCommand("echo 1")
	return err == nil
}

// GetSessionWithFallback 获取会话，支持自动降级（root失败尝试普通用户）
func (sm *SessionManager) GetSessionWithFallback(userID int, preferRoot bool) (*UserSession, error) {
	// 优先尝试请求的权限级别
	session, err := sm.GetSession(userID, preferRoot)
	if err == nil {
		return session, nil
	}

	// 如果请求root失败，尝试普通用户（反之亦然）
	fallbackRoot := !preferRoot
	session, err2 := sm.GetSession(userID, fallbackRoot)
	if err2 == nil {
		sm.logger.Warn("使用降级权限获取会话",
			zap.Int("user_id", userID),
			zap.Bool("requested_root", preferRoot),
			zap.Bool("actual_root", fallbackRoot),
		)
		return session, nil
	}

	return nil, fmt.Errorf("未找到任何有效会话（root=%v 或 root=%v）", preferRoot, fallbackRoot)
}

// DeleteSession 删除会话
func (sm *SessionManager) DeleteSession(userID int, isRoot bool) error {
	key := sm.getSessionKey(userID, isRoot)

	sm.mu.Lock()
	session, exists := sm.sessions[key]
	if exists {
		delete(sm.sessions, key)
	}
	sm.mu.Unlock()

	if !exists {
		return nil
	}

	if err := session.Close(); err != nil {
		sm.logger.Warn("关闭SSH会话失败", zap.Int("user_id", userID), zap.Error(err))
	}

	sm.logger.Info("SSH会话删除成功", zap.Int("user_id", userID), zap.Bool("is_root", isRoot))
	return nil
}

// HasSession 检查会话是否存在
func (sm *SessionManager) HasSession(userID int, isRoot bool) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	_, exists := sm.sessions[sm.getSessionKey(userID, isRoot)]
	return exists
}

// GetActiveSessionCount 获取活跃会话数
func (sm *SessionManager) GetActiveSessionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessions)
}

// StartCleanupTimer 启动清理定时器
func (sm *SessionManager) StartCleanupTimer() {
	if sm.Cfg.SessionTimeout <= 0 {
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
	for key, session := range sm.sessions {
		session.mu.RLock()
		lastUsed := session.LastUsedAt
		session.mu.RUnlock()

		if now.Sub(lastUsed) > sm.Cfg.SessionTimeout {
			sm.logger.Info("清理过期SSH会话",
				zap.String("key", key),
				zap.Duration("idle", now.Sub(lastUsed)),
			)
			go func() {
				err := session.Close()
				if err != nil {
					log.Println("清除会话失败", err)
				}
			}()
			delete(sm.sessions, key)
		}
	}
}

// CloseAll 关闭所有会话
func (sm *SessionManager) CloseAll() {
	close(sm.stopChan)

	sm.mu.Lock()
	sessions := make([]*UserSession, 0, len(sm.sessions))
	for _, s := range sm.sessions {
		sessions = append(sessions, s)
	}
	sm.sessions = make(map[string]*UserSession)
	sm.mu.Unlock()

	for _, s := range sessions {
		if err := s.Close(); err != nil {
			sm.logger.Warn("关闭会话失败", zap.Error(err))
		}
	}
}

package service

import (
	"cloudque/internal/model/dto/request"
	dto "cloudque/internal/model/dto/response"
	"cloudque/internal/repository"
	bizerrors "cloudque/pkg/errors"
	"cloudque/pkg/jwt"
	"cloudque/pkg/logger"
	"cloudque/pkg/server"
	"cloudque/pkg/ssh"
	"fmt"
	"os/user"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

const (
	// RoleAdmin 管理员角色
	RoleAdmin = 2
	// RoleUser 普通用户角色
	RoleUser = 1
)

// authService 认证服务实现
type authService struct {
	userRepo       repository.UserRepository
	userService    UserService
	sessionRepo    repository.SessionRepository
	sessionManager *ssh.SessionManager
	redisClient    interface{} // Redis客户端（用于存储会话凭证）
	sshServerHost  string      // SSH服务器地址
}

// NewAuthService 创建认证服务
func NewAuthService(userRepo repository.UserRepository, userService UserService, sessionRepo repository.SessionRepository, sessionManager *ssh.SessionManager) AuthService {
	return &authService{
		userRepo:       userRepo,
		userService:    userService,
		sessionRepo:    sessionRepo,
		sessionManager: sessionManager,
	}
}

// SetRedisClient 设置Redis客户端
func (s *authService) SetRedisClient(redisClient interface{}) {
	s.redisClient = redisClient
}

// SetSSHServerHost 设置SSH服务器地址
func (s *authService) SetSSHServerHost(host string) {
	s.sshServerHost = host
}

// verifySSHCredentials 通过SSH验证用户凭证
func (s *authService) verifySSHCredentials(username, password string) (*server.Client, error) {
	// 如果未配置SSH服务器，跳过验证
	if s.sessionManager == nil {
		logger.Warn("SSH会话管理器未配置，跳过SSH验证")
		return nil, nil
	}

	// 获取服务器地址
	serverHost := s.sshServerHost
	if serverHost == "" {
		// 默认使用 localhost:22
		serverHost = "localhost:22"
	}

	// 尝试连接SSH服务器验证凭证
	sshConfig := &server.Config{
		Host:     serverHost,
		Username: username,
		Password: password,
		Timeout:  10 * time.Second,
	}

	client, err := server.NewClient(sshConfig)
	if err != nil {
		logger.Warn("SSH验证失败",
			zap.String("username", username),
			zap.String("server", serverHost),
			zap.Error(err),
		)
		return nil, fmt.Errorf("SSH验证失败: %w", err)
	}

	logger.Info("SSH验证成功", zap.String("username", username))
	return client, nil
}

// getSystemUserInfo 获取系统用户信息（HomeDir）
func (s *authService) getSystemUserInfo(username string, sshClient *server.Client) (homeDir string, err error) {
	// 如果有SSH客户端，通过远程命令获取
	if sshClient != nil {
		// 使用 getent passwd 命令获取用户信息
		output, err := sshClient.ExecuteCommand(fmt.Sprintf("getent passwd %s", username))
		if err != nil {
			logger.Warn("获取远程系统用户信息失败", zap.String("username", username), zap.Error(err))
			return "", nil // 不阻塞登录流程
		}

		// 解析输出
		// getent passwd 输出格式: username:x:uid:gid:gecos:home:shell
		parts := strings.Split(strings.TrimSpace(output), ":")
		if len(parts) >= 6 {
			homeDir = parts[5]
		}
		logger.Info("获取远程系统用户信息成功",
			zap.String("username", username),
			zap.String("home_dir", homeDir),
		)
		return homeDir, nil
	}

	// 如果服务运行在本地，使用 os/user 包获取
	u, err := user.Lookup(username)
	if err != nil {
		logger.Warn("获取本地系统用户信息失败", zap.String("username", username), zap.Error(err))
		return "", nil // 不阻塞登录流程
	}

	homeDir = u.HomeDir

	logger.Info("获取本地系统用户信息成功",
		zap.String("username", username),
		zap.String("home_dir", homeDir),
	)
	return homeDir, nil
}

// saveUserCredentialsToRedis 将用户凭证存储到Redis
func (s *authService) saveUserCredentialsToRedis(userID uint, username, password string) error {
	if s.sessionRepo == nil {
		logger.Warn("SessionRepository未配置，跳过凭证存储")
		return nil
	}

	// 保存凭证，设置30分钟过期
	expiresAt := time.Now().Add(30 * time.Minute)
	if err := s.sessionRepo.SaveUserCredentials(userID, username, password, expiresAt); err != nil {
		logger.Warn("存储用户凭证到Redis失败", zap.Error(err))
		return err
	}

	logger.Info("用户凭证已存储到Redis",
		zap.Uint("user_id", userID),
		zap.Time("expires_at", expiresAt),
	)
	return nil
}

// Login 用户登录
func (s *authService) Login(req *request.LoginRequest) (*dto.LoginResponse, error) {
	// 查找用户
	user, err := s.userRepo.FindByUsername(req.Username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, bizerrors.ErrInvalidCredentials
	}

	// 检查用户状态
	if user.Status != 1 {
		return nil, bizerrors.ErrUserDisabled
	}

	// SSH 拨号验证 - 确保Web账号与系统账号同步
	var sshClient *server.Client
	if s.sessionManager != nil {
		// 根据角色确定SSH用户名
		sshUser := req.Username
		if user.Role == RoleAdmin {
			sshUser = s.sessionManager.GetRootUsername()
		}

		sshClient, err = s.verifySSHCredentials(sshUser, req.Password)
		if err != nil && s.sshServerHost != "" {
			// 如果配置了SSH服务器但验证失败，拒绝登录
			logger.Error("SSH验证失败，拒绝登录",
				zap.String("username", sshUser),
				zap.Error(err),
			)
			return nil, bizerrors.ErrInvalidCredentials
		}
	}

	// 验证数据库密码 (如果SSH验证通过，则跳过DB密码验证并同步密码；否则必须验证DB密码)
	if sshClient != nil {
		// SSH验证通过，同步密码到数据库（如果不同）
		// 注意：这里我们信任SSH验证的结果
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err == nil {
			// 简单比较哈希值是不行的，因为每次生成的盐不同
			// 我们直接更新密码，或者先检查是否匹配
			if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
				// 密码不匹配，更新为新密码
				user.Password = string(hashedPassword)
				if err := s.userRepo.Update(user); err != nil {
					logger.Warn("同步用户密码失败", zap.Error(err))
				} else {
					logger.Info("用户密码已同步为SSH密码", zap.String("username", req.Username))
				}
			}
		}
	} else {
		// 未进行SSH验证（例如未配置SSH Host），必须验证DB密码
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
			return nil, bizerrors.ErrInvalidCredentials
		}
	}

	// 获取系统用户信息（HomeDir）
	homeDir, _ := s.getSystemUserInfo(req.Username, sshClient)
	if homeDir != "" {
		user.HomeDir = &homeDir
	}

	// 更新用户系统信息
	if homeDir != "" {
		if err := s.userRepo.Update(user); err != nil {
			logger.Warn("更新用户系统信息失败", zap.Error(err))
		}
	}

	// 将用户凭证存入Redis（供终端模块重连使用）
	if err := s.saveUserCredentialsToRedis(user.ID, req.Username, req.Password); err != nil {
		logger.Warn("存储用户凭证到Redis失败", zap.Error(err))
	}

	// 生成 Token
	token, err := jwt.GenerateToken(user.ID, user.Username)
	if err != nil {
		return nil, err
	}

	// 创建SSH会话（如果会话管理器已启用）
	if s.sessionManager != nil {
		// 判断用户角色：管理员使用root账号，普通用户使用自己的账号
		isRoot := user.Role == RoleAdmin

		// 如果已经通过SSH验证创建了客户端，复用；否则重新创建
		if sshClient == nil {
			session, err := s.sessionManager.CreateSession(user.ID, user.Username, req.Password, isRoot)
			if err != nil {
				logger.Warn("SSH会话创建失败，文件功能将受限",
					zap.Uint("user_id", user.ID),
					zap.String("username", user.Username),
					zap.Bool("is_root", isRoot),
					zap.Error(err),
				)
			} else {
				// 保存会话元数据到仓库
				if s.sessionRepo != nil {
					_ = s.sessionRepo.SaveSession(user.ID, session.GetUsername(), session.IsRoot(), session.CreatedAt)
				}
			}
		} else {
			// 使用已验证的SSH客户端创建会话
			session := &ssh.UserSession{
				Client:     sshClient,
				UserID:     user.ID,
				Username:   req.Username,
				CreatedAt:  time.Now(),
				LastUsedAt: time.Now(),
				IsRootUser: isRoot,
			}
			// 将session添加到SessionManager中
			s.sessionManager.AddSession(user.ID, session)
			logger.Info("SSH会话已创建（复用验证连接）",
				zap.Uint("user_id", user.ID),
				zap.String("username", req.Username),
				zap.Bool("is_root", isRoot),
			)
			// 保存会话元数据到仓库
			if s.sessionRepo != nil {
				_ = s.sessionRepo.SaveSession(user.ID, session.GetUsername(), session.IsRoot(), session.CreatedAt)
			}
		}
	}

	// 构建响应
	userResp := s.userService.GetUserResponse(user)
	return &dto.LoginResponse{
		Token: token,
		User:  *userResp,
	}, nil
}

// EnsureSSHSession 确保用户的SSH会话存在
func (s *authService) EnsureSSHSession(userID uint) error {
	if s.sessionManager == nil {
		return fmt.Errorf("SSH会话管理器未配置")
	}

	// 1. 检查会话是否已存在
	if s.sessionManager.HasSession(userID) {
		return nil
	}

	// 2. 获取用户凭证
	creds, err := s.GetUserCredentialsFromRedis(userID)
	if err != nil {
		return fmt.Errorf("无法恢复SSH会话，请重新登录: %v", err)
	}

	// 3. 获取用户信息（需要角色）
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return fmt.Errorf("获取用户信息失败: %v", err)
	}
	if user == nil {
		return fmt.Errorf("用户不存在")
	}

	// 4. 创建SSH会话
	isRoot := user.Role == RoleAdmin
	session, err := s.sessionManager.CreateSession(user.ID, user.Username, creds.Password, isRoot)
	if err != nil {
		return fmt.Errorf("创建SSH会话失败: %v", err)
	}

	// 5. 保存会话元数据
	if s.sessionRepo != nil {
		_ = s.sessionRepo.SaveSession(user.ID, session.GetUsername(), session.IsRoot(), session.CreatedAt)
	}

	logger.Info("SSH会话已恢复",
		zap.Uint("user_id", userID),
		zap.String("username", user.Username),
		zap.Bool("is_root", isRoot),
	)

	return nil
}

// Logout 用户登出
func (s *authService) Logout(userID uint) error {
	// 删除Redis中的凭证
	if s.sessionRepo != nil {
		if err := s.sessionRepo.DeleteUserCredentials(userID); err != nil {
			logger.Warn("删除用户凭证失败", zap.Uint("user_id", userID), zap.Error(err))
		} else {
			logger.Info("已清除Redis中的用户凭证", zap.Uint("user_id", userID))
		}
	}

	// 删除SSH会话
	if s.sessionManager != nil {
		if err := s.sessionManager.DeleteSession(userID); err != nil {
			logger.Warn("删除SSH会话失败", zap.Uint("user_id", userID), zap.Error(err))
		}
	}

	// 删除会话元数据
	if s.sessionRepo != nil {
		_ = s.sessionRepo.DeleteSession(userID)
	}

	logger.Info("用户登出成功", zap.Uint("user_id", userID))
	return nil
}

// RefreshToken 刷新 Token
func (s *authService) RefreshToken(token string) (string, error) {
	newToken, err := jwt.RefreshToken(token)
	if err != nil {
		return "", err
	}
	return newToken, nil
}

// RedisCredentials Redis中存储的用户凭证结构
type RedisCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Expires  int64  `json:"expires"`
}

// GetUserCredentialsFromRedis 从Redis获取用户凭证
func (s *authService) GetUserCredentialsFromRedis(userID uint) (*RedisCredentials, error) {
	if s.sessionRepo == nil {
		return nil, fmt.Errorf("SessionRepository未配置")
	}

	creds, err := s.sessionRepo.GetUserCredentials(userID)
	if err != nil {
		return nil, fmt.Errorf("获取用户凭证失败: %w", err)
	}

	return &RedisCredentials{
		Username: creds.Username,
		Password: creds.Password,
		Expires:  creds.ExpiresAt.Unix(),
	}, nil
}

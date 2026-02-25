package service

import (
	"cloudque/internal/model/dto/request"
	dto "cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"cloudque/internal/repository"
	"cloudque/pkg/captcha"
	"cloudque/pkg/email"
	bizerrors "cloudque/pkg/errors"
	"cloudque/pkg/jwt"
	"cloudque/pkg/logger"
	"cloudque/pkg/server"
	"cloudque/pkg/ssh"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"time"

	"go.uber.org/zap"

	"golang.org/x/crypto/bcrypt"
)

const PermissionRootSSH = "ssh:root" // 定义SSH root权限slug

// sshCredentials 用于存储用户的SSH凭证
type sshCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// authService 认证服务实现
type authService struct {
	userRepo       repository.UserRepository
	roleRepo       repository.RoleRepository
	redisRepo      repository.RedisRepository
	userService    UserService
	sessionRepo    repository.SessionRepository
	sessionManager *ssh.SessionManager
	sshServerHost  string // SSH服务器地址
}

// NewAuthService 创建认证服务
func NewAuthService(
	userRepo repository.UserRepository,
	roleRepo repository.RoleRepository,
	redisRepo repository.RedisRepository,
	userService UserService,
	sessionRepo repository.SessionRepository,
	sessionManager *ssh.SessionManager,
	sshConfig *ssh.Config,
) AuthService {
	return &authService{
		userRepo:       userRepo,
		roleRepo:       roleRepo,
		redisRepo:      redisRepo,
		userService:    userService,
		sessionRepo:    sessionRepo,
		sessionManager: sessionManager,
		sshServerHost:  sshConfig.ServerHost,
	}
}

// userHasPermission 检查用户是否拥有特定权限
func (s *authService) userHasPermission(user *entity.User, permissionSlug string) bool {
	for _, role := range user.Roles {
		if role.Status != 1 {
			continue
		}
		// 从数据库加载角色的权限
		fullRole, err := s.roleRepo.FindBySlug(role.Slug)
		if err != nil || fullRole == nil {
			// 记录错误，但继续检查其他角色
			logger.Warn("加载角色权限失败", zap.String("role_slug", role.Slug), zap.Error(err))
			continue
		}
		for _, perm := range fullRole.Permissions {
			if perm.Slug == permissionSlug && perm.Status == 1 {
				return true
			}
		}
	}
	return false
}

// Login 用户登录
func (s *authService) Login(req *request.LoginRequest) (*dto.LoginResponse, error) {
	// 校验验证码
	if !captcha.Verify(req.CaptchaID, req.Captcha) {
		return nil, bizerrors.ErrInvalidCaptcha
	}

	// 查找用户并校验存在性
	user, err := s.userRepo.FindByUsername(req.Username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, bizerrors.ErrInvalidCredentials
	}

	// 检查用户状态是否启用
	if user.Status != 1 {
		return nil, bizerrors.ErrUserDisabled
	}

	///////////////////////////////////////验证对应的SSH是否可以连接成功//////////////////////////////////////////////////
	// SSH 拨号验证 - 确保Web账号与系统账号同步
	var sshClient *server.Client
	if s.sessionManager != nil {
		sshUser := req.Username

		if s.userHasPermission(user, PermissionRootSSH) {
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

	// 将用户凭证存入Redis（供终端模块重连使用）
	if err := s.saveUserCredentialsToRedis(user.ID, req.Username, req.Password); err != nil {
		logger.Warn("存储用户凭证到Redis失败", zap.Error(err))
	}

	// 构建角色列表
	roles := s.buildRoles(user)
	// 聚合权限和菜单
	permsDTO, menus := s.aggregate(user)
	// 构建菜单树
	menuNodes := buildMenuTree(menus)

	// 生成 JWT
	token, err := jwt.GenerateToken(user.ID, user.Username, roles)
	if err != nil {
		return nil, err
	}

	//////////////////////////建立SSH会话/////////////////////

	// 创建SSH会话（如果会话管理器已启用）
	if s.sessionManager != nil {

		// 判断用户是否拥有SSH root权限：拥有则使用root账号，否则使用自己的账号
		isRoot := s.userHasPermission(user, PermissionRootSSH)

		// 如果已经通过SSH验证创建了客户端，复用；否则重新创建
		if sshClient == nil {
			session, err := s.sessionManager.CreateSession(user.ID, user.Username, req.Password, isRoot)
			if err != nil {
				logger.Warn("SSH会话创建失败，文件功能将受限",
					zap.Int("user_id", user.ID),
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
				zap.Int("user_id", user.ID),
				zap.String("username", req.Username),
				zap.Bool("is_root", isRoot),
			)
			// 保存会话元数据到仓库
			if s.sessionRepo != nil {
				_ = s.sessionRepo.SaveSession(user.ID, session.GetUsername(), session.IsRoot(), session.CreatedAt)
			}
		}
	}

	// 组装用户信息
	// 构建响应
	userResp := s.userService.GetUserResponse(user)
	// 返回登录响应数据
	return &dto.LoginResponse{
		Token:       token,
		User:        *userResp,
		Permissions: permsDTO,
		MenusTree:   menuNodes,
	}, nil
}

// 聚合角色
func (s *authService) buildRoles(user *entity.User) []string {
	roles := make([]string, 0, len(user.Roles))
	for _, r := range user.Roles {
		if r.Status == 1 {
			roles = append(roles, r.Slug)
		}
	}
	return roles
}

// 将权限和菜单聚合到map中，并以固定形式返回
func (s *authService) aggregate(user *entity.User) ([]dto.PermissionResponse, []entity.Menu) {
	permMap := make(map[int]entity.Permission)
	menuMap := make(map[int]entity.Menu)
	for _, r := range user.Roles {
		if r.Status != 1 {
			continue
		}
		role, err := s.roleRepo.FindBySlug(r.Slug)
		if err != nil || role == nil {
			continue
		}
		for _, p := range role.Permissions {
			if p.Status == 1 {
				permMap[p.ID] = p
			}
		}
		for _, m := range role.Menus {
			if m.Status == 1 {
				menuMap[m.ID] = m
			}
		}
	}
	permsDTO := make([]dto.PermissionResponse, 0, len(permMap))
	for _, p := range permMap {
		permsDTO = append(permsDTO, dto.PermissionResponse{
			ID:         p.ID,
			Name:       p.Name,
			Category:   p.Category,
			Slug:       p.Slug,
			Type:       p.Type,
			Status:     p.Status,
			HttpMethod: p.HttpMethod,
			HttpPath:   p.HttpPath,
		})
	}
	menus := make([]entity.Menu, 0, len(menuMap))
	for _, m := range menuMap {
		menus = append(menus, m)
	}
	return permsDTO, menus
}

// 构建菜单树
func buildMenuTree(menus []entity.Menu) []dto.MenuTreeNode {
	nodeMap := make(map[int]*dto.MenuTreeNode)
	parentChildren := make(map[int][]*dto.MenuTreeNode)
	for _, m := range menus {
		nodeMap[m.ID] = &dto.MenuTreeNode{
			ID:       m.ID,
			ParentID: m.ParentID,
			Title:    m.Title,
			Status:   m.Status,
			Type:     m.Type,
			Icon:     m.Icon,
			URI:      m.URI,
			Sort:     m.Sort,
		}
	}
	for _, n := range nodeMap {
		if n.ParentID != 0 {
			parentChildren[n.ParentID] = append(parentChildren[n.ParentID], n)
		}
	}
	menuNodes := make([]dto.MenuTreeNode, 0)
	for id, n := range nodeMap {
		if ch, ok := parentChildren[id]; ok {
			// 子节点排序
			sort.Slice(ch, func(i, j int) bool {
				if ch[i].Sort == ch[j].Sort {
					return ch[i].Title < ch[j].Title
				}
				return ch[i].Sort < ch[j].Sort
			})
			n.Children = ch
		}
		// 如果是根节点（ParentID == 0）或者没有父节点（nodeMap[n.ParentID] == nil）
		if n.ParentID == 0 || nodeMap[n.ParentID] == nil {
			menuNodes = append(menuNodes, *n)
		}
	}
	// 根节点排序
	sort.Slice(menuNodes, func(i, j int) bool {
		if menuNodes[i].Sort == menuNodes[j].Sort {
			return menuNodes[i].Title < menuNodes[j].Title
		}
		return menuNodes[i].Sort < menuNodes[j].Sort
	})
	return menuNodes
}

// EnsureSSHSession 确保用户的SSH会话存在 (默认身份)
func (s *authService) EnsureSSHSession(userID int) error {
	// 获取用户信息（需要角色）
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return fmt.Errorf("获取用户信息失败: %v", err)
	}
	if user == nil {
		return fmt.Errorf("用户不存在")
	}

	isRoot := s.userHasPermission(user, PermissionRootSSH)
	return s.EnsureSSHSessionByType(userID, isRoot)
}

// EnsureSSHSessionByType 确保特定身份的SSH会话存在
func (s *authService) EnsureSSHSessionByType(userID int, isRoot bool) error {
	if s.sessionManager == nil {
		return fmt.Errorf("SSH会话管理器未配置")
	}

	// 1. 检查会话是否已存在
	if s.sessionManager.HasSession(userID, isRoot) {
		return nil
	}

	// 2. 获取用户凭证
	creds, err := s.GetUserCredentialsFromRedis(userID)
	if err != nil {
		return fmt.Errorf("无法恢复SSH会话，请重新登录: %v", err)
	}

	// 3. 获取用户信息
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return fmt.Errorf("获取用户信息失败: %v", err)
	}
	if user == nil {
		return fmt.Errorf("用户不存在")
	}

	// 4. 创建SSH会话
	session, err := s.sessionManager.CreateSession(user.ID, user.Username, creds.Password, isRoot)
	if err != nil {
		return fmt.Errorf("创建SSH会话失败: %v", err)
	}

	// 5. 保存会话元数据
	if s.sessionRepo != nil {
		_ = s.sessionRepo.SaveSession(user.ID, session.GetUsername(), session.IsRoot(), session.CreatedAt)
	}

	logger.Info("SSH会话已恢复",
		zap.Int("user_id", userID),
		zap.String("username", user.Username),
		zap.Bool("is_root", isRoot),
	)

	return nil
}

// Logout 用户登出
func (s *authService) Logout(userID int) error {
	// 删除Redis中的凭证
	if s.sessionRepo != nil {
		if err := s.sessionRepo.DeleteUserCredentials(userID); err != nil {
			logger.Warn("删除用户凭证失败", zap.Int("user_id", userID), zap.Error(err))
		} else {
			logger.Info("已清除Redis中的用户凭证", zap.Int("user_id", userID))
		}
	}

	// 删除SSH会话 (清理 root 和 user 两种身份)
	if s.sessionManager != nil {
		_ = s.sessionManager.DeleteSession(userID, true)
		_ = s.sessionManager.DeleteSession(userID, false)
	}

	// 删除会话元数据
	if s.sessionRepo != nil {
		_ = s.sessionRepo.DeleteSession(userID)
	}

	logger.Info("用户登出成功", zap.Int("user_id", userID))
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

// verifySSHCredentials 验证SSH凭据
func (s *authService) verifySSHCredentials(username, password string) (*server.Client, error) {
	if s.sshServerHost == "" {
		return nil, fmt.Errorf("SSH服务器地址未配置")
	}

	sshConfig := &server.Config{
		Host:     s.sshServerHost,
		Username: username,
		Password: password,
		Timeout:  s.sessionManager.GetTimeout(),
	}

	client, err := server.NewClient(sshConfig)
	if err != nil {
		return nil, fmt.Errorf("SSH凭据验证失败: %w", err)
	}
	return client, nil
}

// saveUserCredentialsToRedis 将用户的SSH凭证保存到Redis
func (s *authService) saveUserCredentialsToRedis(userID int, username, password string) error {
	creds := sshCredentials{
		Username: username,
		Password: password,
	}
	credsJSON, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("序列化SSH凭证失败: %w", err)
	}

	ctx := context.Background()
	key := fmt.Sprintf("ssh_creds:%d", userID)
	// 凭证有效期设置为7天
	err = s.redisRepo.Set(ctx, key, string(credsJSON), 7*24*time.Hour)
	if err != nil {
		return fmt.Errorf("保存SSH凭证到Redis失败: %w", err)
	}
	return nil
}

// GetUserCredentialsFromRedis 从Redis获取用户的SSH凭证
func (s *authService) GetUserCredentialsFromRedis(userID int) (*sshCredentials, error) {
	ctx := context.Background()
	key := fmt.Sprintf("ssh_creds:%d", userID)
	credsJSON, err := s.redisRepo.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("从Redis获取SSH凭证失败: %w", err)
	}
	if credsJSON == "" {
		return nil, bizerrors.ErrSSHCredentialsNotFound
	}

	var creds sshCredentials
	err = json.Unmarshal([]byte(credsJSON), &creds)
	if err != nil {
		return nil, fmt.Errorf("反序列化SSH凭证失败: %w", err)
	}
	return &creds, nil
}

// SendEmailCode 发送邮箱验证码
func (s *authService) SendEmailCode(emailStr string) error {
	// 1. 生成6位随机数字
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	code := fmt.Sprintf("%06d", rnd.Intn(1000000))

	// 2. 存储到 Redis (有效期5分钟)
	ctx := context.Background()
	key := fmt.Sprintf("email_code:%s", emailStr)
	err := s.redisRepo.Set(ctx, key, code, 5*time.Minute)
	if err != nil {
		return bizerrors.NewWithErr(bizerrors.CodeInternalError, "缓存验证码失败", err)
	}

	// 3. 发送邮件
	subject := "您的验证码"
	body := fmt.Sprintf("<h1>您的验证码是: %s</h1><p>有效期5分钟，请勿泄露给他人。</p>", code)
	if err := email.SendEmail(emailStr, subject, body); err != nil {
		return bizerrors.NewWithErr(bizerrors.CodeInternalError, "发送邮件失败", err)
	}

	return nil
}

// GetPermissionsByRole 根据角色 Slug 获取权限列表
func (s *authService) GetPermissionsByRole(slug string) ([]entity.Permission, error) {
	role, err := s.roleRepo.FindBySlug(slug)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "角色不存在")
	}

	return role.Permissions, nil
}
func (s *authService) CheckUserPermission(userID int, method string, path string) (bool, error) {
	return s.roleRepo.CheckUserPermission(userID, method, path)
}

// GetAllRoles 获取所有角色
func (s *authService) GetAllRoles() ([]entity.Role, error) {
	return s.roleRepo.ListAll()
}

// SetSSHServerHost 设置SSH服务器地址
func (s *authService) SetSSHServerHost(host string) {
	s.sshServerHost = host
}

// SetSSHTimeout 设置SSH连接超时
func (s *authService) SetSSHTimeout(timeout time.Duration) {
	if s.sessionManager != nil {
		s.sessionManager.SetTimeout(timeout)
	}
}

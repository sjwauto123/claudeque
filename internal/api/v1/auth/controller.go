package auth

import (
	"cloudque/internal/model/dto/request"
	"cloudque/internal/service"
	"cloudque/pkg/response"

	"github.com/gin-gonic/gin"
)

// Controller 认证控制器
type Controller struct {
	authService service.AuthService
	userService service.UserService
	logService  service.LogService
}

// NewController 创建认证控制器
func NewController(authService service.AuthService, userService service.UserService, logService service.LogService) *Controller {
	return &Controller{
		authService: authService,
		userService: userService,
		logService:  logService,
	}
}

func (ctrl *Controller) Register(c *gin.Context) {
	var req request.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := ctrl.userService.Register(&req); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}

// Login 用户登录
// @Summary 用户登录
// @Description 用户登录获取 Token
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body request.LoginRequest true "登录信息"
// @Success 200 {object} response.Response{data=response.LoginResponse}
// @Router /api/v1/auth/login [post]
func (ctrl *Controller) Login(c *gin.Context) {
	var req request.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := ctrl.authService.Login(&req)
	if err != nil {
		response.BizError(c, err)
		return
	}

	ctrl.logService.CreateLog(req.Username, "Login", "User logged in")
	response.Success(c, resp)
}

// RefreshToken 刷新 Token
// @Summary 刷新 Token
// @Description 使用旧 Token 获取新 Token
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body request.RefreshTokenRequest true "Token"
// @Success 200 {object} response.Response
// @Router /api/v1/auth/refresh [post]
func (ctrl *Controller) RefreshToken(c *gin.Context) {
	var req request.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	newToken, err := ctrl.authService.RefreshToken(req.Token)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, map[string]string{
		"token": newToken,
	})
}

// Logout 用户登出
// @Summary 用户登出
// @Description 用户登出并关闭SSH会话
// @Tags 认证
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} response.Response
// @Router /api/v1/auth/logout [post]
func (ctrl *Controller) Logout(c *gin.Context) {
	// 从上下文获取用户ID
	userID := getUserIDFromContext(c)
	if userID == 0 {
		response.Unauthorized(c, "获取用户信息失败")
		return
	}

	if err := ctrl.authService.Logout(userID); err != nil {
		response.BizError(c, err)
		return
	}

	username := getUsernameFromContext(c)
	ctrl.logService.CreateLog(username, "Logout", "User logged out")
	response.Success(c, nil)
}

// getUserIDFromContext 从上下文获取用户ID
func getUserIDFromContext(c *gin.Context) uint {
	if userID, exists := c.Get("user_id"); exists {
		if id, ok := userID.(uint); ok {
			return id
		}
	}
	return 0
}

// getUsernameFromContext 从上下文获取用户名
func getUsernameFromContext(c *gin.Context) string {
	if username, exists := c.Get("username"); exists {
		if name, ok := username.(string); ok {
			return name
		}
	}
	return ""
}

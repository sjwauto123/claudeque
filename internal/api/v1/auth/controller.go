package auth

import (
	"cloudque/internal/model/dto/request"
	"cloudque/internal/service"
	"cloudque/pkg/captcha"
	"cloudque/pkg/response"
	"cloudque/pkg/utils"
	"regexp"

	"github.com/gin-gonic/gin"
)

// Controller 认证控制器
type Controller struct {
	authService             service.AuthService
	userService             service.UserService
	userOperationLogService service.UserOperationLogService
}

// NewController 创建认证控制器
func NewController(authService service.AuthService, userService service.UserService, userOperationLogService service.UserOperationLogService) *Controller {
	return &Controller{
		authService:             authService,
		userService:             userService,
		userOperationLogService: userOperationLogService,
	}
}

func (ctrl *Controller) Register(c *gin.Context) {
	var req request.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	p1 := utils.DecryptIfCryptoJS(req.Password)
	p2 := utils.DecryptIfCryptoJS(req.ConfirmPassword)
	if p1 != p2 {
		response.BadRequest(c, "两次输入的密码不一致")
		return
	}

	qqEmailRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9_.]{2,14})[a-zA-Z0-9]@qq\.com$`)
	if qqEmailRegex.MatchString(req.Email) == false {
		response.BadRequest(c, "目前仅支持qq邮箱")
		return
	}

	var regexpLetterOnly = regexp.MustCompile(`^[a-zA-Z]+$`)
	if regexpLetterOnly.MatchString(req.Username) == false {
		response.BadRequest(c, "用户名只能包含大小写英文字母，不能有数字、符号或中文")
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

	var regexpLetterOnly = regexp.MustCompile(`^[a-zA-Z]+$`)
	if regexpLetterOnly.MatchString(req.Username) == false {
		response.BadRequest(c, "用户名只能包含大小写英文字母，不能有数字、符号或中文")
		return
	}

	resp, err := ctrl.authService.Login(&req)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// ResetPassword 重置密码
func (ctrl *Controller) ResetPassword(c *gin.Context) {
	var req request.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	n1 := utils.DecryptIfCryptoJS(req.NewPassword)
	n2 := utils.DecryptIfCryptoJS(req.ConfirmPassword)
	if n1 != n2 {
		response.BadRequest(c, "两次输入的密码不一致")
	}

	if err := ctrl.userService.ResetPassword(&req); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
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

// SendEmailCode 发送邮箱验证码
// @Summary 发送邮箱验证码
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body request.SendEmailCodeRequest true "邮箱信息"
// @Success 200 {object} response.Response
// @Router /api/v1/auth/email/code [post]
func (ctrl *Controller) SendEmailCode(c *gin.Context) {
	var req request.SendEmailCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := ctrl.authService.SendEmailCode(req.Email); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}

// GetCaptcha 获取图形验证码
// @Summary 获取图形验证码
// @Description 获取图形验证码 ID 和 Base64 图片
// @Tags 认证
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=map[string]string}
// @Router /api/v1/auth/captcha [get]
func (ctrl *Controller) GetCaptcha(c *gin.Context) {
	id, b64s, err := captcha.Generate()
	if err != nil {
		response.InternalError(c, "生成图形验证码失败，请刷新重试")
		return
	}
	response.Success(c, map[string]string{
		"captcha_id":  id,
		"captcha_val": b64s,
	})
}

package user

import (
	"cloudque/internal/middleware"
	"cloudque/internal/model/dto/request"
	"cloudque/internal/service"
	"cloudque/pkg/response"
	"cloudque/pkg/utils"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

// Controller 用户控制器
type Controller struct {
	userService            service.UserService
	authService            service.AuthService
	useOperationLogService service.UserOperationLogService
}

// NewController 创建用户控制器
func NewController(userService service.UserService,
	userOperationLogService service.UserOperationLogService,
	authService service.AuthService,
) *Controller {
	return &Controller{
		userService:            userService,
		authService:            authService,
		useOperationLogService: userOperationLogService,
	}
}

// GetProfile 获取当前用户信息
// @Summary 获取当前用户信息
// @Description 获取当前登录用户的详细信息
// @Tags 用户
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} response.Response
// @Router /api/v1/user/profile [get]
func (ctrl *Controller) GetProfile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	user, err := ctrl.userService.GetUserByID(userID)
	if err != nil {
		response.BizError(c, err)
		return
	}

	userResp := ctrl.userService.GetUserResponse(user)
	response.Success(c, userResp)
}

// UpdateProfile 更新用户信息
// @Summary 更新用户信息
// @Description 更新当前登录用户的信息
// @Tags 用户
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body request.UpdateUserRequest true "更新信息"
// @Success 200 {object} response.Response
// @Router /api/v1/user/profile [put]
func (ctrl *Controller) UpdateProfile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req request.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := ctrl.userService.UpdateUser(userID, &req); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}

// ChangePassword 修改密码
// @Summary 修改密码
// @Description 修改当前登录用户的密码
// @Tags 用户
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body request.ChangePasswordRequest true "密码信息"
// @Success 200 {object} response.Response
// @Router /api/v1/user/password [post]
func (ctrl *Controller) ChangePassword(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req request.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	n1 := utils.DecryptIfCryptoJS(req.NewPassword)
	n2 := utils.DecryptIfCryptoJS(req.ConfirmPassword)
	if n1 != n2 {
		response.BadRequest(c, "两次输入的密码不一致")
	}

	if err := ctrl.userService.ChangePassword(userID, &req); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}

// ListUsers 获取用户列表（分页）
// @Summary 获取用户列表
// @Description 分页获取用户列表
// @Tags 用户
// @Accept json
// @Produce json
// @Security Bearer
// @Param page query int true "页码" minimum(1)
// @Param size query int true "每页大小" minimum(1) maximum(100)
// @Success 200 {object} response.Response
// @Router /api/v1/user/list [get]
func (ctrl *Controller) ListUsers(c *gin.Context) {
	var req request.UserListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	pageResp, err := ctrl.userService.ListUsers(&req)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, pageResp)
}

func (ctrl *Controller) GetByUsername(c *gin.Context) {
	username := c.Query("username")
	if username == "" {
		response.BadRequest(c, "缺少参数：username")
		return
	}
	user, err := ctrl.userService.GetUserByUsername(username)
	if err != nil {
		response.BizError(c, err)
		return
	}
	userResp := ctrl.userService.GetUserResponse(user)
	response.Success(c, userResp)
}

func (ctrl *Controller) UploadAvatar(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}
	file, err := c.FormFile("avatar")
	if err != nil {
		response.BadRequest(c, "缺少文件")
		return
	}
	dir := filepath.Join("uploads", "avatars")
	if err := os.MkdirAll(dir, 0755); err != nil {
		response.BizError(c, err)
		return
	}
	ext := filepath.Ext(file.Filename)
	name := fmt.Sprintf("%d_%d%s", userID, time.Now().UnixNano(), ext)
	dst := filepath.Join(dir, name)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		response.BizError(c, err)
		return
	}
	if err := ctrl.userService.UpdateAvatar(userID, filepath.ToSlash(dst)); err != nil {
		response.BizError(c, err)
		return
	}

	imgFullPath := "http://192.168.10.7:8080/" + filepath.ToSlash(dst)
	response.Success(c, gin.H{"path": imgFullPath})
}

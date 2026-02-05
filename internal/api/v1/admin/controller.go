package admin

import (
	"cloudque/internal/model/dto/request"
	"cloudque/internal/service"
	"cloudque/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Controller struct {
	adminService service.AdminService
	userService  service.UserService
}

func NewController(adminService service.AdminService, userService service.UserService) *Controller {
	return &Controller{
		adminService: adminService,
		userService:  userService,
	}
}

// CreateUser 创建用户
func (ctrl *Controller) CreateUser(c *gin.Context) {
	var req request.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := ctrl.adminService.CreateUser(&req); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, nil)
}

// DeleteUser 删除用户
func (ctrl *Controller) DeleteUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "invalid user id")
		return
	}

	if err := ctrl.adminService.DeleteUser(uint(id)); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, nil)
}

// UpdateUser 更新用户
func (ctrl *Controller) UpdateUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "invalid user id")
		return
	}

	var req request.AdminUpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := ctrl.adminService.AdminUpdateUser(uint(id), &req); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, nil)
}

// GetUser 获取用户详情
func (ctrl *Controller) GetUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "invalid user id")
		return
	}

	user, err := ctrl.userService.GetUserByID(uint(id))
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, ctrl.userService.GetUserResponse(user))
}

// ListUsers 获取用户列表
func (ctrl *Controller) ListUsers(c *gin.Context) {
	var req request.UserListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := ctrl.userService.ListUsers(&req)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, resp)
}

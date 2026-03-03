package admin

import (
	"cloudque/internal/middleware"
	"cloudque/internal/model/dto/request"
	dtoResp "cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"cloudque/internal/service"
	"cloudque/pkg/response"
	"os/exec"
	"regexp"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type Controller struct {
	adminService         service.AdminService
	userService          service.UserService
	authService          service.AuthService
	userOperationLogSer  service.UserOperationLogService
	adminOperationLogSer service.AdminOperationLogService
}

func NewController(adminService service.AdminService, userService service.UserService, authService service.AuthService, userOperationLogSer service.UserOperationLogService, adminOperationLogSer service.AdminOperationLogService) *Controller {
	return &Controller{
		adminService:         adminService,
		userService:          userService,
		authService:          authService,
		userOperationLogSer:  userOperationLogSer,
		adminOperationLogSer: adminOperationLogSer,
	}
}

// CreateUser 创建用户
func (ctrl *Controller) CreateUser(c *gin.Context) {
	var req request.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "数据格式有误")
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

	if err := ctrl.adminService.CreateUser(&req); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, nil)
}

// DeleteUser 删除用户
func (ctrl *Controller) DeleteUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "不存在的id")
		return
	}

	if err := ctrl.adminService.DeleteUser(int(id)); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, nil)
}

// UpdateUser 更新用户
func (ctrl *Controller) UpdateUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "不存在的id")
		return
	}

	var req request.AdminUpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求格式错误")
		return
	}

	if err := ctrl.adminService.AdminUpdateUser(int(id), &req); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, nil)
}

// GetUser 获取用户详情
func (ctrl *Controller) GetUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "invalid user id")
		return
	}

	user, err := ctrl.userService.GetUserByID(int(id))
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

// ListRoleSimple 获取所有角色的名称与标识
func (ctrl *Controller) ListRoleSimple(c *gin.Context) {
	roles, err := ctrl.authService.GetAllRoles()
	if err != nil {
		response.BizError(c, err)
		return
	}
	out := make([]dtoResp.RoleSimple, 0, len(roles))
	for _, r := range roles {
		out = append(out, dtoResp.RoleSimple{
			Name: r.Name,
			Slug: r.Slug,
		})
	}
	response.Success(c, out)
}

// Restart 重启系统
func (ctrl *Controller) Restart(c *gin.Context) {

	logId, err := ctrl.adminOperationLogSer.CreateLog(&entity.AdminOperationLog{
		Username:   middleware.GetUsername(c),
		ActionType: "restart",
		Object:     "重启系统",
		Status:     1,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	})
	if err != nil {
		response.BizError(c, err)
		return
	}
	cmd := exec.Command("sudo", "reboot", "now")
	if err = cmd.Start(); err != nil {
		status, err := ctrl.adminOperationLogSer.UpdateStatus(logId)
		if err != nil || status == 0 {
			response.BizError(c, err)
			return
		}
		response.InternalError(c, "failed to restart system")
		return
	}
	response.Success(c, gin.H{"message": "restarting"})
}

// Shutdown 关闭系统
func (ctrl *Controller) Shutdown(c *gin.Context) {
	logId, err := ctrl.adminOperationLogSer.CreateLog(&entity.AdminOperationLog{
		Username:   middleware.GetUsername(c),
		ActionType: "close",
		Object:     "关闭系统",
		Status:     1,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	})
	if err != nil {
		response.BizError(c, err)
		return
	}
	cmd := exec.Command("sudo", "shutdown", "now")
	if err := cmd.Start(); err != nil {
		status, err := ctrl.adminOperationLogSer.UpdateStatus(logId)
		if err != nil || status == 0 {
			response.BizError(c, err)
			return
		}
		response.InternalError(c, "failed to close system")
		return
	}
	response.Success(c, gin.H{"message": "closing"})
}

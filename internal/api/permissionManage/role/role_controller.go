package role

import (
	"cloudque/internal/model/dto/request"
	"cloudque/internal/service"
	"cloudque/pkg/errors"
	"cloudque/pkg/logger"
	"cloudque/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"strconv"
)

type RoleController struct {
	roleService service.RoleService
}

func NewRoleController(roleService service.RoleService) *RoleController {
	return &RoleController{roleService: roleService}
}
func (ctrl *RoleController) GetRoleByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	role, err := ctrl.roleService.GetRoleByID(id)
	if err != nil {
		logger.Error("获取角色失败", zap.Error(err))
		response.BizError(c, err)
		return
	}
	if role == nil {
		response.NotFound(c, errors.GetMessage(errors.CodeResourceNotFound))
		return
	}

	response.Success(c, role)
}
func (ctrl *RoleController) PageList(c *gin.Context) {
	var req request.RolePageQueryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, errors.GetMessage(errors.CodeInvalidParam))
		return
	}

	roles, total, err := ctrl.roleService.PageList(&req)
	if err != nil {
		logger.Error("查询角色列表失败", zap.Error(err))
		response.BizError(c, err)
		return
	}

	pageResp := response.NewPageResponse(roles, total, req.Page, req.PageSize)
	response.Success(c, pageResp)
}
func (ctrl *RoleController) Create(c *gin.Context) {
	var req request.CreateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数格式错误!")
		return
	}
	err := ctrl.roleService.Create(&req)
	if err != nil {
		logger.Error("创建角色失败", zap.Error(err))
		response.BizError(c, err)
		return
	}
	response.Success(c, "创建成功")
}
func (ctrl *RoleController) Update(c *gin.Context) {
	var req request.UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数格式错误!")
		return
	}
	err := ctrl.roleService.Update(&req)
	if err != nil {
		logger.Error("更新角色失败:", zap.Error(err))
		response.BizError(c, err)
		return
	}
	response.Success(c, "成功")
}
func (ctrl *RoleController) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, errors.GetMessage(errors.CodeInvalidParam))
		return
	}

	err = ctrl.roleService.Delete(id)
	if err != nil {
		logger.Error("删除角色失败", zap.Error(err))
		response.BizError(c, err)
		return
	}
	response.Success(c, "删除成功")
}
func (ctrl *RoleController) BatchDelete(c *gin.Context) {
	var req request.BatchDeleteAPIRequest
	// 绑定请求参数
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, errors.GetMessage(errors.CodeInvalidParam))
		return
	}

	err := ctrl.roleService.BatchDelete(req.IDs)
	if err != nil {
		logger.Error("批量删除角色失败", zap.Error(err))
		response.BizError(c, err)
		return
	}

	response.Success(c, "批量删除成功")
}
func (ctrl *RoleController) GetRolePermissionByID(c *gin.Context) {
	idStr := c.Param("id")
	roleID, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, errors.GetMessage(errors.CodeInvalidParam))
		return
	}

	res, err := ctrl.roleService.GetRolePermissionByID(roleID)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, res)
}
func (ctrl *RoleController) UpdateRolePermission(c *gin.Context) {
	idStr := c.Param("role_id")
	roleID, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, errors.GetMessage(errors.CodeInvalidParam))
		return
	}

	var req request.UpdateRolePermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, errors.GetMessage(errors.CodeInvalidParam))
		return
	}

	err = ctrl.roleService.UpdateRolePermission(roleID, &req)
	if err != nil {
		logger.Error("更新角色权限失败", zap.Error(err))
		response.BizError(c, err)
		return
	}

	response.Success(c, "权限更新成功")
}

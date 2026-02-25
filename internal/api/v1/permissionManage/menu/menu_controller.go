package menu

import (
	"cloudque/internal/model/dto/request"
	dto "cloudque/internal/model/dto/response"
	"cloudque/internal/service"
	"cloudque/pkg/errors"
	"cloudque/pkg/logger"
	"cloudque/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"strconv"
)

type MenuController struct {
	menuService    service.MenuService
	authService    service.AuthService
	userLogService service.UserOperationLogService
}

// NewMenuController 创建菜单控制器实例
func NewMenuController(menuService service.MenuService, authService service.AuthService, userLogService service.UserOperationLogService) *MenuController {
	return &MenuController{
		menuService:    menuService,
		authService:    authService,
		userLogService: userLogService,
	}
}

func (ctrl *MenuController) GetMenuByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, errors.GetMessage(errors.CodeInvalidParam))
		return
	}

	menu, err := ctrl.menuService.GetMenuByID(id)
	if err != nil {
		logger.Error("查询菜单详情失败", zap.Error(err))
		response.BizError(c, err)
		return
	}
	if menu == nil {
		response.NotFound(c, errors.GetMessage(errors.CodeResourceNotFound))
		return
	}

	// 转换为响应格式
	menuResp := dto.MenuResponse{
		ID:       menu.ID,
		ParentID: menu.ParentID,
		Title:    menu.Title,
		Type:     menu.Type,
		Status:   menu.Status,
		Icon:     menu.Icon,
		URI:      menu.URI,
		Sort:     menu.Sort,
	}

	response.Success(c, menuResp)
}

// PageList 分页查询菜单列表（返回树形结构）
func (ctrl *MenuController) PageList(c *gin.Context) {
	var req request.MenuPageQueryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, errors.GetMessage(errors.CodeInvalidParam))
		return
	}

	list, total, err := ctrl.menuService.PageList(&req)
	if err != nil {
		logger.Error("查询菜单列表失败", zap.Error(err))
		response.BizError(c, err)
		return
	}

	pageResp := response.NewPageResponse(list, total, req.Page, req.PageSize)
	response.Success(c, pageResp)
}

func (ctrl *MenuController) Create(c *gin.Context) {
	var req request.CreateMenuRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数格式错误!")
		return
	}

	err := ctrl.menuService.Create(&req)
	if err != nil {
		logger.Error("创建菜单失败", zap.Error(err))
		response.BizError(c, err)
		return
	}

	response.Success(c, "创建成功")
}

func (ctrl *MenuController) Update(c *gin.Context) {
	var req request.UpdateMenuRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数格式错误!")
		return
	}

	err := ctrl.menuService.Update(&req)
	if err != nil {
		logger.Error("更新菜单失败:", zap.Error(err))
		response.BizError(c, err)
		return
	}

	response.Success(c, "更新成功")
}

func (ctrl *MenuController) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, errors.GetMessage(errors.CodeInvalidParam))
		return
	}

	err = ctrl.menuService.Delete(id)
	if err != nil {
		logger.Error("删除菜单失败", zap.Error(err))
		response.BizError(c, err)
		return
	}

	response.Success(c, "删除成功")
}

func (ctrl *MenuController) BatchDelete(c *gin.Context) {
	var req request.BatchDeleteMenuRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, errors.GetMessage(errors.CodeInvalidParam))
		return
	}

	err := ctrl.menuService.BatchDelete(req.IDs)
	if err != nil {
		logger.Error("批量删除菜单失败", zap.Error(err))
		response.BizError(c, err)
		return
	}

	response.Success(c, "批量删除成功")
}

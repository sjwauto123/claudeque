package permission

import (
	"cloudque/internal/model/dto/request"
	response2 "cloudque/internal/model/dto/response"
	"cloudque/internal/service"
	"cloudque/pkg/errors"
	"cloudque/pkg/logger"
	"cloudque/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"strconv"
)

type APIController struct {
	apiService     service.APIService
	authService    service.AuthService
	userLogService service.UserOperationLogService
}

func NewAPIController(apiService service.APIService, authService service.AuthService, userLogService service.UserOperationLogService) *APIController {
	return &APIController{
		apiService:     apiService,
		authService:    authService,
		userLogService: userLogService,
	}
}

func (ctrl *APIController) GetAPIByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	api, err := ctrl.apiService.GetAPIByID(id)
	if err != nil {
		logger.Error("获取api失败", zap.Error(err))
		response.BizError(c, err)
		return
	}
	if api == nil {
		response.NotFound(c, errors.GetMessage(errors.CodeResourceNotFound))
		return
	}

	response.Success(c, api)
}

// PageList 分页查询API列表
func (ctrl *APIController) PageList(c *gin.Context) {
	var req request.APIPageQueryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, errors.GetMessage(errors.CodeInvalidParam))
		return
	}

	apis, total, err := ctrl.apiService.PageList(&req)
	if err != nil {
		logger.Error("查询API列表失败", zap.Error(err))
		response.BizError(c, err)
		return
	}

	// 转换为响应格式
	var apiResponses []response2.APIResponse
	for _, api := range apis {
		apiResponses = append(apiResponses, response2.APIResponse{
			ID:         api.ID,
			Name:       api.Name,
			Category:   api.Category,
			Slug:       api.Slug,
			Status:     api.Status,
			HTTPMethod: api.HttpMethod,
			HTTPPath:   api.HttpPath,
		})
	}

	pageResp := response.NewPageResponse(apiResponses, total, req.Page, req.PageSize)
	response.Success(c, pageResp)
}

// Create 创建API
func (ctrl *APIController) Create(c *gin.Context) {
	var req request.CreateAPIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数格式错误!")
		return
	}

	err := ctrl.apiService.Create(&req)
	if err != nil {
		logger.Error("创建API失败", zap.Error(err))
		response.BizError(c, err)
		return
	}

	response.Success(c, "创建成功")
}

// Update 修改API
func (ctrl *APIController) Update(c *gin.Context) {
	var req request.UpdateAPIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数格式错误!")
		return
	}

	err := ctrl.apiService.Update(&req)
	if err != nil {
		logger.Error("更新API失败:", zap.Error(err))
		response.BizError(c, err)
		return
	}

	response.Success(c, "更新成功")
}

func (ctrl *APIController) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, errors.GetMessage(errors.CodeInvalidParam))
		return
	}

	err = ctrl.apiService.Delete(id)
	if err != nil {
		logger.Error("删除API失败", zap.Error(err))
		response.BizError(c, err)
		return
	}

	response.Success(c, "删除成功")
}

func (ctrl *APIController) BatchDelete(c *gin.Context) {
	var req request.BatchDeleteAPIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, errors.GetMessage(errors.CodeInvalidParam))
		return
	}

	err := ctrl.apiService.BatchDelete(req.IDs)
	if err != nil {
		logger.Error("批量删除API失败", zap.Error(err))
		response.BizError(c, err)
		return
	}

	response.Success(c, "批量删除成功")
}

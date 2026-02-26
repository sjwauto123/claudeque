package files

import (
	"cloudque/internal/middleware"
	"cloudque/internal/model/dto/request"
	"cloudque/internal/service"
	"cloudque/pkg/response"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"

	"github.com/gin-gonic/gin"
)

// Controller 文件控制器
type Controller struct {
	fileService    service.FileService
	authService    service.AuthService
	userLogService service.UserOperationLogService
}

// NewController 创建文件控制器
func NewController(fileService service.FileService, authService service.AuthService, userLogService service.UserOperationLogService) *Controller {
	return &Controller{
		fileService:    fileService,
		authService:    authService,
		userLogService: userLogService,
	}
}

// GetFileList 获取文件列表

func (ctrl *Controller) GetFileList(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req request.FileListRequest
	// 优先尝试从 Query 参数获取 (标准 GET 请求)
	if err := c.ShouldBindQuery(&req); err != nil {
		// 如果 Query 绑定失败或为空，尝试从 JSON Body 获取 (支持前端非标准调用)
		if errBody := c.ShouldBindJSON(&req); errBody != nil {
			response.BadRequest(c, err.Error())
			return
		}
	}
	// 二次检查：如果 Query 绑定成功但关键参数为空，尝试 JSON
	// 注意：FileListRequest 的字段可能都不是必填的，所以这里只是尝试性补充
	if req.Path == "" && req.Page == 0 && req.PageSize == 0 {
		_ = c.ShouldBindJSON(&req)
	}

	isRootMode, err := ctrl.authService.HasSystemAccess(userID)
	if err != nil {
		response.BizError(c, err)
		return
	}

	data, err := ctrl.fileService.GetFileList(userID, &req, isRootMode)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, data)
}

// GetDiskUsage 计算目录磁盘占比和大小
// @Summary 计算目录磁盘占比和大小
// @Description 计算指定目录的大小和磁盘占用比例
// @Tags 文件管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body request.DiskUsageRequest true "请求参数"
// @Success 200 {object} response.Response{data=dto.DiskUsageData}
// @Router /api/user/directories/calculate-usage [get]
func (ctrl *Controller) GetDiskUsage(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req request.DiskUsageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if req.Path == "" {
		response.BadRequest(c, "缺少 path 参数")
		return
	}

	isRootMode, err := ctrl.authService.HasSystemAccess(userID)
	if err != nil {
		response.BizError(c, err)
		return
	}

	data, err := ctrl.fileService.GetDiskUsage(userID, req.Path, isRootMode)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, data)
}

// ListHomeDirectories 获取 /home 目录下的所有用户目录列表
func (ctrl *Controller) ListHomeDirectories(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req request.FileListRequest
	// 优先尝试从 Query 参数获取 (标准 GET 请求)
	if err := c.ShouldBindQuery(&req); err != nil {
		// 如果 Query 绑定失败，尝试从 JSON Body 获取
		if errBody := c.ShouldBindJSON(&req); errBody != nil {
			response.BadRequest(c, err.Error())
			return
		}
	}
	// 如果 Query 参数没传，尝试 JSON
	if req.Path == "" && req.Page == 0 && req.PageSize == 0 {
		_ = c.ShouldBindJSON(&req)
	}

	isRootMode, err := ctrl.authService.HasSystemAccess(userID)
	if err != nil {
		response.BizError(c, err)
		return
	}

	data, err := ctrl.fileService.GetHomeDirectoriesList(userID, &req, isRootMode)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, data)
}

// CalculateSize 计算目录或文件大小
func (ctrl *Controller) CalculateSize(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req request.DiskUsageRequest
	// 优先尝试从 Query 参数获取
	if err := c.ShouldBindQuery(&req); err != nil {
		// 如果 Query 绑定失败，尝试从 JSON Body 获取
		if errBody := c.ShouldBindJSON(&req); errBody != nil {
			response.BadRequest(c, err.Error())
			return
		}
	}
	// 二次检查：如果 Query 绑定成功但 Path 为空，尝试 JSON
	if req.Path == "" {
		_ = c.ShouldBindJSON(&req)
	}

	isRootMode, err := ctrl.authService.HasSystemAccess(userID)
	if err != nil {
		response.BizError(c, err)
		return
	}

	size, sizeStr, usage, err := ctrl.fileService.CalculateSize(userID, req.Path, isRootMode)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, gin.H{
		"size":          size,
		"size_human":    sizeStr,
		"usage_percent": usage,
	})
}

// DeleteFile 删除文件或目录
func (ctrl *Controller) DeleteFile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req request.DeleteFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	isRootMode, err := ctrl.authService.HasSystemAccess(userID)
	if err != nil {
		response.BizError(c, err)
		return
	}

	err = ctrl.fileService.DeleteFile(userID, req.Path, isRootMode)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}

// UploadFile 上传文件
func (ctrl *Controller) UploadFile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req request.UploadFileRequest
	if err := c.ShouldBind(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	file, err := req.File.Open()
	if err != nil {
		response.BadRequest(c, "文件打开失败: "+err.Error())
		return
	}
	defer func(file multipart.File) {
		err := file.Close()
		if err != nil {
			return
		}
	}(file)

	// 判断是系统模式还是用户模式
	isRootMode, err := ctrl.authService.HasSystemAccess(userID)
	if err != nil {
		response.BizError(c, err)
		return
	}

	data, err := ctrl.fileService.UploadFile(userID, file, req.File, req.TargetPath, isRootMode)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, data)
}

// DownloadFile 下载文件
func (ctrl *Controller) DownloadFile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req request.DownloadRequest
	// 优先尝试从 Query 参数获取 (支持浏览器直接下载)
	if err := c.ShouldBindQuery(&req); err != nil {
		// 如果 Query 参数绑定失败(通常是校验失败)，或者没有传参
		// 尝试从 JSON Body 获取 (支持前端 POST/GET JSON 调用)
		// 注意：GET 请求带 Body 不符合标准，但在某些内部调用中可能存在
		if errBody := c.ShouldBindJSON(&req); errBody != nil {
			// 如果两者都失败，返回 Query 的错误(或者根据情况返回)
			response.BadRequest(c, "Invalid parameters: "+err.Error())
			return
		}
	}
	// 二次校验：如果 Query 绑定成功但 Path 为空(虽然有 required 校验，但为了稳妥)，再次尝试 JSON
	if req.Path == "" {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
	}

	isRootMode, err := ctrl.authService.HasSystemAccess(userID)
	if err != nil {
		response.BizError(c, err)
		return
	}
	reader, filename, fileSize, err := ctrl.fileService.DownloadFile(userID, req.Path, isRootMode)
	if err != nil {
		response.BizError(c, err)
		return
	}
	defer func(reader io.ReadCloser) {
		err := reader.Close()
		if err != nil {
			return
		}
	}(reader)

	c.Header("Content-Disposition", "attachment; filename="+url.QueryEscape(filename))
	c.Header("Content-Length", fmt.Sprintf("%d", fileSize))
	_, err = io.Copy(c.Writer, reader)
	if err != nil {
		return
	}
}

// UnzipFile 解压文件
func (ctrl *Controller) UnzipFile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req request.UnzipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	isRootMode, err := ctrl.authService.HasSystemAccess(userID)
	if err != nil {
		response.BizError(c, err)
		return
	}

	err = ctrl.fileService.UnzipFile(userID, &req, isRootMode)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}
